package service

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
)

const deployKeyPrefix = "AGT-"

// deployKeyPayload mirrors the backend's DeployKeyPayload struct
type deployKeyPayload struct {
	ID         string `json:"id"`
	BackendURL string `json:"backend_url"`
	Key        string `json:"key"`
}

// ReadConfig returns BackendURL and AgentKey for the agent.
//
// Priority:
//  1. AURIGON_DEPLOY_KEY env var (new — single tenant key)
//  2. AURIGON_BACKEND_URL + AURIGON_AGENT_KEY env vars (legacy)
func ReadConfig() (backendURL string, agentKey string, deployKey string, err error) {
	// 1. Try deploy key first
	dk := os.Getenv("AURIGON_DEPLOY_KEY")
	if dk != "" {
		backendURL, agentKey, err = decodeDeployKey(dk)
		if err != nil {
			return "", "", "", fmt.Errorf("invalid AURIGON_DEPLOY_KEY: %w", err)
		}
		log.Printf("Config loaded from deploy key (BackendURL: %s)", backendURL)
		return backendURL, agentKey, dk, nil
	}

	// 2. Fall back to legacy env vars
	backendURL = os.Getenv("AURIGON_BACKEND_URL")
	agentKey = os.Getenv("AURIGON_AGENT_KEY")

	if backendURL == "" {
		backendURL = "http://localhost:8080"
		log.Println("AURIGON_BACKEND_URL not set, defaulting to http://localhost:8080")
	}
	if agentKey == "" {
		return "", "", "", fmt.Errorf("no auth configured: set AURIGON_DEPLOY_KEY or AURIGON_AGENT_KEY")
	}

	log.Printf("Config loaded from environment (BackendURL: %s)", backendURL)
	return backendURL, agentKey, "", nil
}

// decodeDeployKey decodes an AGT-... deploy key into its components.
func decodeDeployKey(key string) (backendURL, agentKey string, err error) {
	if len(key) <= len(deployKeyPrefix) || key[:len(deployKeyPrefix)] != deployKeyPrefix {
		return "", "", fmt.Errorf("key must start with %s", deployKeyPrefix)
	}

	encoded := key[len(deployKeyPrefix):]
	payloadJSON, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return "", "", fmt.Errorf("invalid base64 encoding")
	}

	var payload deployKeyPayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return "", "", fmt.Errorf("invalid key payload")
	}

	if payload.BackendURL == "" || payload.Key == "" {
		return "", "", fmt.Errorf("key is missing required fields")
	}

	return payload.BackendURL, payload.Key, nil
}