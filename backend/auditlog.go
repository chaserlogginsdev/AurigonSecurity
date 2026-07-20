package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
)

// writeAudit records a security-relevant event. Call this at every point
// where an admin or the system does something that changes state or
// touches credentials — logins, password changes, account actions, user
// management. machineID may be empty for events not tied to a specific
// managed machine (e.g. a dashboard login).
func writeAudit(db *sql.DB, machineID, actionType, username, performedBy, ipAddress, detail string) {
	if db == nil {
		return
	}
	_, err := db.Exec(`
		INSERT INTO audit_log (machine_id, action_type, username, performed_by, ip_address, detail)
		VALUES (?, ?, ?, ?, ?, ?)
	`, machineID, actionType, username, performedBy, ipAddress, detail)
	if err != nil {
		// Audit logging must never break the actual request — log and move on.
		log.Printf("Failed to write audit log entry (%s by %s): %v", actionType, performedBy, err)
	}
}

// clientIP extracts the caller's IP, respecting a reverse proxy's
// X-Forwarded-For if present (first entry is the original client).
func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return fwd
	}
	return r.RemoteAddr
}

type AuditLogEntry struct {
	ID          int    `json:"id"`
	MachineID   string `json:"machine_id"`
	ActionType  string `json:"action_type"`
	Username    string `json:"username"`
	PerformedBy string `json:"performed_by"`
	IPAddress   string `json:"ip_address"`
	Detail      string `json:"detail"`
	CreatedAt   string `json:"created_at"`
}

// auditLogListHandler returns recent audit entries for the current tenant.
// Separate from the existing /audit endpoint (which lists account actions
// specifically) — this covers logins, user management, and anything else
// routed through writeAudit.
func auditLogListHandler(w http.ResponseWriter, r *http.Request) {
	db := dbFromCtx(r)

	rows, err := db.Query(`
		SELECT id, COALESCE(machine_id, ''), action_type, username, performed_by,
			ip_address, COALESCE(detail, ''), created_at
		FROM audit_log
		ORDER BY created_at DESC
		LIMIT 500
	`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	entries := []AuditLogEntry{}
	for rows.Next() {
		var e AuditLogEntry
		rows.Scan(&e.ID, &e.MachineID, &e.ActionType, &e.Username, &e.PerformedBy,
			&e.IPAddress, &e.Detail, &e.CreatedAt)
		entries = append(entries, e)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}