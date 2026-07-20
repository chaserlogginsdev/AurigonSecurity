package accounts

type LocalAccount struct {
	Username    string `json:"username"`
	SID         string `json:"sid"`
	Enabled     bool   `json:"enabled"`
	Description string `json:"description"`
	FullName    string `json:"full_name"`
	LastLogon   string `json:"last_logon"`
	IsAdmin     bool   `json:"is_admin"`

	PasswordLastSet       string `json:"password_last_set"`
	PasswordExpiresDate   string `json:"password_expires_date"`
	AccountExpiresDate    string `json:"account_expires_date"`
	PasswordNeverExpires  bool   `json:"password_never_expires"`
	PasswordRequired      bool   `json:"password_required"`
	UserMayChangePassword bool   `json:"user_may_change_password"`
	DaysSinceLastLogon    int    `json:"days_since_last_logon"` // -1 = never logged on
	IsBuiltIn              bool  `json:"is_built_in"`
}

type LocalGroup struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Members     []string `json:"members"`
}

// MachineInfo holds network, domain, health, and security posture metadata
// collected from the machine. Uploaded on every inventory cycle.
type MachineInfo struct {
	Hostname       string   `json:"hostname"`
	Domain         string   `json:"domain"`         // empty if not domain-joined
	IsDomainJoined bool     `json:"is_domain_joined"`
	IPAddresses    []string `json:"ip_addresses"`   // all non-loopback IPv4 addresses
	OSVersion      string   `json:"os_version"`

	OSBuild                    string  `json:"os_build"`
	UptimeHours                float64 `json:"uptime_hours"`
	FreeDiskGB                 float64 `json:"free_disk_gb"`
	TotalMemoryGB              float64 `json:"total_memory_gb"`
	PendingReboot              bool    `json:"pending_reboot"`
	DefenderEnabled            bool    `json:"defender_enabled"`
	DefenderRealtimeProtection bool    `json:"defender_realtime_protection"`
	PasswordMinLength          int     `json:"password_min_length"`
	PasswordLockoutThreshold   int     `json:"password_lockout_threshold"`
	PasswordMaxAgeDays         int     `json:"password_max_age_days"`
	FailedLogonCount24h        int     `json:"failed_logon_count_24h"`
}

// SessionInfo represents one currently logged-on user session, as reported
// by `query user`. Uploaded on every cycle so the dashboard reflects live
// activity, not just account existence.
type SessionInfo struct {
	Username    string `json:"username"`
	SessionName string `json:"session_name"`
	ID          string `json:"id"`
	State       string `json:"state"`
	IdleTime    string `json:"idle_time"`
	LogonTime   string `json:"logon_time"`
}