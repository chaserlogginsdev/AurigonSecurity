package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
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
	DeviceID string               `json:"device_id"`
	Groups   []accounts.LocalGroup `json:"groups"`
}

func getBackendURL() string {
	url := os.Getenv("AURIGON_BACKEND_URL")
	if url == "" {
		url = "http://localhost:8080"
		log.Println("AURIGON_BACKEND_URL not set, using default: http://localhost:8080")
	}
	return url
}

func register(c *client.Client) (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	respBytes, statusCode, err := c.Post("/register", map[string]string{"hostname": hostname})
	if err != nil {
		return "", err
	}
	if statusCode == http.StatusUnauthorized {
		return "", fmt.Errorf("agent key rejected by backend — check AURIGON_AGENT_KEY")
	}
	if statusCode != http.StatusOK {
		return "", fmt.Errorf("registration failed with status %d: %s", statusCode, string(respBytes))
	}

	var reg client.RegisterResponse
	if err := json.Unmarshal(respBytes, &reg); err != nil {
		return "", fmt.Errorf("failed to parse registration response: %v", err)
	}

	c.SetToken(reg.Token)
	log.Printf("Registered as %s (device: %s)\n", hostname, reg.DeviceID)
	return reg.DeviceID, nil
}

func uploadInventory(c *client.Client, deviceID string, accs interface{}) error {
	req := client.InventoryRequest{DeviceID: deviceID, Accounts: accs}
	_, statusCode, err := c.Post("/inventory", req)
	if err != nil {
		return err
	}
	if statusCode != http.StatusOK {
		return fmt.Errorf("inventory upload failed with status %d", statusCode)
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
		return fmt.Errorf("group inventory upload failed with status %d", statusCode)
	}
	return nil
}

func pollActions(c *client.Client, deviceID string) ([]ActionRow, error) {
	respBytes, statusCode, err := c.Get("/actions?device_id=" + deviceID)
	if err != nil {
		return nil, err
	}
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("polling actions failed with status %d", statusCode)
	}
	var actions []ActionRow
	if err := json.Unmarshal(respBytes, &actions); err != nil {
		return nil, fmt.Errorf("failed to parse actions response: %v", err)
	}
	return actions, nil
}

func reportResult(c *client.Client, actionID int, status, result string) {
	req := ActionResultRequest{ActionID: actionID, Status: status, Result: result}
	body, _ := json.Marshal(req)
	_, _, err := c.PostRaw("/action-result", body)
	if err != nil {
		log.Printf("Failed to report action result: %v\n", err)
	}
}

func runCommand(name string, args ...string) (string, error) {
	var out bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return strings.TrimSpace(out.String()), err
}

func executeAction(c *client.Client, action ActionRow) {
	log.Printf("Executing action %d: %s %s\n", action.ID, action.Type, action.Username)

	var (
		output string
		err    error
	)

	switch action.Type {
	case "disable_account":
		output, err = runCommand("net", "user", action.Username, "/active:no")

	case "enable_account":
		output, err = runCommand("net", "user", action.Username, "/active:yes")

	case "delete_account":
		output, err = runCommand("net", "user", action.Username, "/delete")

	case "create_account":
		password := action.Params["password"]
		isAdmin := action.Params["is_admin"]
		if password == "" {
			reportResult(c, action.ID, "failed", "no password provided")
			return
		}
		output, err = runCommand("net", "user", action.Username, password, "/add")
		if err != nil {
			break
		}
		if isAdmin == "true" {
			adminOut, adminErr := runCommand("net", "localgroup", "Administrators", action.Username, "/add")
			if adminErr != nil {
				output += " | admin group error: " + adminOut
			}
		}

	case "create_group":
		desc := action.Params["description"]
		if desc != "" {
			output, err = runCommand("net", "localgroup", action.Username, "/add", "/comment:"+desc)
		} else {
			output, err = runCommand("net", "localgroup", action.Username, "/add")
		}

	case "delete_group":
		output, err = runCommand("net", "localgroup", action.Username, "/delete")

	case "add_to_group":
		group := action.Params["group"]
		if group == "" {
			reportResult(c, action.ID, "failed", "no group specified")
			return
		}
		output, err = runCommand("net", "localgroup", group, action.Username, "/add")

	case "remove_from_group":
		group := action.Params["group"]
		if group == "" {
			reportResult(c, action.ID, "failed", "no group specified")
			return
		}
		output, err = runCommand("net", "localgroup", group, action.Username, "/delete")

	default:
		log.Printf("Unknown action type: %s\n", action.Type)
		reportResult(c, action.ID, "failed", "unknown action type: "+action.Type)
		return
	}

	if err != nil {
		log.Printf("Action %d failed: %v — %s\n", action.ID, err, output)
		reportResult(c, action.ID, "failed", output)
	} else {
		log.Printf("Action %d succeeded: %s %s\n", action.ID, action.Type, action.Username)
		reportResult(c, action.ID, "completed", "success")
	}
}

func Run() error {
	agentKey := os.Getenv("AURIGON_AGENT_KEY")
	if agentKey == "" {
		log.Println("WARNING: AURIGON_AGENT_KEY not set — agent will be rejected if backend requires it")
	}

	backendURL := getBackendURL()
	c := client.New(backendURL, agentKey)

	deviceID, err := register(c)
	if err != nil {
		return err
	}

	for {
		// Enumerate and upload accounts
		accs, err := accounts.Enumerate()
		if err != nil {
			log.Println("Error enumerating accounts:", err)
		} else {
			log.Printf("Found %d accounts — uploading...\n", len(accs))
			if err := uploadInventory(c, deviceID, accs); err != nil {
				log.Println("Failed to upload inventory:", err)
			} else {
				log.Println("Inventory uploaded successfully")
			}
		}

		// Enumerate and upload groups
		groups, err := accounts.EnumerateGroups()
		if err != nil {
			log.Println("Error enumerating groups:", err)
		} else {
			log.Printf("Found %d groups — uploading...\n", len(groups))
			if err := uploadGroups(c, deviceID, groups); err != nil {
				log.Println("Failed to upload groups:", err)
			} else {
				log.Println("Groups uploaded successfully")
			}
		}

		// Poll and execute actions
		actions, err := pollActions(c, deviceID)
		if err != nil {
			log.Println("Failed to poll actions:", err)
		} else if len(actions) > 0 {
			log.Printf("Got %d pending action(s)\n", len(actions))
			for _, action := range actions {
				executeAction(c, action)
			}
		}

		time.Sleep(30 * time.Second)
	}
}