package main

import (
	"encoding/json"
	"log"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

type UserRow struct {
	ID        int    `json:"id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
}

type CreateUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type DeleteUserRequest struct {
	Username string `json:"username"`
}

// adminOnly middleware — only allows admin role through
func adminOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := getUsernameFromToken(r)
		if username == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var role string
		err := db.QueryRow(`SELECT role FROM users WHERE username = ?`, username).Scan(&role)
		if err != nil || role != "admin" {
			http.Error(w, "forbidden — admin role required", http.StatusForbidden)
			return
		}

		next(w, r)
	}
}

// listUsersHandler returns all dashboard users (no password hashes)
func listUsersHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`SELECT id, username, role, created_at FROM users ORDER BY created_at ASC`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	users := []UserRow{}
	for rows.Next() {
		var u UserRow
		rows.Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt)
		users = append(users, u)
	}
	json.NewEncoder(w).Encode(users)
}

// createUserHandler creates a new dashboard user
func createUserHandler(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if req.Username == "" {
		http.Error(w, "username is required", http.StatusBadRequest)
		return
	}
	if len(req.Password) < 8 {
		http.Error(w, "password must be at least 8 characters", http.StatusBadRequest)
		return
	}
	if req.Role != "admin" && req.Role != "viewer" {
		http.Error(w, "role must be 'admin' or 'viewer'", http.StatusBadRequest)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "failed to hash password", http.StatusInternalServerError)
		return
	}

	_, err = db.Exec(`INSERT INTO users (username, password, role) VALUES (?, ?, ?)`,
		req.Username, string(hash), req.Role)
	if err != nil {
		http.Error(w, "username already exists", http.StatusConflict)
		return
	}

	log.Printf("User created: %s (%s)\n", req.Username, req.Role)
	w.WriteHeader(http.StatusCreated)
}

// deleteUserHandler deletes a dashboard user
func deleteUserHandler(w http.ResponseWriter, r *http.Request) {
	var req DeleteUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	// Prevent deleting yourself
	requestingUser := getUsernameFromToken(r)
	if req.Username == requestingUser {
		http.Error(w, "cannot delete your own account", http.StatusBadRequest)
		return
	}

	// Prevent deleting the last admin
	var adminCount int
	db.QueryRow(`SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&adminCount)
	var targetRole string
	db.QueryRow(`SELECT role FROM users WHERE username = ?`, req.Username).Scan(&targetRole)
	if targetRole == "admin" && adminCount <= 1 {
		http.Error(w, "cannot delete the last admin account", http.StatusBadRequest)
		return
	}

	result, err := db.Exec(`DELETE FROM users WHERE username = ?`, req.Username)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	log.Printf("User deleted: %s\n", req.Username)
	w.WriteHeader(http.StatusOK)
}