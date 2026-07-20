package main

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// ── Agent Key ──────────────────────────────────────────────────────────────
//
// Each tenant has exactly ONE permanent agent key. It never changes unless
// explicitly rotated. The key is a base64url-encoded token prefixed with
// "AGT-" that carries everything the agent needs to connect and identify
// its tenant:
//
//   AGT-<base64({"tenant_id":"tnt_xxx","key":"<tenant secret>","backend_url":"https://..."})>
//
// The installer asks the customer for this one value — nothing else.
// ──────────────────────────────────────────────────────────────────────────

const agentKeyPrefix = "AGT-"

type agentTokenPayload struct {
	TenantID   string `json:"tenant_id"`
	Key        string `json:"key"`
	BackendURL string `json:"backend_url"`
}

// buildAgentToken creates the full AGT- token for a tenant, embedding the
// backend URL the dashboard was accessed from. This means the token
// automatically points at the right place when the deployment moves from
// localhost to a real production domain — no manual reconfiguration needed.
func buildAgentToken(tenant *Tenant, backendURL string) (string, error) {
	payload := agentTokenPayload{
		TenantID:   tenant.ID,
		Key:        tenant.AgentKey,
		BackendURL: backendURL,
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return agentKeyPrefix + base64.URLEncoding.EncodeToString(payloadJSON), nil
}

// validateAgentToken decodes an AGT- token sent by an agent and confirms
// it matches an active tenant's stored secret. Returns the tenant ID on success.
func validateAgentToken(token string) (tenantID string, err error) {
	if !strings.HasPrefix(token, agentKeyPrefix) {
		return "", fmt.Errorf("invalid agent key format")
	}

	encoded := token[len(agentKeyPrefix):]
	payloadJSON, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("invalid agent key encoding")
	}

	var payload agentTokenPayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return "", fmt.Errorf("invalid agent key payload")
	}

	if payload.TenantID == "" || payload.Key == "" {
		return "", fmt.Errorf("agent key missing required fields")
	}

	tenant, err := getTenant(payload.TenantID)
	if err != nil {
		return "", fmt.Errorf("tenant not found")
	}
	if tenant.Status != "active" {
		return "", fmt.Errorf("tenant is not active")
	}
	if tenant.AgentKey == "" || subtle.ConstantTimeCompare([]byte(tenant.AgentKey), []byte(payload.Key)) != 1 {
		return "", fmt.Errorf("agent key does not match tenant record")
	}

	return tenant.ID, nil
}

// deriveBackendURL figures out the public-facing URL of this backend from
// the incoming request, so the generated agent key always points at
// wherever the dashboard is actually being served from — localhost during
// dev, and the real domain once deployed, with no config change needed.
func deriveBackendURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}
	host := r.Host
	if fwd := r.Header.Get("X-Forwarded-Host"); fwd != "" {
		host = fwd
	}
	return scheme + "://" + host
}

// ── Handler ────────────────────────────────────────────────────────────────

// agentKeyHandler returns the current tenant's permanent agent key token,
// ready to paste into the installer.
func agentKeyHandler(w http.ResponseWriter, r *http.Request) {
	tenantID := tenantIDFromCtx(r)

	agentKeySecret, err := ensureTenantAgentKey(tenantID)
	if err != nil {
		http.Error(w, "failed to load agent key", http.StatusInternalServerError)
		return
	}

	tenant, err := getTenant(tenantID)
	if err != nil {
		http.Error(w, "tenant not found", http.StatusNotFound)
		return
	}
	tenant.AgentKey = agentKeySecret

	backendURL := deriveBackendURL(r)

	token, err := buildAgentToken(tenant, backendURL)
	if err != nil {
		http.Error(w, "failed to build agent key", http.StatusInternalServerError)
		return
	}

	log.Printf("Agent key requested for tenant %s", tenantID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token":       token,
		"backend_url": backendURL,
	})
}