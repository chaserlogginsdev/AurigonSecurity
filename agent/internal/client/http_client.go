package client

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type Client struct {
	BaseURL    string
	Token      string // per-machine session token, set after registration
	AgentToken string // the AGT-... tenant key, sent on every request
	http       *http.Client
}

type RegisterResponse struct {
	DeviceID string `json:"device_id"`
	Token    string `json:"token"`
}

type InventoryRequest struct {
	DeviceID string      `json:"device_id"`
	Accounts interface{} `json:"accounts"`
}

func New(baseURL, agentToken string) *Client {
	httpClient := &http.Client{Timeout: 10 * time.Second}

	// DEV/TEST ONLY: allows the agent to trust a self-signed certificate
	// while testing HTTPS locally, before a real CA-signed cert is in
	// place. Never set this in a real deployment — it disables
	// certificate validation entirely, which defeats the point of TLS.
	if os.Getenv("AURIGON_INSECURE_SKIP_VERIFY") == "true" {
		log.Println("WARNING: AURIGON_INSECURE_SKIP_VERIFY is enabled — TLS certificate validation is OFF.")
		log.Println("WARNING: this must only be used for local testing with a self-signed cert. Never use in production.")
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	return &Client{
		BaseURL:    baseURL,
		AgentToken: agentToken,
		http:       httpClient,
	}
}

func (c *Client) SetToken(token string) { c.Token = token }

func (c *Client) addAuthHeaders(req *http.Request) {
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if c.AgentToken != "" {
		req.Header.Set("X-Agent-Key", c.AgentToken)
	}
}

func (c *Client) Post(path string, body interface{}) ([]byte, int, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, 0, err
	}
	return c.PostRaw(path, jsonBody)
}

func (c *Client) PostRaw(path string, body []byte) ([]byte, int, error) {
	req, err := http.NewRequest("POST", c.BaseURL+path, bytes.NewBuffer(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	c.addAuthHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	return data, resp.StatusCode, err
}

func (c *Client) Get(path string) ([]byte, int, error) {
	req, err := http.NewRequest("GET", c.BaseURL+path, nil)
	if err != nil {
		return nil, 0, err
	}
	c.addAuthHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	return data, resp.StatusCode, err
}