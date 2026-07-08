package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

// tenantDBs holds one open database connection per tenant.
// Protected by a mutex so concurrent requests don't race.
var (
	tenantDBs   = map[string]*sql.DB{}
	tenantDBsMu sync.RWMutex
)

// getTenantDB returns the database connection for a tenant,
// opening it if not already open.
func getTenantDB(tenantID string) (*sql.DB, error) {
	// Fast path — already open
	tenantDBsMu.RLock()
	if db, ok := tenantDBs[tenantID]; ok {
		tenantDBsMu.RUnlock()
		return db, nil
	}
	tenantDBsMu.RUnlock()

	// Slow path — open and cache
	tenantDBsMu.Lock()
	defer tenantDBsMu.Unlock()

	// Double-check after acquiring write lock
	if db, ok := tenantDBs[tenantID]; ok {
		return db, nil
	}

	dbPath := tenantDBPath(tenantID)
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("tenant database not found: %s", tenantID)
	}

	db, err := openTenantDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open tenant DB for %s: %w", tenantID, err)
	}

	tenantDBs[tenantID] = db
	log.Printf("Opened tenant database: %s", tenantID)
	return db, nil
}

// tenantDBPath returns the filesystem path for a tenant's database.
func tenantDBPath(tenantID string) string {
	return filepath.Join("data", tenantID, "aurigon.db")
}

// openTenantDB opens a SQLite connection, applies pragmas, and runs
// any pending schema migrations so existing tenant databases stay current.
func openTenantDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.Exec(`PRAGMA journal_mode=WAL`)
	db.Exec(`PRAGMA foreign_keys=ON`)
	migrateTenantDB(db)
	return db, nil
}

// migrateTenantDB applies incremental schema changes to an existing
// tenant database. Safe to run repeatedly — errors (e.g. column already
// exists) are ignored.
func migrateTenantDB(db *sql.DB) {
	migrations := []string{
		`ALTER TABLE actions ADD COLUMN params TEXT NOT NULL DEFAULT '{}'`,
	}
	for _, m := range migrations {
		db.Exec(m)
	}
}

// provisionTenant creates a new tenant:
//   1. Generates a unique tenant ID
//   2. Creates the tenant data directory
//   3. Initializes the tenant's database with the full schema
//   4. Creates the initial admin user
//   5. Registers the tenant in the master DB
func provisionTenant(name, slug, adminPassword string) (*Tenant, error) {
	// Validate slug uniqueness
	existing, _ := getTenantBySlug(slug)
	if existing != nil {
		return nil, fmt.Errorf("slug %q is already taken", slug)
	}

	// Generate tenant ID
	id := "tnt_" + randomHex(8)

	// Create tenant data directory
	dir := filepath.Join("data", id)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create tenant directory: %w", err)
	}

	// Initialize tenant database
	dbPath := filepath.Join(dir, "aurigon.db")
	db, err := openTenantDB(dbPath)
	if err != nil {
		os.RemoveAll(dir)
		return nil, fmt.Errorf("failed to create tenant database: %w", err)
	}

	if err := initTenantSchema(db); err != nil {
		db.Close()
		os.RemoveAll(dir)
		return nil, fmt.Errorf("failed to init tenant schema: %w", err)
	}

	// Create the initial admin user in the tenant DB
	hash, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
	if err != nil {
		db.Close()
		os.RemoveAll(dir)
		return nil, fmt.Errorf("failed to hash admin password: %w", err)
	}
	db.Exec(`INSERT INTO users (username, password, role) VALUES ('admin', ?, 'admin')`, string(hash))

	// Cache the open connection
	tenantDBsMu.Lock()
	tenantDBs[id] = db
	tenantDBsMu.Unlock()

	// Register in master DB
	_, err = masterDB.Exec(`
		INSERT INTO tenants (id, name, slug) VALUES (?, ?, ?)
	`, id, name, slug)
	if err != nil {
		db.Close()
		os.RemoveAll(dir)
		return nil, fmt.Errorf("failed to register tenant: %w", err)
	}

	// Track admin in master index
	masterDB.Exec(`
		INSERT INTO tenant_admins (tenant_id, username) VALUES (?, 'admin')
	`, id)

	log.Printf("Tenant provisioned: %s (%s) at %s", name, id, dbPath)

	return &Tenant{ID: id, Name: name, Slug: slug, Status: "active"}, nil
}

// initTenantSchema creates all tables in a tenant's database.
// This is the same schema as before, just applied per-tenant.
func initTenantSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS machines (
		id               TEXT PRIMARY KEY,
		hostname         TEXT NOT NULL,
		token            TEXT NOT NULL,
		domain           TEXT NOT NULL DEFAULT '',
		is_domain_joined INTEGER NOT NULL DEFAULT 0,
		ip_addresses     TEXT NOT NULL DEFAULT '',
		os_version       TEXT NOT NULL DEFAULT '',
		last_seen        DATETIME DEFAULT CURRENT_TIMESTAMP
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
		params      TEXT NOT NULL DEFAULT '{}',
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

	CREATE TABLE IF NOT EXISTS audit_log (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		machine_id  TEXT,
		action_type TEXT NOT NULL,
		username    TEXT NOT NULL,
		performed_by TEXT NOT NULL,
		detail      TEXT,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := db.Exec(schema)
	return err
}

// preloadTenantDBs opens connections for all active tenants at startup
// so the first request to each tenant isn't slow.
func preloadTenantDBs() {
	tenants, err := listTenants()
	if err != nil {
		log.Printf("Warning: could not preload tenant DBs: %v", err)
		return
	}

	for _, t := range tenants {
		if t.Status != "active" {
			continue
		}
		dbPath := tenantDBPath(t.ID)
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			log.Printf("Warning: tenant %s has no database at %s", t.ID, dbPath)
			continue
		}
		db, err := openTenantDB(dbPath)
		if err != nil {
			log.Printf("Warning: could not open tenant DB for %s: %v", t.ID, err)
			continue
		}
		tenantDBsMu.Lock()
		tenantDBs[t.ID] = db
		tenantDBsMu.Unlock()
		log.Printf("Preloaded tenant: %s (%s)", t.Name, t.ID)
	}
}

// closeTenantDBs cleanly closes all open tenant database connections.
// Call on shutdown.
func closeTenantDBs() {
	tenantDBsMu.Lock()
	defer tenantDBsMu.Unlock()
	for id, db := range tenantDBs {
		db.Close()
		delete(tenantDBs, id)
	}
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}