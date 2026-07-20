package service

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
)

const agentKeyPrefix = "AGT-"

// agentTokenPayload mirrors the backend's agentTokenPayload struct.
type agentTokenPayload struct {
	TenantID   string `json:"tenant_id"`
	Key        string `json:"key"`
	BackendURL string `json:"backend_url"`
}

// ReadConfig returns the backend URL to connect to and the raw agent key
// token to send on every request. The token itself carries the tenant
// identity — the agent doesn't need to know or store anything else.
func ReadConfig() (backendURL string, agentToken string, err error) {
	token := os.Getenv("AURIGON_AGENT_KEY")
	if token == "" {
		return "", "", fmt.Errorf("AURIGON_AGENT_KEY is not set")
	}

	backendURL, err = decodeAgentTokenBackendURL(token)
	if err != nil {
		return "", "", fmt.Errorf("invalid AURIGON_AGENT_KEY: %w", err)
	}

	log.Printf("Config loaded from agent key (BackendURL: %s)", backendURL)
	return backendURL, token, nil
}

// decodeAgentTokenBackendURL extracts just the backend URL from an AGT- token
// so the agent knows where to connect. The full token is still sent as-is
// on every request — the backend re-derives tenant identity from it.
func decodeAgentTokenBackendURL(token string) (string, error) {
	if len(token) <= len(agentKeyPrefix) || token[:len(agentKeyPrefix)] != agentKeyPrefix {
		return "", fmt.Errorf("key must start with %s", agentKeyPrefix)
	}

	encoded := token[len(agentKeyPrefix):]
	payloadJSON, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("invalid base64 encoding")
	}

	var payload agentTokenPayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return "", fmt.Errorf("invalid key payload")
	}

	if payload.BackendURL == "" {
		return "", fmt.Errorf("key is missing backend_url")
	}

	return payload.BackendURL, nil
}