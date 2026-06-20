package main

import (
    "encoding/json"
    "log"
    "net/http"
)

type RegisterResponse struct {
    DeviceID string `json:"device_id"`
    Token    string `json:"token"`
}

type InventoryRequest struct {
    DeviceID string        `json:"device_id"`
    Accounts []interface{} `json:"accounts"`
}

type Action struct {
    Type     string `json:"type"`
    Username string `json:"username"`
}

var lastInventory InventoryRequest

func registerHandler(w http.ResponseWriter, r *http.Request) {
    log.Println("Received /register request")

    resp := RegisterResponse{
        DeviceID: "device-123",
        Token:    "test-token",
    }

    json.NewEncoder(w).Encode(resp)
}

func inventoryHandler(w http.ResponseWriter, r *http.Request) {
    log.Println("Received /inventory request")

    var inv InventoryRequest
    if err := json.NewDecoder(r.Body).Decode(&inv); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    log.Printf("Inventory from device %s: %v\n", inv.DeviceID, inv.Accounts)
    lastInventory = inv
}

func accountsHandler(w http.ResponseWriter, r *http.Request) {
    log.Println("Dashboard requested accounts")
    if lastInventory.DeviceID == "" {
        http.Error(w, "no inventory yet", http.StatusNotFound)
        return
    }
    json.NewEncoder(w).Encode(lastInventory.Accounts)
}

func actionsHandler(w http.ResponseWriter, r *http.Request) {
    log.Println("Agent asked for actions")

    actions := []Action{
        {Type: "disable_account", Username: "Guest"},
    }

    json.NewEncoder(w).Encode(actions)
}

func main() {
    http.HandleFunc("/register", registerHandler)
    http.HandleFunc("/inventory", inventoryHandler)
    http.HandleFunc("/accounts", accountsHandler)
    http.HandleFunc("/actions", actionsHandler)

    log.Println("Backend running on http://localhost:8080")
    http.ListenAndServe(":8080", nil)
}
