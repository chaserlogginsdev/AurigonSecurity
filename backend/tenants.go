package main

import (
	"database/sql"
	"log"
	"os"

	_ "modernc.org/sqlite"
)

var masterDB *sql.DB

// initMasterDB opens (or creates) the master tenant registry database.
// This DB only stores tenant metadata — no customer data ever lives here.
func initMasterDB() {
	// Ensure data directory exists
	if err := os.MkdirAll("data", 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	var err error
	masterDB, err = sql.Open("sqlite", "./data/tenants.db")
	if err != nil {
		log.Fatalf("Failed to open master database: %v", err)
	}

	masterDB.Exec(`PRAGMA journal_mode=WAL`)
	masterDB.Exec(`PRAGMA foreign_keys=ON`)

	schema := `
	-- One row per customer tenant
	CREATE TABLE IF NOT EXISTS tenants (
		id         TEXT PRIMARY KEY,        -- e.g. "tnt_a1b2c3d4"
		name       TEXT NOT NULL,           -- e.g. "Acme Corp"
		slug       TEXT NOT NULL UNIQUE,    -- e.g. "acme" (used at login)
		status     TEXT NOT NULL DEFAULT 'active', -- active | suspended | deleted
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Dashboard users scoped to a tenant
	-- Each tenant has their own user table in their own DB,
	-- but we keep a lightweight index here for lookups
	CREATE TABLE IF NOT EXISTS tenant_admins (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		tenant_id  TEXT NOT NULL,
		username   TEXT NOT NULL,
		FOREIGN KEY (tenant_id) REFERENCES tenants(id),
		UNIQUE(tenant_id, username)
	);
	`

	if _, err := masterDB.Exec(schema); err != nil {
		log.Fatalf("Failed to create master schema: %v", err)
	}

	log.Println("Master database ready: data/tenants.db")
}

// Tenant represents a customer tenant record.
type Tenant struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// getTenant looks up a tenant by ID.
func getTenant(id string) (*Tenant, error) {
	var t Tenant
	err := masterDB.QueryRow(`
		SELECT id, name, slug, status, created_at
		FROM tenants WHERE id = ?
	`, id).Scan(&t.ID, &t.Name, &t.Slug, &t.Status, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// getTenantBySlug looks up a tenant by their login slug.
func getTenantBySlug(slug string) (*Tenant, error) {
	var t Tenant
	err := masterDB.QueryRow(`
		SELECT id, name, slug, status, created_at
		FROM tenants WHERE slug = ? AND status = 'active'
	`, slug).Scan(&t.ID, &t.Name, &t.Slug, &t.Status, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// listTenants returns all tenants (for super-admin use).
func listTenants() ([]Tenant, error) {
	rows, err := masterDB.Query(`
		SELECT id, name, slug, status, created_at
		FROM tenants ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tenants []Tenant
	for rows.Next() {
		var t Tenant
		rows.Scan(&t.ID, &t.Name, &t.Slug, &t.Status, &t.CreatedAt)
		tenants = append(tenants, t)
	}
	return tenants, nil
}