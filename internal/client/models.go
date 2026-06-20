package client

type RegisterResponse struct {
    DeviceID string `json:"device_id"`
    Token    string `json:"token"`
}
