package main

import (
	"database/sql"
	"log"
	"os"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

var db *sql.DB

func initDB() {
	var err error
	db, err = sql.Open("sqlite", "./aurigon.db")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	// Enable WAL mode for better concurrent read performance
	db.Exec(`PRAGMA journal_mode=WAL`)
	db.Exec(`PRAGMA foreign_keys=ON`)

	schema := `
	CREATE TABLE IF NOT EXISTS machines (
		id          TEXT PRIMARY KEY,
		hostname    TEXT NOT NULL,
		token       TEXT NOT NULL,
		last_seen   DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS accounts (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		machine_id  TEXT NOT NULL,
		username    TEXT NOT NULL,
		sid         TEXT,
		enabled     BOOLEAN,
		is_admin    BOOLEAN,
		description TEXT,
		last_logon  TEXT,
		updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (machine_id) REFERENCES machines(id),
		UNIQUE(machine_id, username)
	);

	CREATE TABLE IF NOT EXISTS groups (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		machine_id  TEXT NOT NULL,
		name        TEXT NOT NULL,
		description TEXT,
		FOREIGN KEY (machine_id) REFERENCES machines(id),
		UNIQUE(machine_id, name)
	);

	CREATE TABLE IF NOT EXISTS group_members (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		machine_id TEXT NOT NULL,
		group_id   INTEGER NOT NULL,
		username   TEXT NOT NULL,
		FOREIGN KEY (machine_id) REFERENCES machines(id),
		FOREIGN KEY (group_id) REFERENCES groups(id),
		UNIQUE(group_id, username)
	);

	CREATE TABLE IF NOT EXISTS users (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		username   TEXT NOT NULL UNIQUE,
		password   TEXT NOT NULL,
		role       TEXT NOT NULL DEFAULT 'admin',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS actions (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		machine_id  TEXT NOT NULL,
		type        TEXT NOT NULL,
		username    TEXT NOT NULL,
		created_by  TEXT NOT NULL DEFAULT 'unknown',
		status      TEXT NOT NULL DEFAULT 'pending',
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
		executed_at DATETIME,
		result      TEXT,
		FOREIGN KEY (machine_id) REFERENCES machines(id)
	);

	CREATE TABLE IF NOT EXISTS deploy_keys (
		id          TEXT PRIMARY KEY,
		label       TEXT NOT NULL,
		token       TEXT NOT NULL,
		agent_key   TEXT NOT NULL,
		backend_url TEXT NOT NULL,
		created_by  TEXT NOT NULL,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_used   DATETIME,
		revoked     INTEGER NOT NULL DEFAULT 0
	);
	`

	if _, err := db.Exec(schema); err != nil {
		log.Fatalf("Failed to create schema: %v", err)
	}

	// Migrations for existing databases
	db.Exec(`ALTER TABLE actions ADD COLUMN created_by TEXT NOT NULL DEFAULT 'unknown'`)

	// Seed default admin user only if no users exist
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	if count == 0 {
		password := os.Getenv("AURIGON_ADMIN_PASSWORD")
		if password == "" {
			log.Fatal("AURIGON_ADMIN_PASSWORD is not set. Cannot create admin user without a password. Set this environment variable and restart.")
		}
		if len(password) < 8 {
			log.Fatal("AURIGON_ADMIN_PASSWORD must be at least 8 characters.")
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			log.Fatalf("Failed to hash admin password: %v", err)
		}
		db.Exec(`INSERT INTO users (username, password, role) VALUES (?, ?, 'admin')`, "admin", string(hash))
		log.Println("Admin user created successfully.")
	}

	log.Println("Database ready: aurigon.db")
}