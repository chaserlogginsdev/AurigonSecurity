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
	CREATE TABLE IF NOT EXISTS tenants (
		id         TEXT PRIMARY KEY,
		name       TEXT NOT NULL,
		slug       TEXT NOT NULL UNIQUE,
		agent_key  TEXT NOT NULL DEFAULT '', -- permanent per-tenant secret, used in the agent key token
		status     TEXT NOT NULL DEFAULT 'active',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

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

	// Migration for databases created before agent_key existed
	masterDB.Exec(`ALTER TABLE tenants ADD COLUMN agent_key TEXT NOT NULL DEFAULT ''`)

	log.Println("Master database ready: data/tenants.db")
}

// Tenant represents a customer tenant record.
type Tenant struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	AgentKey  string `json:"-"` // never serialized directly — exposed only via the agent-key endpoint as a signed token
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

func getTenant(id string) (*Tenant, error) {
	var t Tenant
	err := masterDB.QueryRow(`
		SELECT id, name, slug, agent_key, status, created_at
		FROM tenants WHERE id = ?
	`, id).Scan(&t.ID, &t.Name, &t.Slug, &t.AgentKey, &t.Status, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func getTenantBySlug(slug string) (*Tenant, error) {
	var t Tenant
	err := masterDB.QueryRow(`
		SELECT id, name, slug, agent_key, status, created_at
		FROM tenants WHERE slug = ? AND status = 'active'
	`, slug).Scan(&t.ID, &t.Name, &t.Slug, &t.AgentKey, &t.Status, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// getTenantByAgentKeySecret looks up a tenant by its raw agent_key secret.
// Used when validating an agent's token.
func getTenantByAgentKeySecret(secret string) (*Tenant, error) {
	var t Tenant
	err := masterDB.QueryRow(`
		SELECT id, name, slug, agent_key, status, created_at
		FROM tenants WHERE agent_key = ? AND status = 'active'
	`, secret).Scan(&t.ID, &t.Name, &t.Slug, &t.AgentKey, &t.Status, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func listTenants() ([]Tenant, error) {
	rows, err := masterDB.Query(`
		SELECT id, name, slug, agent_key, status, created_at
		FROM tenants ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tenants []Tenant
	for rows.Next() {
		var t Tenant
		rows.Scan(&t.ID, &t.Name, &t.Slug, &t.AgentKey, &t.Status, &t.CreatedAt)
		tenants = append(tenants, t)
	}
	return tenants, nil
}

// ensureTenantAgentKey backfills an agent_key for tenants created before
// this feature existed. Safe to call repeatedly.
func ensureTenantAgentKey(tenantID string) (string, error) {
	tenant, err := getTenant(tenantID)
	if err != nil {
		return "", err
	}
	if tenant.AgentKey != "" {
		return tenant.AgentKey, nil
	}
	newKey := randomHex(20)
	_, err = masterDB.Exec(`UPDATE tenants SET agent_key = ? WHERE id = ?`, newKey, tenantID)
	if err != nil {
		return "", err
	}
	log.Printf("Backfilled agent_key for tenant %s", tenantID)
	return newKey, nil
}