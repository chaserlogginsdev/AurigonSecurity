package accounts

type LocalAccount struct {
	Username    string `json:"username"`
	SID         string `json:"sid"`
	Enabled     bool   `json:"enabled"`
	Description string `json:"description"`
	LastLogon   string `json:"last_logon"`
	IsAdmin     bool   `json:"is_admin"`
}

type LocalGroup struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Members     []string `json:"members"`
}

// MachineInfo holds network and domain metadata collected from the machine.
// Uploaded on every inventory cycle so the dashboard always has current info.
type MachineInfo struct {
	Hostname       string   `json:"hostname"`
	Domain         string   `json:"domain"`         // empty if not domain-joined
	IsDomainJoined bool     `json:"is_domain_joined"`
	IPAddresses    []string `json:"ip_addresses"`   // all non-loopback IPv4 addresses
	OSVersion      string   `json:"os_version"`
}