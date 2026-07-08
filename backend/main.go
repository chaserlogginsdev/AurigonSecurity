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
	LastLogon   string `json:"last_logon"`
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
	isDomainInt := 0
	if req.IsDomainJoined {
		isDomainInt = 1
	}

	db.Exec(`
		UPDATE machines SET
			domain = ?, is_domain_joined = ?,
			ip_addresses = ?, os_version = ?,
			last_seen = CURRENT_TIMESTAMP
		WHERE id = ?
	`, req.Domain, isDomainInt, ipStr, req.OSVersion, req.DeviceID)

	w.WriteHeader(http.StatusOK)
}

func inventoryHandler(w http.ResponseWriter, r *http.Request) {
	db := dbFromCtx(r)

	var inv InventoryRequest
	if err := json.NewDecoder(r.Body).Decode(&inv); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	db.Exec(`UPDATE machines SET last_seen = CURRENT_TIMESTAMP WHERE id = ?`, inv.DeviceID)

	// Track which usernames are in this upload so we can reconcile afterward
	seen := make(map[string]bool, len(inv.Accounts))

	for _, a := range inv.Accounts {
		seen[a.Username] = true
		db.Exec(`
			INSERT INTO accounts (machine_id, username, sid, enabled, is_admin, description, last_logon, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(machine_id, username) DO UPDATE SET
				sid=excluded.sid, enabled=excluded.enabled, is_admin=excluded.is_admin,
				description=excluded.description, last_logon=excluded.last_logon,
				updated_at=CURRENT_TIMESTAMP
		`, inv.DeviceID, a.Username, a.SID, a.Enabled, a.IsAdmin, a.Description, a.LastLogon)
	}

	// Reconcile: remove any account rows for this machine that were NOT in
	// this upload — these are accounts that were deleted from Windows
	// (e.g. via delete_account action, or manually) since the last sync.
	// Skip this entirely if the upload is empty — an empty inventory is more
	// likely a transient agent error than every account being deleted.
	if len(inv.Accounts) > 0 {
		existingRows, err := db.Query(`SELECT username FROM accounts WHERE machine_id = ?`, inv.DeviceID)
		if err == nil {
			var stale []string
			for existingRows.Next() {
				var username string
				existingRows.Scan(&username)
				if !seen[username] {
					stale = append(stale, username)
				}
			}
			existingRows.Close()
			for _, username := range stale {
				db.Exec(`DELETE FROM accounts WHERE machine_id = ? AND username = ?`, inv.DeviceID, username)
				log.Printf("Removed stale account record: %s on %s (no longer present on machine)", username, inv.DeviceID)
			}
		}
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
		SELECT id, hostname, domain, is_domain_joined, ip_addresses, os_version, last_seen
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
		var isDomainInt int
		rows.Scan(&m.ID, &m.Hostname, &m.Domain, &isDomainInt, &ipStr, &m.OSVersion, &m.LastSeen)
		m.IsDomainJoined = isDomainInt == 1
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

	var (
		rows *sql.Rows
		err  error
	)
	if machineID != "" {
		rows, err = db.Query(`
			SELECT username, sid, enabled, is_admin, description, last_logon
			FROM accounts WHERE machine_id = ? ORDER BY username
		`, machineID)
	} else {
		rows, err = db.Query(`
			SELECT username, sid, enabled, is_admin, description, last_logon
			FROM accounts ORDER BY username
		`)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	accounts := []AccountPayload{}
	for rows.Next() {
		var a AccountPayload
		rows.Scan(&a.Username, &a.SID, &a.Enabled, &a.IsAdmin, &a.Description, &a.LastLogon)
		accounts = append(accounts, a)
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

	validTypes := map[string]bool{
		"disable_account": true,
		"enable_account":  true,
		"create_account":  true,
		"delete_account":  true,
		"set_admin":       true,
		"remove_admin":    true,
	}
	if !validTypes[req.Type] {
		http.Error(w, "invalid action type", http.StatusBadRequest)
		return
	}

	if req.Username == "" {
		http.Error(w, "username is required", http.StatusBadRequest)
		return
	}

	// create_account requires a password param
	if req.Type == "create_account" {
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

	// Idempotency guard: reject if there's already a pending action of the
	// same type for this username on this machine. Prevents duplicate
	// actions from double-clicks or accidental resubmits from executing
	// twice (e.g. two "delete_account" actions racing each other).
	var existingID int
	err = db.QueryRow(`
		SELECT id FROM actions
		WHERE machine_id = ? AND username = ? AND type = ? AND status = 'pending'
		LIMIT 1
	`, req.MachineID, req.Username, req.Type).Scan(&existingID)
	if err == nil {
		// A matching pending action already exists — return it instead of creating a duplicate
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"action_id": existingID,
			"duplicate": true,
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
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int64{"action_id": id})
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
	// Master DB (tenant registry)
	initMasterDB()

	// JWT
	initJWT()

	// Pre-load all tenant DBs
	preloadTenantDBs()

	// Bootstrap a default tenant for local dev if none exist
	bootstrapDevTenant()

	// ── Agent endpoints (tenant routed via deploy key) ────────────────────
	http.HandleFunc("/register",     corsMiddleware(agentTenantMiddleware(registerHandler)))
	http.HandleFunc("/machine-info", corsMiddleware(agentTenantMiddleware(machineInfoHandler)))
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
	http.HandleFunc("/actions/status",  corsMiddleware(tenantMiddleware(actionsStatusHandler)))
	http.HandleFunc("/audit",           corsMiddleware(tenantMiddleware(auditLogHandler)))

	// ── Users (admin only) ────────────────────────────────────────────────
	http.HandleFunc("/users",        corsMiddleware(tenantMiddleware(adminOnly(listUsersHandler))))
	http.HandleFunc("/users/create", corsMiddleware(tenantMiddleware(adminOnly(createUserHandler))))
	http.HandleFunc("/users/delete", corsMiddleware(tenantMiddleware(adminOnly(deleteUserHandler))))

	// ── Groups ────────────────────────────────────────────────────────────
	http.HandleFunc("/groups",           corsMiddleware(tenantMiddleware(groupsHandler)))
	http.HandleFunc("/groups/inventory", corsMiddleware(agentTenantMiddleware(groupInventoryHandler)))

	// ── Deploy keys ───────────────────────────────────────────────────────
	http.HandleFunc("/deploy-keys",          corsMiddleware(tenantMiddleware(adminOnly(listDeployKeysHandler))))
	http.HandleFunc("/deploy-keys/generate", corsMiddleware(tenantMiddleware(adminOnly(generateDeployKeyHandler))))
	http.HandleFunc("/deploy-keys/revoke",   corsMiddleware(tenantMiddleware(adminOnly(revokeDeployKeyHandler))))

	// ── Tenant management (operator only — protect this in production) ────
	http.HandleFunc("/tenants",        corsMiddleware(listTenantsHandler))
	http.HandleFunc("/tenants/create", corsMiddleware(createTenantHandler))

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

	log.Printf("Aurigon Security backend running on http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}