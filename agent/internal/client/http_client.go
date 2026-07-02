package client

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

type Client struct {
	BaseURL   string
	Token     string
	AgentKey  string
	DeployKey string // AGT-... token, takes priority over AgentKey
	http      *http.Client
}

type RegisterResponse struct {
	DeviceID string `json:"device_id"`
	Token    string `json:"token"`
}

type InventoryRequest struct {
	DeviceID string      `json:"device_id"`
	Accounts interface{} `json:"accounts"`
}

func New(baseURL, agentKey, deployKey string) *Client {
	return &Client{
		BaseURL:   baseURL,
		AgentKey:  agentKey,
		DeployKey: deployKey,
		http:      &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) SetToken(token string) { c.Token = token }

func (c *Client) addAuthHeaders(req *http.Request) {
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	// Deploy key takes priority over legacy agent key
	if c.DeployKey != "" {
		req.Header.Set("X-Deploy-Key", c.DeployKey)
	} else if c.AgentKey != "" {
		req.Header.Set("X-Agent-Key", c.AgentKey)
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