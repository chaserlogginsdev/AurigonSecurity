package service

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"time"

	"aurigon-agent/internal/accounts"
	"aurigon-agent/internal/client"
)

type ActionRow struct {
	ID       int    `json:"id"`
	Type     string `json:"type"`
	Username string `json:"username"`
}

type ActionResultRequest struct {
	ActionID int    `json:"action_id"`
	Status   string `json:"status"`
	Result   string `json:"result"`
}

func register(c *client.Client) (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	respBytes, err := c.Post("/register", map[string]string{"hostname": hostname})
	if err != nil {
		return "", err
	}

	var reg client.RegisterResponse
	if err := json.Unmarshal(respBytes, &reg); err != nil {
		return "", err
	}

	c.SetToken(reg.Token)
	log.Printf("Registered as %s (device: %s)\n", hostname, reg.DeviceID)
	return reg.DeviceID, nil
}

func uploadInventory(c *client.Client, deviceID string, accs interface{}) error {
	req := client.InventoryRequest{DeviceID: deviceID, Accounts: accs}
	_, err := c.Post("/inventory", req)
	return err
}

func pollActions(c *client.Client, deviceID string) ([]ActionRow, error) {
	respBytes, err := c.Get("/actions?device_id=" + deviceID)
	if err != nil {
		return nil, err
	}
	var actions []ActionRow
	if err := json.Unmarshal(respBytes, &actions); err != nil {
		return nil, err
	}
	return actions, nil
}

func reportResult(c *client.Client, actionID int, status, result string) {
	req := ActionResultRequest{ActionID: actionID, Status: status, Result: result}
	body, _ := json.Marshal(req)
	_, err := c.PostRaw("/action-result", body)
	if err != nil {
		log.Printf("Failed to report action result: %v\n", err)
	}
}

func executeAction(c *client.Client, action ActionRow) {
	log.Printf("Executing action %d: %s %s\n", action.ID, action.Type, action.Username)

	var cmd *exec.Cmd
	switch action.Type {
	case "disable_account":
		cmd = exec.Command("net", "user", action.Username, "/active:no")
	case "enable_account":
		cmd = exec.Command("net", "user", action.Username, "/active:yes")
	default:
		log.Printf("Unknown action type: %s\n", action.Type)
		reportResult(c, action.ID, "failed", "unknown action type: "+action.Type)
		return
	}

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()

	if err != nil {
		msg := out.String()
		log.Printf("Action %d failed: %v — %s\n", action.ID, err, msg)
		reportResult(c, action.ID, "failed", msg)
	} else {
		log.Printf("Action %d succeeded: %s %s\n", action.ID, action.Type, action.Username)
		reportResult(c, action.ID, "completed", "success")
		// Trigger immediate re-enumeration so dashboard reflects the change
		accs, err := accounts.Enumerate()
		if err == nil {
			uploadInventory(c, "", accs) // deviceID will be re-set from token
		}
	}
}

func Run() error {
	c := client.New("http://localhost:8080")

	deviceID, err := register(c)
	if err != nil {
		return err
	}

	for {
		// Enumerate and upload
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