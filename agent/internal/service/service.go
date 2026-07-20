package service

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
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

// getSyncInterval reads AURIGON_SYNC_INTERVAL_SECONDS to let an admin
// control how chatty the agent is. Defaults to 30s. Clamped to a 10s
// minimum so it can't be configured into hammering the backend.
func getSyncInterval() time.Duration {
	const defaultSeconds = 30
	const minSeconds = 10

	val := os.Getenv("AURIGON_SYNC_INTERVAL_SECONDS")
	if val == "" {
		return defaultSeconds * time.Second
	}

	seconds, err := strconv.Atoi(val)
	if err != nil || seconds < minSeconds {
		log.Printf("AURIGON_SYNC_INTERVAL_SECONDS invalid or below minimum (%ds) — using default %ds", minSeconds, defaultSeconds)
		return defaultSeconds * time.Second
	}

	return time.Duration(seconds) * time.Second
}

// RunWithStop is the main agent loop. Runs until stop channel is closed.
func RunWithStop(stop <-chan struct{}) error {
	backendURL, agentToken, err := ReadConfig()
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	c := client.New(backendURL, agentToken)

	deviceID, err := registerWithRetry(c, stop)
	if err != nil {
		return err
	}

	syncInterval := getSyncInterval()
	log.Printf("Agent running. Polling every %v...", syncInterval)

	ticker := time.NewTicker(syncInterval)
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
	// Machine info (IP, domain, OS, health, security posture)
	info, err := accounts.EnumerateMachineInfo()
	if err != nil {
		log.Printf("Machine info enumeration failed: %v", err)
	} else {
		if err := uploadMachineInfo(c, deviceID, info); err != nil {
			log.Printf("Machine info upload failed: %v", err)
		} else {
			log.Printf(
				"Machine info uploaded — IPs: %s | OS: %s (build %s) | Uptime: %.1fh | Disk free: %.1fGB | RAM: %.1fGB | Reboot pending: %t | Defender: %t (realtime: %t) | Password policy: min=%d lockout=%d maxage=%d | Failed logons (24h): %d",
				strings.Join(info.IPAddresses, ", "), info.OSVersion, info.OSBuild,
				info.UptimeHours, info.FreeDiskGB, info.TotalMemoryGB, info.PendingReboot,
				info.DefenderEnabled, info.DefenderRealtimeProtection,
				info.PasswordMinLength, info.PasswordLockoutThreshold, info.PasswordMaxAgeDays,
				info.FailedLogonCount24h,
			)
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

	// Sessions (currently logged-on users)
	sessions, err := accounts.EnumerateSessions()
	if err != nil {
		log.Printf("Session enumeration failed: %v", err)
	} else {
		if err := uploadSessions(c, deviceID, sessions); err != nil {
			log.Printf("Session upload failed: %v", err)
		} else {
			log.Printf("Uploaded %d active session(s)", len(sessions))
		}
	}

	// Actions
	actions, err := pollActions(c, deviceID)
	if err != nil {
		log.Printf("Action poll failed: %v", err)
		return
	}
	if len(actions) > 0 {
		for _, action := range actions {
			executeAction(c, action)
		}
		// Actions complete almost instantly, but the accounts/groups/sessions
		// tables in the backend only reflect reality on the next inventory
		// sync — which runs on its own independent clock. Without this,
		// the dashboard's "Pending" indicator can clear (action done) while
		// the account still visibly shows stale data for up to another full
		// cycle. Re-sync immediately so the two stay in step.
		resyncAfterActions(c, deviceID)
	}
}

// resyncAfterActions re-uploads current state right after executing actions,
// so the dashboard reflects the change within seconds instead of waiting
// for the next scheduled 30s cycle.
func resyncAfterActions(c *client.Client, deviceID string) {
	if accs, err := accounts.Enumerate(); err == nil {
		if err := uploadInventory(c, deviceID, accs); err != nil {
			log.Printf("Post-action inventory resync failed: %v", err)
		}
	}
	if groups, err := accounts.EnumerateGroups(); err == nil {
		if err := uploadGroups(c, deviceID, groups); err != nil {
			log.Printf("Post-action group resync failed: %v", err)
		}
	}
	if sessions, err := accounts.EnumerateSessions(); err == nil {
		if err := uploadSessions(c, deviceID, sessions); err != nil {
			log.Printf("Post-action session resync failed: %v", err)
		}
	}
	log.Println("Post-action resync complete")
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

		OSBuild:                    info.OSBuild,
		UptimeHours:                info.UptimeHours,
		FreeDiskGB:                 info.FreeDiskGB,
		TotalMemoryGB:              info.TotalMemoryGB,
		PendingReboot:              info.PendingReboot,
		DefenderEnabled:            info.DefenderEnabled,
		DefenderRealtimeProtection: info.DefenderRealtimeProtection,
		PasswordMinLength:          info.PasswordMinLength,
		PasswordLockoutThreshold:   info.PasswordLockoutThreshold,
		PasswordMaxAgeDays:         info.PasswordMaxAgeDays,
		FailedLogonCount24h:        info.FailedLogonCount24h,
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

type SessionsRequest struct {
	DeviceID string                 `json:"device_id"`
	Sessions []accounts.SessionInfo `json:"sessions"`
}

func uploadSessions(c *client.Client, deviceID string, sessions []accounts.SessionInfo) error {
	req := SessionsRequest{DeviceID: deviceID, Sessions: sessions}
	_, statusCode, err := c.Post("/sessions", req)
	if err != nil {
		return err
	}
	if statusCode != http.StatusOK {
		return fmt.Errorf("session upload failed (status %d)", statusCode)
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

	case "reset_password":
		password := action.Params["password"]
		err = accounts.ResetPassword(action.Username, password)

	case "require_password_change":
		err = accounts.RequirePasswordChangeAtNextLogon(action.Username)

	case "unlock_account":
		err = accounts.UnlockAccount(action.Username)

	case "rename_account":
		newUsername := action.Params["new_username"]
		err = accounts.RenameAccount(action.Username, newUsername)

	case "update_account_details":
		fullName := action.Params["full_name"]
		description := action.Params["description"]
		err = accounts.UpdateAccountDetails(action.Username, fullName, description)

	case "set_password_never_expires":
		neverExpires := action.Params["never_expires"] == "true"
		err = accounts.SetPasswordNeverExpires(action.Username, neverExpires)

	case "set_account_expiration":
		expires := action.Params["expires"]
		err = accounts.SetAccountExpiration(action.Username, expires)

	case "force_logoff":
		err = accounts.ForceLogoff(action.Username)

	case "add_to_group":
		group := action.Params["group"]
		err = accounts.AddToGroup(action.Username, group)

	case "remove_from_group":
		group := action.Params["group"]
		err = accounts.RemoveFromGroup(action.Username, group)

	case "create_group":
		// Group actions reuse the "username" field to carry the group name
		description := action.Params["description"]
		err = accounts.CreateGroup(action.Username, description)

	case "delete_group":
		err = accounts.DeleteGroup(action.Username)

	default:
		reportResult(c, action.ID, "failed", fmt.Sprintf("unknown action type: %s", action.Type))
		return
	}

	if err != nil {
		log.Printf("Action %d failed: %v", action.ID, err)
		reportResult(c, action.ID, "failed", err.Error())
	} else {
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