package service

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"aurigon-agent/internal/accounts"
	"aurigon-agent/internal/client"
)

type ActionRow struct {
	ID       int               `json:"id"`
	Type     string            `json:"type"`
	Username string            `json:"username"`
	Params   map[string]string `json:"params"`
}

type ActionResultRequest struct {
	ActionID int    `json:"action_id"`
	Status   string `json:"status"`
	Result   string `json:"result"`
}

type GroupInventoryRequest struct {
	DeviceID string                `json:"device_id"`
	Groups   []accounts.LocalGroup `json:"groups"`
}

type MachineInfoRequest struct {
	DeviceID       string   `json:"device_id"`
	Domain         string   `json:"domain"`
	IsDomainJoined bool     `json:"is_domain_joined"`
	IPAddresses    []string `json:"ip_addresses"`
	OSVersion      string   `json:"os_version"`
}

// RunWithStop is the main agent loop. Runs until stop channel is closed.
func RunWithStop(stop <-chan struct{}) error {
	backendURL, agentKey, deployKey, err := ReadConfig()
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	c := client.New(backendURL, agentKey, deployKey)

	deviceID, err := registerWithRetry(c, stop)
	if err != nil {
		return err
	}

	log.Println("Agent running. Polling every 30 seconds...")

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	runCycle(c, deviceID)

	for {
		select {
		case <-stop:
			log.Println("Stop signal received — shutting down")
			return nil
		case <-ticker.C:
			runCycle(c, deviceID)
		}
	}
}

func registerWithRetry(c *client.Client, stop <-chan struct{}) (string, error) {
	for {
		deviceID, err := register(c)
		if err == nil {
			return deviceID, nil
		}
		log.Printf("Registration failed: %v — retrying in 15s", err)
		select {
		case <-stop:
			return "", fmt.Errorf("stopped before registration succeeded")
		case <-time.After(15 * time.Second):
		}
	}
}

func runCycle(c *client.Client, deviceID string) {
	// Machine info (IP, domain, OS)
	info, err := accounts.EnumerateMachineInfo()
	if err != nil {
		log.Printf("Machine info enumeration failed: %v", err)
	} else {
		if err := uploadMachineInfo(c, deviceID, info); err != nil {
			log.Printf("Machine info upload failed: %v", err)
		} else {
			log.Printf("Machine info uploaded (IPs: %s, Domain: %s)",
				strings.Join(info.IPAddresses, ", "), info.Domain)
		}
	}

	// Accounts
	accs, err := accounts.Enumerate()
	if err != nil {
		log.Printf("Account enumeration failed: %v", err)
	} else {
		if err := uploadInventory(c, deviceID, accs); err != nil {
			log.Printf("Inventory upload failed: %v", err)
		} else {
			log.Printf("Uploaded %d accounts", len(accs))
		}
	}

	// Groups
	groups, err := accounts.EnumerateGroups()
	if err != nil {
		log.Printf("Group enumeration failed: %v", err)
	} else {
		if err := uploadGroups(c, deviceID, groups); err != nil {
			log.Printf("Group upload failed: %v", err)
		} else {
			log.Printf("Uploaded %d groups", len(groups))
		}
	}

	// Actions
	actions, err := pollActions(c, deviceID)
	if err != nil {
		log.Printf("Action poll failed: %v", err)
		return
	}
	for _, action := range actions {
		executeAction(c, action)
	}
}

// ── Backend communication ──────────────────────────────────────────────────

func register(c *client.Client) (string, error) {
	hostname := getHostname()
	respBytes, statusCode, err := c.Post("/register", map[string]string{"hostname": hostname})
	if err != nil {
		return "", err
	}
	if statusCode == http.StatusUnauthorized {
		return "", fmt.Errorf("authentication rejected — check deploy key or agent key")
	}
	if statusCode != http.StatusOK {
		return "", fmt.Errorf("registration failed (status %d): %s", statusCode, string(respBytes))
	}

	var reg client.RegisterResponse
	if err := json.Unmarshal(respBytes, &reg); err != nil {
		return "", fmt.Errorf("bad registration response: %v", err)
	}

	c.SetToken(reg.Token)
	log.Printf("Registered as %s (device: %s)", hostname, reg.DeviceID)
	return reg.DeviceID, nil
}

