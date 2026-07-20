package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
)

// ── Types ──────────────────────────────────────────────────────────────────

type RegisterRequest struct {
	Hostname string `json:"hostname"`
}

type RegisterResponse struct {
	DeviceID string `json:"device_id"`
	Token    string `json:"token"`
}

type AccountPayload struct {
	Username    string `json:"username"`
	SID         string `json:"sid"`
	Enabled     bool   `json:"enabled"`
	IsAdmin     bool   `json:"is_admin"`
	Description string `json:"description"`
	FullName    string `json:"full_name"`
	LastLogon   string `json:"last_logon"`

	PasswordLastSet        string `json:"password_last_set"`
	PasswordExpiresDate    string `json:"password_expires_date"`
	AccountExpiresDate     string `json:"account_expires_date"`
	PasswordNeverExpires   bool   `json:"password_never_expires"`
	PasswordRequired       bool   `json:"password_required"`
	UserMayChangePassword  bool   `json:"user_may_change_password"`
	DaysSinceLastLogon     int    `json:"days_since_last_logon"`
	IsBuiltIn              bool   `json:"is_built_in"`

	// Computed at read time — not stored as their own columns.
	Groups         []string `json:"groups"`
	IsStale        bool     `json:"is_stale"`         // enabled but unused for 90+ days
	IsPasswordRisk bool     `json:"is_password_risk"` // enabled account that allows a blank password
}

