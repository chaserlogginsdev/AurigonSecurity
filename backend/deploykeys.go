package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// ── Deploy key format ──────────────────────────────────────────────────────
//
// A deploy key is a base64url-encoded JSON payload prefixed with "AGT-":
//
//   AGT-eyJiYWNrZW5kdXJsIjoiaHR0cDovLzEwLjAuMC41OjgwODAiLCJrZXkiOiJzZWNyZXQifQ==
//
// The payload contains:
//   { "backend_url": "http://10.0.0.5:8080", "key": "shared-agent-secret", "id": "abc123" }
//
// The agent decodes this on startup — no raw secrets ever visible to the customer.
// ──────────────────────────────────────────────────────────────────────────

const deployKeyPrefix = "AGT-"

type DeployKeyPayload struct {
	ID         string `json:"id"`
	BackendURL string `json:"backend_url"`
	Key        string `json:"key"`
}

type DeployKeyRow struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Token     string `json:"token"`
	CreatedBy string `json:"created_by"`
	CreatedAt string `json:"created_at"`
	LastUsed  string `json:"last_used"`
	Revoked   bool   `json:"revoked"`
}

type CreateDeployKeyRequest struct {
	Label      string `json:"label"`
	BackendURL string `json:"backend_url"`
}

// ── Generate ───────────────────────────────────────────────────────────────

func generateDeployKeyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	createdBy := getUsernameFromToken(r)
	if createdBy == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req CreateDeployKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if req.Label == "" {
		req.Label = "Deploy Key " + time.Now().Format("2006-01-02")
	}
	if req.BackendURL == "" {
		http.Error(w, "backend_url is required", http.StatusBadRequest)
		return
	}

	// Generate a unique ID and a random agent key for this deploy key
	id := randomHex(8)
	agentKey := randomHex(16)

	// Build the payload the agent will decode
	payload := DeployKeyPayload{
		ID:         id,
		BackendURL: req.BackendURL,
		Key:        agentKey,
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "failed to create key", http.StatusInternalServerError)
		return
	}

	// Encode as base64url and prefix with AGT-
	token := deployKeyPrefix + base64.URLEncoding.EncodeToString(payloadJSON)

	// Store in database (store the agent key so backend can validate agents using it)
	_, err = db.Exec(`
		INSERT INTO deploy_keys (id, label, token, agent_key, backend_url, created_by)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id, req.Label, token, agentKey, req.BackendURL, createdBy)
	if err != nil {
		http.Error(w, "failed to store key: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("Deploy key created: %s (%s) by %s", id, req.Label, createdBy)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"id":    id,
		"label": req.Label,
		"token": token,
	})
}

// ── List ───────────────────────────────────────────────────────────────────

func listDeployKeysHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`
		SELECT id, label, token, created_by, created_at,
		       COALESCE(last_used, ''), revoked
		FROM deploy_keys
		ORDER BY created_at DESC
	`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	keys := []DeployKeyRow{}
	for rows.Next() {
		var k DeployKeyRow
		var revoked int
		rows.Scan(&k.ID, &k.Label, &k.Token, &k.CreatedBy, &k.CreatedAt, &k.LastUsed, &revoked)
		k.Revoked = revoked == 1
		keys = append(keys, k)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(keys)
}

// ── Revoke ─────────────────────────────────────────────────────────────────

func revokeDeployKeyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	result, err := db.Exec(`UPDATE deploy_keys SET revoked = 1 WHERE id = ?`, req.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		http.Error(w, "key not found", http.StatusNotFound)
		return
	}

	log.Printf("Deploy key revoked: %s", req.ID)
	w.WriteHeader(http.StatusOK)
}

// ── Validate (called by agent on registration) ─────────────────────────────
// Agents now send X-Deploy-Key header instead of X-Agent-Key.
// Backend decodes it, looks up the key in the DB, and validates it.

func validateDeployKey(deployKey string) (agentKey string, err error) {
	if len(deployKey) <= len(deployKeyPrefix) {
		return "", fmt.Errorf("invalid deploy key format")
	}

	encoded := deployKey[len(deployKeyPrefix):]
	payloadJSON, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("invalid deploy key encoding")
	}

	var payload DeployKeyPayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return "", fmt.Errorf("invalid deploy key payload")
	}

	// Look up in database and check not revoked
	var storedKey string
	var revoked int
	err = db.QueryRow(`
		SELECT agent_key, revoked FROM deploy_keys WHERE id = ?
	`, payload.ID).Scan(&storedKey, &revoked)
	if err != nil {
		return "", fmt.Errorf("deploy key not found")
	}
	if revoked == 1 {
		return "", fmt.Errorf("deploy key has been revoked")
	}

	// Update last_used timestamp
	db.Exec(`UPDATE deploy_keys SET last_used = CURRENT_TIMESTAMP WHERE id = ?`, payload.ID)

	return storedKey, nil
}

// ── Helpers ────────────────────────────────────────────────────────────────

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	result := make([]byte, n*2)
	const hexChars = "0123456789abcdef"
	for i, v := range b {
		result[i*2] = hexChars[v>>4]
		result[i*2+1] = hexChars[v&0xf]
	}
	return string(result)
}