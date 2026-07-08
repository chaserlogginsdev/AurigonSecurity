package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

const deployKeyPrefix = "AGT-"

// DeployKeyPayload is embedded in the AGT- token.
// It carries everything the agent needs to connect AND
// enough info for the backend to route to the right tenant.
type DeployKeyPayload struct {
	ID       string `json:"id"`        // key ID (for DB lookup)
	TenantID string `json:"tenant_id"` // which tenant this key belongs to
	Key      string `json:"key"`       // the agent shared secret
	BackendURL string `json:"backend_url"`
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

	tenantID := tenantIDFromCtx(r)
	createdBy := usernameFromCtx(r)
	db := dbFromCtx(r)

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

	// Generate a unique key ID and a random agent secret
	id := randomHex(8)
	agentKey := randomHex(16)

	// Build the payload — includes tenant_id so backend can route requests
	payload := DeployKeyPayload{
		ID:         id,
		TenantID:   tenantID,
		BackendURL: req.BackendURL,
		Key:        agentKey,
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "failed to create key", http.StatusInternalServerError)
		return
	}

	token := deployKeyPrefix + base64.URLEncoding.EncodeToString(payloadJSON)

	// Store in tenant's database
	_, err = db.Exec(`
		INSERT INTO deploy_keys (id, label, token, agent_key, backend_url, created_by)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id, req.Label, token, agentKey, req.BackendURL, createdBy)
	if err != nil {
		http.Error(w, "failed to store key: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("Deploy key created: %s (%s) by %s in tenant %s", id, req.Label, createdBy, tenantID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"id":    id,
		"label": req.Label,
		"token": token,
	})
}

// ── List ───────────────────────────────────────────────────────────────────

func listDeployKeysHandler(w http.ResponseWriter, r *http.Request) {
	db := dbFromCtx(r)

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
		// Never return the full token to the list — only show it once at creation
		k.Token = ""
		keys = append(keys, k)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(keys)
}

// ── Revoke ─────────────────────────────────────────────────────────────────

func revokeDeployKeyHandler(w http.ResponseWriter, r *http.Request) {
	db := dbFromCtx(r)
	tenantID := tenantIDFromCtx(r)

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
	n, _ := result.RowsAffected()
	if n == 0 {
		http.Error(w, "key not found", http.StatusNotFound)
		return
	}

	log.Printf("Deploy key revoked: %s in tenant %s", req.ID, tenantID)
	w.WriteHeader(http.StatusOK)
}

// ── Validation (called by agent middleware) ────────────────────────────────

// validateDeployKeyForTenant decodes an AGT- token and returns
// the tenant ID and agent key. Also marks last_used in the tenant DB.
func validateDeployKeyForTenant(deployKey string) (tenantID string, agentKey string, err error) {
	if len(deployKey) <= len(deployKeyPrefix) {
		return "", "", fmt.Errorf("invalid deploy key format")
	}

	encoded := deployKey[len(deployKeyPrefix):]
	payloadJSON, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return "", "", fmt.Errorf("invalid deploy key encoding")
	}

	var payload DeployKeyPayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return "", "", fmt.Errorf("invalid deploy key payload")
	}

	if payload.TenantID == "" || payload.ID == "" {
		return "", "", fmt.Errorf("deploy key missing required fields")
	}

	// Look up in tenant's database
	db, err := getTenantDB(payload.TenantID)
	if err != nil {
		return "", "", fmt.Errorf("tenant not found")
	}

	var storedKey string
	var revoked int
	err = db.QueryRow(`
		SELECT agent_key, revoked FROM deploy_keys WHERE id = ?
	`, payload.ID).Scan(&storedKey, &revoked)
	if err != nil {
		return "", "", fmt.Errorf("deploy key not found")
	}
	if revoked == 1 {
		return "", "", fmt.Errorf("deploy key has been revoked")
	}

	// Update last_used
	db.Exec(`UPDATE deploy_keys SET last_used = CURRENT_TIMESTAMP WHERE id = ?`, payload.ID)

	return payload.TenantID, storedKey, nil
}

// validateDeployKey is kept for backward compatibility
func validateDeployKey(deployKey string) (string, error) {
	_, agentKey, err := validateDeployKeyForTenant(deployKey)
	return agentKey, err
}