func uploadMachineInfo(c *client.Client, deviceID string, info *accounts.MachineInfo) error {
	req := MachineInfoRequest{
		DeviceID:       deviceID,
		Domain:         info.Domain,
		IsDomainJoined: info.IsDomainJoined,
		IPAddresses:    info.IPAddresses,
		OSVersion:      info.OSVersion,
	}
	_, statusCode, err := c.Post("/machine-info", req)
	if err != nil {
		return err
	}
	if statusCode != http.StatusOK {
		return fmt.Errorf("machine info upload failed (status %d)", statusCode)
	}
	return nil
}

func uploadInventory(c *client.Client, deviceID string, accs interface{}) error {
	req := client.InventoryRequest{DeviceID: deviceID, Accounts: accs}
	_, statusCode, err := c.Post("/inventory", req)
	if err != nil {
		return err
	}
	if statusCode != http.StatusOK {
		return fmt.Errorf("inventory upload failed (status %d)", statusCode)
	}
	return nil
}

func uploadGroups(c *client.Client, deviceID string, groups []accounts.LocalGroup) error {
	req := GroupInventoryRequest{DeviceID: deviceID, Groups: groups}
	_, statusCode, err := c.Post("/groups/inventory", req)
	if err != nil {
		return err
	}
	if statusCode != http.StatusOK {
		return fmt.Errorf("group upload failed (status %d)", statusCode)
	}
	return nil
}

func pollActions(c *client.Client, deviceID string) ([]ActionRow, error) {
	respBytes, statusCode, err := c.Get("/actions?device_id=" + deviceID)
	if err != nil {
		return nil, err
	}
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("action poll failed (status %d)", statusCode)
	}
	var actions []ActionRow
	if err := json.Unmarshal(respBytes, &actions); err != nil {
		return nil, fmt.Errorf("bad actions response: %v", err)
	}
	return actions, nil
}

func reportResult(c *client.Client, actionID int, status, result string) {
	req := ActionResultRequest{ActionID: actionID, Status: status, Result: result}
	if _, _, err := c.Post("/action-result", req); err != nil {
		log.Printf("Failed to report result for action %d: %v", actionID, err)
	}
}

func executeAction(c *client.Client, action ActionRow) {
	log.Printf("Executing action %d: %s on %s", action.ID, action.Type, action.Username)
	var err error
	switch action.Type {
	case "disable_account":
		err = runNetUser(action.Username, "/active:no")

	case "enable_account":
		err = runNetUser(action.Username, "/active:yes")

	case "create_account":
		password := action.Params["password"]
		isAdmin := action.Params["is_admin"] == "true"
		err = accounts.CreateAccount(action.Username, password, isAdmin)

	case "delete_account":
		err = accounts.DeleteAccount(action.Username)

	case "set_admin":
		err = accounts.SetAdminPrivilege(action.Username, true)

	case "remove_admin":
		err = accounts.SetAdminPrivilege(action.Username, false)

	default:
		reportResult(c, action.ID, "failed", fmt.Sprintf("unknown action type: %s", action.Type))
		return
	}

	if err != nil {
		log.Printf("Action %d failed: %v", action.ID, err)
		reportResult(c, action.ID, "failed", err.Error())
	} else {
		log.Printf("Action %d completed successfully", action.ID)
		reportResult(c, action.ID, "completed", "success")
	}
}

func runNetUser(username, flag string) error {
	cmd := exec.Command("net", "user", username, flag)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v (%s)", err, string(out))
	}
	return nil
}

func getHostname() string {
	cmd := exec.Command("hostname")
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	h := string(out)
	for len(h) > 0 && (h[len(h)-1] == '\n' || h[len(h)-1] == '\r') {
		h = h[:len(h)-1]
	}
	return h
}