package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
)

// ── Types ─────────────────────────────────────────────────────────────────────

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

type ActionRow struct {
	ID         int               `json:"id"`
	MachineID  string            `json:"machine_id"`
	Hostname   string            `json:"hostname"`
	Type       string            `json:"type"`
	Username   string            `json:"username"`
	Params     map[string]string `json:"params"`
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
	ID       string `json:"id"`
	Hostname string `json:"hostname"`
	LastSeen string `json:"last_seen"`
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func generateToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

func agentAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		expectedKey := os.Getenv("AURIGON_AGENT_KEY")
		if expectedKey == "" {
			next(w, r)
			return
		}
		key := r.Header.Get("X-Agent-Key")
		if key != expectedKey {
			log.Printf("Rejected agent request from %s — invalid key\n", r.RemoteAddr)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// ── Agent handlers ────────────────────────────────────────────────────────────

func registerHandler(w http.ResponseWriter, r *http.Request) {
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
		json.NewEncoder(w).Encode(RegisterResponse{DeviceID: existingID, Token: existingToken})
		return
	}

	id := generateToken()[:12]
	token := generateToken()
	db.Exec(`INSERT INTO machines (id, hostname, token) VALUES (?, ?, ?)`, id, req.Hostname, token)
	json.NewEncoder(w).Encode(RegisterResponse{DeviceID: id, Token: token})
	log.Printf("New machine: %s (%s)\n", req.Hostname, id)
}

func inventoryHandler(w http.ResponseWriter, r *http.Request) {
	var inv InventoryRequest
	if err := json.NewDecoder(r.Body).Decode(&inv); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	db.Exec(`UPDATE machines SET last_seen = CURRENT_TIMESTAMP WHERE id = ?`, inv.DeviceID)

	for _, a := range inv.Accounts {
		db.Exec(`
			INSERT INTO accounts (machine_id, username, sid, enabled, is_admin, description, last_logon, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(machine_id, username) DO UPDATE SET
				sid=excluded.sid, enabled=excluded.enabled, is_admin=excluded.is_admin,
				description=excluded.description, last_logon=excluded.last_logon,
				updated_at=CURRENT_TIMESTAMP
		`, inv.DeviceID, a.Username, a.SID, a.Enabled, a.IsAdmin, a.Description, a.LastLogon)
	}

	log.Printf("Inventory: %s — %d accounts\n", inv.DeviceID, len(inv.Accounts))
	w.WriteHeader(http.StatusOK)
}

func agentActionsHandler(w http.ResponseWriter, r *http.Request) {
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
		rows.Scan(&a.ID, &a.MachineID, &a.Hostname, &a.Type, &a.Username,
			&paramsJSON, &a.CreatedBy, &a.Status, &a.CreatedAt, &a.ExecutedAt, &a.Result)
		json.Unmarshal([]byte(paramsJSON), &a.Params)
		actions = append(actions, a)
	}
	json.NewEncoder(w).Encode(actions)
}

func actionResultHandler(w http.ResponseWriter, r *http.Request) {
	var req ActionResultRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	db.Exec(`
		UPDATE actions SET status = ?, result = ?, executed_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, req.Status, req.Result, req.ActionID)

	log.Printf("Action %d: %s — %s\n", req.ActionID, req.Status, req.Result)
	w.WriteHeader(http.StatusOK)
}

// ── Dashboard handlers ────────────────────────────────────────────────────────

func machinesHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`SELECT id, hostname, last_seen FROM machines ORDER BY last_seen DESC`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	machines := []MachineRow{}
	for rows.Next() {
		var m MachineRow
		rows.Scan(&m.ID, &m.Hostname, &m.LastSeen)
		machines = append(machines, m)
	}
	json.NewEncoder(w).Encode(machines)
}

func accountsHandler(w http.ResponseWriter, r *http.Request) {
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
	json.NewEncoder(w).Encode(accounts)
}

func createActionHandler(w http.ResponseWriter, r *http.Request) {
	requestingUser := getUsernameFromToken(r)
	var role string
	db.QueryRow(`SELECT role FROM users WHERE username = ?`, requestingUser).Scan(&role)
	if role != "admin" {
		http.Error(w, "forbidden — admin role required", http.StatusForbidden)
		return
	}

	var req CreateActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	validTypes := map[string]bool{
		"disable_account":  true,
		"enable_account":   true,
		"delete_account":   true,
		"create_account":   true,
		"create_group":     true,
		"delete_group":     true,
		"add_to_group":     true,
		"remove_from_group": true,
	}
	if !validTypes[req.Type] {
		http.Error(w, "invalid action type", http.StatusBadRequest)
		return
	}

	if req.Type == "create_account" {
		if req.Params == nil || req.Params["password"] == "" {
			http.Error(w, "create_account requires a password param", http.StatusBadRequest)
			return
		}
		if len(req.Params["password"]) < 8 {
			http.Error(w, "password must be at least 8 characters", http.StatusBadRequest)
			return
		}
	}

	if (req.Type == "add_to_group" || req.Type == "remove_from_group") {
		if req.Params == nil || req.Params["group"] == "" {
			http.Error(w, "group name required", http.StatusBadRequest)
			return
		}
	}

	paramsJSON, _ := json.Marshal(req.Params)

	result, err := db.Exec(`
		INSERT INTO actions (machine_id, type, username, params, created_by, status)
		VALUES (?, ?, ?, ?, ?, 'pending')
	`, req.MachineID, req.Type, req.Username, string(paramsJSON), requestingUser)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	id, _ := result.LastInsertId()
	log.Printf("Action queued: %s %s on %s by %s (id: %d)\n",
		req.Type, req.Username, req.MachineID, requestingUser, id)
	json.NewEncoder(w).Encode(map[string]int64{"action_id": id})
}

func actionsStatusHandler(w http.ResponseWriter, r *http.Request) {
	machineID := r.URL.Query().Get("machine_id")
	if machineID == "" {
		http.Error(w, "machine_id required", http.StatusBadRequest)
		return
	}

	rows, err := db.Query(`
		SELECT id, machine_id, '', type, username, COALESCE(params, '{}'),
			created_by, status, created_at,
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
		var paramsJSON string
		rows.Scan(&a.ID, &a.MachineID, &a.Hostname, &a.Type, &a.Username,
			&paramsJSON, &a.CreatedBy, &a.Status, &a.CreatedAt, &a.ExecutedAt, &a.Result)
		json.Unmarshal([]byte(paramsJSON), &a.Params)
		actions = append(actions, a)
	}
	json.NewEncoder(w).Encode(actions)
}

func auditLogHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`
		SELECT a.id, a.machine_id, m.hostname, a.type, a.username, COALESCE(a.params, '{}'),
			a.created_by, a.status, a.created_at,
			COALESCE(a.executed_at, ''), COALESCE(a.result, '')
		FROM actions a
		LEFT JOIN machines m ON a.machine_id = m.id
		ORDER BY a.created_at DESC
		LIMIT 200
	`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	actions := []ActionRow{}
	for rows.Next() {
		var a ActionRow
		var paramsJSON string
		rows.Scan(&a.ID, &a.MachineID, &a.Hostname, &a.Type, &a.Username,
			&paramsJSON, &a.CreatedBy, &a.Status, &a.CreatedAt, &a.ExecutedAt, &a.Result)
		json.Unmarshal([]byte(paramsJSON), &a.Params)
		actions = append(actions, a)
	}
	json.NewEncoder(w).Encode(actions)
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	logFile, err := os.OpenFile("aurigon-backend.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Warning: could not open log file: %v\n", err)
	} else {
		defer logFile.Close()
		log.SetOutput(io.MultiWriter(os.Stdout, logFile))
	}
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	initDB()
	initJWT()

	// Agent endpoints (no JWT)
	http.HandleFunc("/register", corsMiddleware(agentAuthMiddleware(registerHandler)))
	http.HandleFunc("/inventory", corsMiddleware(agentAuthMiddleware(inventoryHandler)))
	http.HandleFunc("/groups/inventory", corsMiddleware(agentAuthMiddleware(groupInventoryHandler)))
	http.HandleFunc("/actions", corsMiddleware(agentAuthMiddleware(agentActionsHandler)))
	http.HandleFunc("/action-result", corsMiddleware(agentAuthMiddleware(actionResultHandler)))

	// Auth
	http.HandleFunc("/login", corsMiddleware(loginHandler))
	http.HandleFunc("/change-password", corsMiddleware(authMiddleware(changePasswordHandler)))

	// Dashboard endpoints (JWT protected)
	http.HandleFunc("/machines", corsMiddleware(authMiddleware(machinesHandler)))
	http.HandleFunc("/accounts", corsMiddleware(authMiddleware(accountsHandler)))
	http.HandleFunc("/groups", corsMiddleware(authMiddleware(groupsHandler)))
	http.HandleFunc("/actions/create", corsMiddleware(authMiddleware(createActionHandler)))
	http.HandleFunc("/actions/status", corsMiddleware(authMiddleware(actionsStatusHandler)))
	http.HandleFunc("/audit", corsMiddleware(authMiddleware(auditLogHandler)))

	// User management (admin only)
	http.HandleFunc("/users", corsMiddleware(authMiddleware(listUsersHandler)))
	http.HandleFunc("/users/create", corsMiddleware(adminOnly(createUserHandler)))
	http.HandleFunc("/users/delete", corsMiddleware(adminOnly(deleteUserHandler)))

	log.Println("Backend running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}