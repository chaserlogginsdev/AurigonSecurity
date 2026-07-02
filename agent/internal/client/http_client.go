package client

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

type Client struct {
	BaseURL  string
	Token    string
	AgentKey string
	http     *http.Client
}

func New(baseURL string, agentKey string) *Client {
	return &Client{
		BaseURL:  baseURL,
		AgentKey: agentKey,
		http:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) SetToken(token string) { c.Token = token }

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
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if c.AgentKey != "" {
		req.Header.Set("X-Agent-Key", c.AgentKey)
	}
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
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if c.AgentKey != "" {
		req.Header.Set("X-Agent-Key", c.AgentKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	return data, resp.StatusCode, err
}