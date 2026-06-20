package client

type InventoryRequest struct {
    DeviceID string      `json:"device_id"`
    Accounts interface{} `json:"accounts"`
}