type InventoryRequest struct {
	DeviceID string           `json:"device_id"`
	Accounts []AccountPayload `json:"accounts"`
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

type ActionRow struct {
	ID         int               `json:"id"`
	MachineID  string            `json:"machine_id"`
	Hostname   string            `json:"hostname"`
	Type       string            `json:"type"`
	Username   string            `json:"username"`
	Params     map[string]string `json:"params,omitempty"`
	CreatedBy  string            `json:"created_by"`
	Status     string            `json:"status"`
	CreatedAt  string            `json:"created_at"`
	ExecutedAt string            `json:"executed_at"`
	Result     string            `json:"result"`
}

type CreateActionRequest struct {
	MachineID string            `json:"machine_id"`
	Type      string            `json:"type"`
	Username  string            `json:"username"`
	Params    map[string]string `json:"params"`
}

type ActionResultRequest struct {
	ActionID int    `json:"action_id"`
	Status   string `json:"status"`
	Result   string `json:"result"`
}

type MachineRow struct {
	ID             string   `json:"id"`
	Hostname       string   `json:"hostname"`
	Domain         string   `json:"domain"`
	IsDomainJoined bool     `json:"is_domain_joined"`
	IPAddresses    []string `json:"ip_addresses"`
	OSVersion      string   `json:"os_version"`
	LastSeen       string   `json:"last_seen"`

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

// validActionTypes lists every action the agent knows how to execute.
// Shared between the single-action and bulk-action endpoints.
var validActionTypes = map[string]bool{
	"disable_account":          true,
	"enable_account":           true,
	"create_account":           true,
	"delete_account":           true,
	"set_admin":                true,
	"remove_admin":             true,
	"reset_password":           true,
	"require_password_change":  true,
	"unlock_account":           true,
	"rename_account":           true,
	"update_account_details":   true,
	"set_password_never_expires": true,
	"set_account_expiration":   true,
	"force_logoff":             true,
	"add_to_group":             true,
	"remove_from_group":        true,
	"create_group":             true,
	"delete_group":             true,
}

// actionTypesRequiringPassword must include a "password" param of at least 8 chars.
var actionTypesRequiringPassword = map[string]bool{
	"create_account": true,
	"reset_password": true,
}

type SessionPayload struct {
	Username    string `json:"username"`
	SessionName string `json:"session_name"`
	ID          string `json:"id"`
	State       string `json:"state"`
	IdleTime    string `json:"idle_time"`
	LogonTime   string `json:"logon_time"`
}

type SessionsRequest struct {
	DeviceID string           `json:"device_id"`
	Sessions []SessionPayload `json:"sessions"`
}

// ── Agent handlers — use tenant DB from context ────────────────────────────

func registerHandler(w http.ResponseWriter, r *http.Request) {
	db := dbFromCtx(r)

	var req RegisterRequest
	json.NewDecoder(r.Body).Decode(&req)
	if req.Hostname == "" {
		req.Hostname = "unknown"
	}

	var existingID, existingToken string
	err := db.QueryRow(`SELECT id, token FROM machines WHERE hostname = ?`, req.Hostname).
		Scan(&existingID, &existingToken)
	if err == nil {
		db.Exec(`UPDATE machines SET last_seen = CURRENT_TIMESTAMP WHERE id = ?`, existingID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(RegisterResponse{DeviceID: existingID, Token: existingToken})
		return
	}

	id := randomHex(6)
	token := randomHex(16)
	db.Exec(`INSERT INTO machines (id, hostname, token) VALUES (?, ?, ?)`, id, req.Hostname, token)
	log.Printf("New machine: %s (%s) in tenant %s", req.Hostname, id, tenantIDFromCtx(r))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(RegisterResponse{DeviceID: id, Token: token})
}

func machineInfoHandler(w http.ResponseWriter, r *http.Request) {
	db := dbFromCtx(r)

	var req MachineInfoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ipStr := strings.Join(req.IPAddresses, ",")
	isDomainInt := boolToInt(req.IsDomainJoined)
	pendingRebootInt := boolToInt(req.PendingReboot)
	defenderEnabledInt := boolToInt(req.DefenderEnabled)
	defenderRealtimeInt := boolToInt(req.DefenderRealtimeProtection)

	db.Exec(`
		UPDATE machines SET
			domain = ?, is_domain_joined = ?,
			ip_addresses = ?, os_version = ?,
			os_build = ?, uptime_hours = ?, free_disk_gb = ?, total_memory_gb = ?,
			pending_reboot = ?, defender_enabled = ?, defender_realtime_protection = ?,
			password_min_length = ?, password_lockout_threshold = ?, password_max_age_days = ?,
			failed_logon_count_24h = ?,
			last_seen = CURRENT_TIMESTAMP
		WHERE id = ?
	`, req.Domain, isDomainInt, ipStr, req.OSVersion,
		req.OSBuild, req.UptimeHours, req.FreeDiskGB, req.TotalMemoryGB,
		pendingRebootInt, defenderEnabledInt, defenderRealtimeInt,
		req.PasswordMinLength, req.PasswordLockoutThreshold, req.PasswordMaxAgeDays,
		req.FailedLogonCount24h,
		req.DeviceID)

	w.WriteHeader(http.StatusOK)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// sessionsHandler receives the agent's current session list each cycle.
// Full replace each time — sessions are point-in-time, not historical.
func sessionsHandler(w http.ResponseWriter, r *http.Request) {
	db := dbFromCtx(r)

	var req SessionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	db.Exec(`DELETE FROM sessions WHERE machine_id = ?`, req.DeviceID)
	for _, s := range req.Sessions {
		db.Exec(`
			INSERT INTO sessions (machine_id, username, session_name, session_id, state, idle_time, logon_time)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, req.DeviceID, s.Username, s.SessionName, s.ID, s.State, s.IdleTime, s.LogonTime)
	}

	w.WriteHeader(http.StatusOK)
}

// sessionsListHandler returns current sessions for a machine (dashboard-facing).
func sessionsListHandler(w http.ResponseWriter, r *http.Request) {
	db := dbFromCtx(r)
	machineID := r.URL.Query().Get("machine_id")
	if machineID == "" {
		http.Error(w, "machine_id required", http.StatusBadRequest)
		return
	}

	rows, err := db.Query(`
		SELECT username, session_name, session_id, state, idle_time, logon_time
		FROM sessions WHERE machine_id = ? ORDER BY username
	`, machineID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	sessions := []SessionPayload{}
	for rows.Next() {
		var s SessionPayload
		rows.Scan(&s.Username, &s.SessionName, &s.ID, &s.State, &s.IdleTime, &s.LogonTime)
		sessions = append(sessions, s)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessions)
}

func inventoryHandler(w http.ResponseWriter, r *http.Request) {
	db := dbFromCtx(r)

	var inv InventoryRequest
	if err := json.NewDecoder(r.Body).Decode(&inv); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	db.Exec(`UPDATE machines SET last_seen = CURRENT_TIMESTAMP WHERE id = ?`, inv.DeviceID)

	for _, a := range inv.Accounts {
		db.Exec(`
			INSERT INTO accounts (
				machine_id, username, sid, enabled, is_admin, description, full_name, last_logon,
				password_last_set, password_expires_date, account_expires_date,
				password_never_expires, password_required, user_may_change_password,
				days_since_last_logon, is_built_in, updated_at
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(machine_id, username) DO UPDATE SET
				sid=excluded.sid, enabled=excluded.enabled, is_admin=excluded.is_admin,
				description=excluded.description, full_name=excluded.full_name, last_logon=excluded.last_logon,
				password_last_set=excluded.password_last_set,
				password_expires_date=excluded.password_expires_date,
				account_expires_date=excluded.account_expires_date,
				password_never_expires=excluded.password_never_expires,
				password_required=excluded.password_required,
				user_may_change_password=excluded.user_may_change_password,
				days_since_last_logon=excluded.days_since_last_logon,
				is_built_in=excluded.is_built_in,
				updated_at=CURRENT_TIMESTAMP
		`, inv.DeviceID, a.Username, a.SID, a.Enabled, a.IsAdmin, a.Description, a.FullName, a.LastLogon,
			a.PasswordLastSet, a.PasswordExpiresDate, a.AccountExpiresDate,
			boolToInt(a.PasswordNeverExpires), boolToInt(a.PasswordRequired), boolToInt(a.UserMayChangePassword),
			a.DaysSinceLastLogon, boolToInt(a.IsBuiltIn))
	}

	log.Printf("Inventory: %s — %d accounts", inv.DeviceID, len(inv.Accounts))
	w.WriteHeader(http.StatusOK)
}

func agentActionsHandler(w http.ResponseWriter, r *http.Request) {
	db := dbFromCtx(r)
	deviceID := r.URL.Query().Get("device_id")
	if deviceID == "" {
		json.NewEncoder(w).Encode([]ActionRow{})
		return
	}

	rows, err := db.Query(`
		SELECT id, machine_id, '', type, username, COALESCE(params, '{}'),
			created_by, status, created_at,
			COALESCE(executed_at, ''), COALESCE(result, '')
		FROM actions WHERE machine_id = ? AND status = 'pending'
		ORDER BY created_at ASC
	`, deviceID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	actions := []ActionRow{}
	for rows.Next() {
		var a ActionRow
		var paramsJSON string
		rows.Scan(&a.ID, &a.MachineID, &a.Hostname, &a.Type, &a.Username, &paramsJSON,
			&a.CreatedBy, &a.Status, &a.CreatedAt, &a.ExecutedAt, &a.Result)
		var params map[string]string
		if err := json.Unmarshal([]byte(paramsJSON), &params); err == nil {
			a.Params = params
		}
		actions = append(actions, a)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(actions)
}

func actionResultHandler(w http.ResponseWriter, r *http.Request) {
	db := dbFromCtx(r)

	var req ActionResultRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	db.Exec(`
		UPDATE actions SET status = ?, result = ?, executed_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, req.Status, req.Result, req.ActionID)
	log.Printf("Action %d: %s", req.ActionID, req.Status)
	w.WriteHeader(http.StatusOK)
}

// ── Dashboard handlers ─────────────────────────────────────────────────────

func machinesHandler(w http.ResponseWriter, r *http.Request) {
	db := dbFromCtx(r)

	rows, err := db.Query(`
		SELECT id, hostname, domain, is_domain_joined, ip_addresses, os_version, last_seen,
			os_build, uptime_hours, free_disk_gb, total_memory_gb,
			pending_reboot, defender_enabled, defender_realtime_protection,
			password_min_length, password_lockout_threshold, password_max_age_days,
			failed_logon_count_24h
		FROM machines ORDER BY last_seen DESC
	`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	machines := []MachineRow{}
	for rows.Next() {
		var m MachineRow
		var ipStr string
		var isDomainInt, pendingRebootInt, defenderEnabledInt, defenderRealtimeInt int
		rows.Scan(&m.ID, &m.Hostname, &m.Domain, &isDomainInt, &ipStr, &m.OSVersion, &m.LastSeen,
			&m.OSBuild, &m.UptimeHours, &m.FreeDiskGB, &m.TotalMemoryGB,
			&pendingRebootInt, &defenderEnabledInt, &defenderRealtimeInt,
			&m.PasswordMinLength, &m.PasswordLockoutThreshold, &m.PasswordMaxAgeDays,
			&m.FailedLogonCount24h)
		m.IsDomainJoined = isDomainInt == 1
		m.PendingReboot = pendingRebootInt == 1
		m.DefenderEnabled = defenderEnabledInt == 1
		m.DefenderRealtimeProtection = defenderRealtimeInt == 1
		if ipStr != "" {
			m.IPAddresses = strings.Split(ipStr, ",")
		} else {
			m.IPAddresses = []string{}
		}
		machines = append(machines, m)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(machines)
}

func accountsHandler(w http.ResponseWriter, r *http.Request) {
	db := dbFromCtx(r)
	machineID := r.URL.Query().Get("machine_id")

	const cols = `username, sid, enabled, is_admin, description, full_name, last_logon,
		password_last_set, password_expires_date, account_expires_date,
		password_never_expires, password_required, user_may_change_password,
		days_since_last_logon, is_built_in`

	var (
		rows *sql.Rows
		err  error
	)
	if machineID != "" {
		rows, err = db.Query(`SELECT `+cols+` FROM accounts WHERE machine_id = ? ORDER BY username`, machineID)
	} else {
		rows, err = db.Query(`SELECT ` + cols + ` FROM accounts ORDER BY username`)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	accounts := []AccountPayload{}
	for rows.Next() {
		var a AccountPayload
		var neverExpiresInt, requiredInt, mayChangeInt, builtInInt int
		rows.Scan(&a.Username, &a.SID, &a.Enabled, &a.IsAdmin, &a.Description, &a.FullName, &a.LastLogon,
			&a.PasswordLastSet, &a.PasswordExpiresDate, &a.AccountExpiresDate,
			&neverExpiresInt, &requiredInt, &mayChangeInt,
			&a.DaysSinceLastLogon, &builtInInt)
		a.PasswordNeverExpires = neverExpiresInt == 1
		a.PasswordRequired = requiredInt == 1
		a.UserMayChangePassword = mayChangeInt == 1
		a.IsBuiltIn = builtInInt == 1

		// Derived risk signals — computed at read time, not stored
		a.IsStale = a.Enabled && a.DaysSinceLastLogon >= 90
		a.IsPasswordRisk = a.Enabled && !a.PasswordRequired

		accounts = append(accounts, a)
	}
	rows.Close() // must close before opening the group-membership query below —
	// otherwise the second query on the same connection can silently
	// return zero rows with some SQLite drivers.

	// Attach group membership (reverse lookup) — only meaningful when
	// scoped to a single machine, since group names aren't unique across
	// machines and AccountPayload doesn't carry machine_id in the response.
	if machineID != "" {
		groupRows, gErr := db.Query(`
			SELECT gm.username, g.name
			FROM group_members gm
			JOIN groups g ON gm.group_id = g.id
			WHERE gm.machine_id = ?
		`, machineID)
		if gErr != nil {
			log.Printf("Group membership lookup failed: %v", gErr)
		} else {
			membership := map[string][]string{}
			rowCount := 0
			for groupRows.Next() {
				var username, groupName string
				groupRows.Scan(&username, &groupName)
				membership[username] = append(membership[username], groupName)
				rowCount++
			}
			groupRows.Close()
			log.Printf("Group membership lookup: %d membership rows found for machine %s", rowCount, machineID)

			for i := range accounts {
				if groups, ok := membership[accounts[i].Username]; ok {
					accounts[i].Groups = groups
				} else {
					accounts[i].Groups = []string{}
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(accounts)
}

func createActionHandler(w http.ResponseWriter, r *http.Request) {
	db := dbFromCtx(r)
	createdBy := usernameFromCtx(r)
	if createdBy == "" {
		createdBy = "unknown"
	}

	var req CreateActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if !validActionTypes[req.Type] {
		http.Error(w, "invalid action type", http.StatusBadRequest)
		return
	}

	if req.Username == "" {
		http.Error(w, "username is required", http.StatusBadRequest)
		return
	}

	// Some action types require a password param
	if actionTypesRequiringPassword[req.Type] {
		pw := req.Params["password"]
		if len(pw) < 8 {
			http.Error(w, "password must be at least 8 characters", http.StatusBadRequest)
			return
		}
	}

	if req.Type == "rename_account" && req.Params["new_username"] == "" {
		http.Error(w, "new_username is required", http.StatusBadRequest)
		return
	}
	if (req.Type == "add_to_group" || req.Type == "remove_from_group") && req.Params["group"] == "" {
		http.Error(w, "group is required", http.StatusBadRequest)
		return
	}

	if req.Params == nil {
		req.Params = map[string]string{}
	}
	paramsJSON, err := json.Marshal(req.Params)
	if err != nil {
		http.Error(w, "invalid params", http.StatusBadRequest)
		return
	}

	// Only one pending action per account at a time — regardless of type.
	// Queuing e.g. "delete" and "rename" on the same account concurrently
	// is inherently unsafe: whichever runs first changes the state the
	// second one assumes, causing confusing "not found" failures.
	var existingID int
	var existingType string
	err = db.QueryRow(`
		SELECT id, type FROM actions
		WHERE machine_id = ? AND username = ? AND status = 'pending'
		LIMIT 1
	`, req.MachineID, req.Username).Scan(&existingID, &existingType)
	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"action_id":     existingID,
			"duplicate":     true,
			"existing_type": existingType,
		})
		return
	}

	result, err := db.Exec(`
		INSERT INTO actions (machine_id, type, username, params, created_by, status)
		VALUES (?, ?, ?, ?, ?, 'pending')
	`, req.MachineID, req.Type, req.Username, string(paramsJSON), createdBy)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	id, _ := result.LastInsertId()
	log.Printf("Action queued: %s %s on %s by %s", req.Type, req.Username, req.MachineID, createdBy)
	writeAudit(db, req.MachineID, req.Type, req.Username, createdBy, clientIP(r), "queued")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int64{"action_id": id})
}

// ── Bulk actions ─────────────────────────────────────────────────────────

type BulkActionTarget struct {
	MachineID string `json:"machine_id"`
	Username  string `json:"username"`
}

type BulkActionRequest struct {
	Targets []BulkActionTarget `json:"targets"`
	Type    string             `json:"type"`
	Params  map[string]string  `json:"params"`
}

type BulkActionResult struct {
	Created  int      `json:"created"`
	Skipped  int      `json:"skipped"`
	Failed   int      `json:"failed"`
	Errors   []string `json:"errors,omitempty"`
}

// bulkCreateActionHandler queues the same action type across many
// machine+username targets in one call — e.g. disable 20 stale accounts
// across 20 machines with a single dashboard action.
func bulkCreateActionHandler(w http.ResponseWriter, r *http.Request) {
	db := dbFromCtx(r)
	createdBy := usernameFromCtx(r)
	if createdBy == "" {
		createdBy = "unknown"
	}

	var req BulkActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if !validActionTypes[req.Type] {
		http.Error(w, "invalid action type", http.StatusBadRequest)
		return
	}
	if len(req.Targets) == 0 {
		http.Error(w, "targets is required", http.StatusBadRequest)
		return
	}
	if len(req.Targets) > 500 {
		http.Error(w, "too many targets in a single request (max 500)", http.StatusBadRequest)
		return
	}

	if actionTypesRequiringPassword[req.Type] {
		pw := req.Params["password"]
		if len(pw) < 8 {
			http.Error(w, "password must be at least 8 characters", http.StatusBadRequest)
			return
		}
	}

	if req.Params == nil {
		req.Params = map[string]string{}
	}
	paramsJSON, err := json.Marshal(req.Params)
	if err != nil {
		http.Error(w, "invalid params", http.StatusBadRequest)
		return
	}

	result := BulkActionResult{}

	for _, t := range req.Targets {
		if t.MachineID == "" || t.Username == "" {
			result.Failed++
			result.Errors = append(result.Errors, "missing machine_id or username in a target")
			continue
		}

		var existingID int
		err := db.QueryRow(`
			SELECT id FROM actions
			WHERE machine_id = ? AND username = ? AND status = 'pending'
			LIMIT 1
		`, t.MachineID, t.Username).Scan(&existingID)
		if err == nil {
			result.Skipped++
			continue
		}

		_, err = db.Exec(`
			INSERT INTO actions (machine_id, type, username, params, created_by, status)
			VALUES (?, ?, ?, ?, ?, 'pending')
		`, t.MachineID, req.Type, t.Username, string(paramsJSON), createdBy)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, t.Username+": "+err.Error())
			continue
		}
		writeAudit(db, t.MachineID, req.Type, t.Username, createdBy, clientIP(r), "queued via bulk action")
		result.Created++
	}

	log.Printf("Bulk action queued: %s — %d created, %d skipped, %d failed (by %s)",
		req.Type, result.Created, result.Skipped, result.Failed, createdBy)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// cancelActionHandler cancels a still-pending action before the agent
// picks it up. No effect on actions already completed/failed — those
// have already happened and can't be undone from here.
func cancelActionHandler(w http.ResponseWriter, r *http.Request) {
	db := dbFromCtx(r)
	cancelledBy := usernameFromCtx(r)

	var req struct {
		ActionID int `json:"action_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	result, err := db.Exec(`
		UPDATE actions SET status = 'cancelled', result = ?
		WHERE id = ? AND status = 'pending'
	`, "Cancelled by "+cancelledBy, req.ActionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	n, _ := result.RowsAffected()
	if n == 0 {
		http.Error(w, "action not found or already resolved", http.StatusNotFound)
		return
	}

	log.Printf("Action %d cancelled by %s", req.ActionID, cancelledBy)
	w.WriteHeader(http.StatusOK)
}

func actionsStatusHandler(w http.ResponseWriter, r *http.Request) {
	db := dbFromCtx(r)
	machineID := r.URL.Query().Get("machine_id")
	if machineID == "" {
		http.Error(w, "machine_id required", http.StatusBadRequest)
		return
	}

	rows, err := db.Query(`
		SELECT id, machine_id, '', type, username, created_by, status, created_at,
			COALESCE(executed_at, ''), COALESCE(result, '')
		FROM actions WHERE machine_id = ?
		ORDER BY created_at DESC LIMIT 50
	`, machineID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	actions := []ActionRow{}
	for rows.Next() {
		var a ActionRow
		rows.Scan(&a.ID, &a.MachineID, &a.Hostname, &a.Type, &a.Username,
			&a.CreatedBy, &a.Status, &a.CreatedAt, &a.ExecutedAt, &a.Result)
		actions = append(actions, a)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(actions)
}

func auditLogHandler(w http.ResponseWriter, r *http.Request) {
	db := dbFromCtx(r)

	rows, err := db.Query(`
		SELECT a.id, a.machine_id, COALESCE(m.hostname,''), a.type, a.username,
			a.created_by, a.status, a.created_at,
			COALESCE(a.executed_at,''), COALESCE(a.result,'')
		FROM actions a
		LEFT JOIN machines m ON a.machine_id = m.id
		ORDER BY a.created_at DESC LIMIT 200
	`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	actions := []ActionRow{}
	for rows.Next() {
		var a ActionRow
		rows.Scan(&a.ID, &a.MachineID, &a.Hostname, &a.Type, &a.Username,
			&a.CreatedBy, &a.Status, &a.CreatedAt, &a.ExecutedAt, &a.Result)
		actions = append(actions, a)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(actions)
}

// ── Bootstrap operator tenant (for local dev) ──────────────────────────────
// Creates a default tenant on first run if none exist.
func bootstrapDevTenant() {
	tenants, err := listTenants()
	if err != nil || len(tenants) > 0 {
		return
	}

	slug := os.Getenv("AURIGON_TENANT_SLUG")
	name := os.Getenv("AURIGON_TENANT_NAME")
	pass := os.Getenv("AURIGON_ADMIN_PASSWORD")

	if slug == "" {
		slug = "default"
	}
	if name == "" {
		name = "Default Tenant"
	}
	if pass == "" {
		log.Fatal("AURIGON_ADMIN_PASSWORD must be set to provision the first tenant")
	}

	tenant, err := provisionTenant(name, slug, pass)
	if err != nil {
		log.Fatalf("Failed to provision default tenant: %v", err)
	}

	log.Printf("Default tenant created — slug: %q, login with admin / %s", tenant.Slug, "[your AURIGON_ADMIN_PASSWORD]")
}

// ── Main ───────────────────────────────────────────────────────────────────

func main() {
	// Load secrets from a .env file next to the binary, if present. This
	// keeps secrets out of NSSM's service config (which is easy to
	// accidentally overwrite wholesale — see the AppEnvironmentExtra
	// footgun) and out of process-list-visible environment variables set
	// at the OS level. Real environment variables (if already set) always
	// take priority over the .env file, so this never silently overrides
	// something you set deliberately elsewhere.
	loadDotEnv(".env")

	// Master DB (tenant registry)
	initMasterDB()

	// JWT
	initJWT()

	// Pre-load all tenant DBs
	preloadTenantDBs()

	// Bootstrap a default tenant for local dev if none exist
	bootstrapDevTenant()

	if os.Getenv("AURIGON_ALLOWED_ORIGINS") == "" {
		log.Println("WARNING: AURIGON_ALLOWED_ORIGINS not set — CORS is restricted to local-dev origins only.")
		log.Println("WARNING: set this explicitly (comma-separated) before deploying to a real domain.")
	}

	if os.Getenv("AURIGON_OPERATOR_KEY") == "" {
		log.Println("Tenant management endpoints (/tenants, /tenants/create) are disabled — AURIGON_OPERATOR_KEY not set.")
	} else {
		log.Println("Tenant management endpoints are active, protected by AURIGON_OPERATOR_KEY.")
	}

	// ── Agent endpoints (tenant routed via deploy key) ────────────────────
	http.HandleFunc("/register",     corsMiddleware(agentTenantMiddleware(registerHandler)))
	http.HandleFunc("/machine-info", corsMiddleware(agentTenantMiddleware(machineInfoHandler)))
	http.HandleFunc("/sessions",     corsMiddleware(agentTenantMiddleware(sessionsHandler)))
	http.HandleFunc("/inventory",    corsMiddleware(agentTenantMiddleware(inventoryHandler)))
	http.HandleFunc("/action-result",corsMiddleware(agentTenantMiddleware(actionResultHandler)))
	http.HandleFunc("/actions",      corsMiddleware(agentTenantMiddleware(agentActionsHandler)))

	// ── Auth (tenant resolved from slug in request body) ──────────────────
	http.HandleFunc("/login", corsMiddleware(loginHandler))

	// ── Dashboard API (tenant resolved from JWT) ──────────────────────────
	http.HandleFunc("/change-password", corsMiddleware(tenantMiddleware(changePasswordHandler)))
	http.HandleFunc("/machines",        corsMiddleware(tenantMiddleware(machinesHandler)))
	http.HandleFunc("/accounts",        corsMiddleware(tenantMiddleware(accountsHandler)))
	http.HandleFunc("/actions/create",  corsMiddleware(tenantMiddleware(createActionHandler)))
	http.HandleFunc("/actions/cancel",  corsMiddleware(tenantMiddleware(cancelActionHandler)))
	http.HandleFunc("/actions/bulk-create", corsMiddleware(tenantMiddleware(bulkCreateActionHandler)))
	http.HandleFunc("/sessions/list",   corsMiddleware(tenantMiddleware(sessionsListHandler)))
	http.HandleFunc("/actions/status",  corsMiddleware(tenantMiddleware(actionsStatusHandler)))
	http.HandleFunc("/audit",           corsMiddleware(tenantMiddleware(auditLogHandler)))
	http.HandleFunc("/audit-log",       corsMiddleware(tenantMiddleware(adminOnly(auditLogListHandler))))

	// ── Users (admin only) ────────────────────────────────────────────────
	http.HandleFunc("/users",        corsMiddleware(tenantMiddleware(adminOnly(listUsersHandler))))
	http.HandleFunc("/users/create", corsMiddleware(tenantMiddleware(adminOnly(createUserHandler))))
	http.HandleFunc("/users/delete", corsMiddleware(tenantMiddleware(adminOnly(deleteUserHandler))))

	// ── Groups ────────────────────────────────────────────────────────────
	http.HandleFunc("/groups",           corsMiddleware(tenantMiddleware(groupsHandler)))
	http.HandleFunc("/groups/inventory", corsMiddleware(agentTenantMiddleware(groupInventoryHandler)))

	// ── Agent key ─────────────────────────────────────────────────────────
	http.HandleFunc("/agent-key", corsMiddleware(tenantMiddleware(adminOnly(agentKeyHandler))))

	// ── Tenant management (operator only — protect this in production) ────
	http.HandleFunc("/tenants",        corsMiddleware(operatorAuthMiddleware(listTenantsHandler)))
	http.HandleFunc("/tenants/create", corsMiddleware(operatorAuthMiddleware(createTenantHandler)))

	// ── Serve agent installer downloads ────────────────────────────────────
	// Place AurigonAgentSetup.exe (and future platform installers) in
	// ./downloads/ next to the backend binary.
	downloadsDir := "./downloads"
	if _, err := os.Stat(downloadsDir); err == nil {
		http.Handle("/downloads/", http.StripPrefix("/downloads/", http.FileServer(http.Dir(downloadsDir))))
		log.Println("Serving agent installers from ./downloads")
	} else {
		log.Println("No ./downloads folder — create it and add AurigonAgentSetup.exe to enable in-dashboard downloads")
	}

	// ── Serve compiled dashboard ──────────────────────────────────────────
	distDir := "./dist"
	if _, err := os.Stat(distDir); err == nil {
		fs := http.FileServer(http.Dir(distDir))
		http.Handle("/assets/", fs)
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.NotFound(w, r)
				return
			}
			http.ServeFile(w, r, distDir+"/index.html")
		})
		log.Println("Serving dashboard from ./dist")
	} else {
		log.Println("No ./dist folder — run 'npm run build' in dashboard/")
	}

	port := os.Getenv("AURIGON_PORT")
	if port == "" {
		port = "8080"
	}

	certFile := os.Getenv("AURIGON_TLS_CERT_FILE")
	keyFile := os.Getenv("AURIGON_TLS_KEY_FILE")

	if certFile != "" && keyFile != "" {
		log.Printf("Aurigon Security backend running on https://localhost:%s (TLS enabled)", port)
		log.Fatal(http.ListenAndServeTLS(":"+port, certFile, keyFile, nil))
	} else {
		log.Println("WARNING: AURIGON_TLS_CERT_FILE / AURIGON_TLS_KEY_FILE not set — running on plain HTTP.")
		log.Println("WARNING: credentials and account data will travel unencrypted. Do not use this outside a trusted local network.")
		log.Printf("Aurigon Security backend running on http://localhost:%s", port)
		log.Fatal(http.ListenAndServe(":"+port, nil))
	}
}