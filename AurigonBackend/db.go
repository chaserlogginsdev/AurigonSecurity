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
		status      TEXT NOT NULL DEFAULT 'pending',
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
		executed_at DATETIME,
		result      TEXT,
		FOREIGN KEY (machine_id) REFERENCES machines(id)
	);
	`

	if _, err := db.Exec(schema); err != nil {
		log.Fatalf("Failed to create schema: %v", err)
	}

	var count int
	db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	if count == 0 {
		password := os.Getenv("AURIGON_ADMIN_PASSWORD")
		if password == "" {
			password = "admin123"
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			log.Fatalf("Failed to hash password: %v", err)
		}
		db.Exec(`INSERT INTO users (username, password, role) VALUES (?, ?, 'admin')`, "admin", string(hash))
		log.Println("Default admin user created (username: admin, password: admin123)")
	}

	log.Println("Database ready: aurigon.db")
}