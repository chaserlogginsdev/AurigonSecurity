package service

import (
    "encoding/json"
    "log"
    "time"

    "aurigon-agent/internal/accounts"
    "aurigon-agent/internal/client"
)

func register(c *client.Client) (string, error) {
    respBytes, err := c.Post("/register", map[string]string{})
    if err != nil {
        return "", err
    }

    var reg client.RegisterResponse
    if err := json.Unmarshal(respBytes, &reg); err != nil {
        return "", err
    }

    c.SetToken(reg.Token)
    return reg.DeviceID, nil
}

func uploadInventory(c *client.Client, deviceID string, accounts interface{}) error {
    req := client.InventoryRequest{
        DeviceID: deviceID,
        Accounts: accounts,
    }

    _, err := c.Post("/inventory", req)
    return err
}

func pollActions(c *client.Client, deviceID string) ([]map[string]interface{}, error) {
    respBytes, err := c.Get("/actions?device_id=" + deviceID)
    if err != nil {
        return nil, err
    }

    var actions []map[string]interface{}
    if err := json.Unmarshal(respBytes, &actions); err != nil {
        return nil, err
    }

    return actions, nil
}

func executeAction(action map[string]interface{}) {
    switch action["type"] {
    case "disable_account":
        username := action["username"].(string)
        log.Println("Disabling account:", username)
        accounts.Disable(username)
    default:
        log.Println("Unknown action:", action["type"])
    }
}

func Run() error {
    c := client.New("http://localhost:8080")

    deviceID, err := register(c)
    if err != nil {
        log.Println("Registration failed:", err)
        return err
    }

    log.Println("Registered with device ID:", deviceID)

    for {
        accs, err := accounts.Enumerate()
        if err != nil {
            log.Println("Error enumerating accounts:", err)
        } else {
            log.Println("Found accounts:", len(accs))
            for _, a := range accs {
                log.Printf("User: %s | Admin: %v | Enabled: %v\n", a.Username, a.IsAdmin, a.Enabled)
            }

            if err := uploadInventory(c, deviceID, accs); err != nil {
                log.Println("Failed to upload inventory:", err)
            } else {
                log.Println("Inventory uploaded successfully")
            }
        }

        // ⭐ ACTION POLLING BLOCK (correct placement)
        actions, err := pollActions(c, deviceID)
        if err != nil {
            log.Println("Failed to poll actions:", err)
        } else {
            for _, action := range actions {
                executeAction(action)
            }
        }

        time.Sleep(30 * time.Second)
    }
}
