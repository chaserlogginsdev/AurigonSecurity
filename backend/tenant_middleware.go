package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
)

// Context keys
type contextKey string

const (
	ctxTenantID contextKey = "tenant_id"
	ctxTenantDB contextKey = "tenant_db"
	ctxUsername contextKey = "username"
	ctxRole     contextKey = "role"
)

// tenantFromRequest extracts the tenant ID from the request.
// For dashboard requests: reads the JWT and extracts tenant_id claim.
// For agent requests: decodes the deploy key and looks up the tenant.
func tenantFromRequest(r *http.Request) (string, *sql.DB, error) {
	// Try deploy key first (agent requests)
	deployKey := r.Header.Get("X-Deploy-Key")
	if deployKey != "" {
		tenantID, _, err := validateDeployKeyForTenant(deployKey)
		if err != nil {
			return "", nil, err
		}
		db, err := getTenantDB(tenantID)
		if err != nil {
			return "", nil, err
		}
		return tenantID, db, nil
	}

	// Try JWT (dashboard requests)
	claims, err := parseJWT(r)
	if err != nil {
		return "", nil, err
	}

	tenantID, ok := claims["tenant_id"].(string)
	if !ok || tenantID == "" {
		return "", nil, errUnauthorized
	}

	db, err := getTenantDB(tenantID)
	if err != nil {
		return "", nil, err
	}

	return tenantID, db, nil
}

// tenantMiddleware injects the tenant DB and user info into the request context.
// All dashboard API handlers use this.
func tenantMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := parseJWT(r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		tenantID, _ := claims["tenant_id"].(string)
		username, _ := claims["username"].(string)
		role, _ := claims["role"].(string)

		if tenantID == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// Check tenant is active
		tenant, err := getTenant(tenantID)
		if err != nil || tenant.Status != "active" {
			http.Error(w, "tenant not found or suspended", http.StatusForbidden)
			return
		}

		db, err := getTenantDB(tenantID)
		if err != nil {
			log.Printf("Failed to get tenant DB for %s: %v", tenantID, err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, ctxTenantID, tenantID)
		ctx = context.WithValue(ctx, ctxTenantDB, db)
		ctx = context.WithValue(ctx, ctxUsername, username)
		ctx = context.WithValue(ctx, ctxRole, role)

		next(w, r.WithContext(ctx))
	}
}

// agentTenantMiddleware validates agent auth (deploy key or legacy key)
// and injects the tenant DB into the request context.
func agentTenantMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deployKey := r.Header.Get("X-Deploy-Key")
		if deployKey != "" {
			tenantID, _, err := validateDeployKeyForTenant(deployKey)
			if err != nil {
				log.Printf("Rejected agent — invalid deploy key from %s: %v", r.RemoteAddr, err)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			db, err := getTenantDB(tenantID)
			if err != nil {
				http.Error(w, "tenant not found", http.StatusUnauthorized)
				return
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, ctxTenantID, tenantID)
			ctx = context.WithValue(ctx, ctxTenantDB, db)
			next(w, r.WithContext(ctx))
			return
		}

		// Legacy: single-tenant mode using AURIGON_AGENT_KEY env var
		// This allows existing deployments to keep working during migration
		http.Error(w, "unauthorized: deploy key required", http.StatusUnauthorized)
	}
}

// Helper getters — call these inside handlers instead of using db directly

func dbFromCtx(r *http.Request) *sql.DB {
	db, _ := r.Context().Value(ctxTenantDB).(*sql.DB)
	return db
}

func tenantIDFromCtx(r *http.Request) string {
	id, _ := r.Context().Value(ctxTenantID).(string)
	return id
}

func usernameFromCtx(r *http.Request) string {
	u, _ := r.Context().Value(ctxUsername).(string)
	return u
}

func roleFromCtx(r *http.Request) string {
	role, _ := r.Context().Value(ctxRole).(string)
	return role
}

func isAdminFromCtx(r *http.Request) bool {
	return roleFromCtx(r) == "admin"
}

// adminOnlyMiddleware blocks non-admin users.
func adminOnlyMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isAdminFromCtx(r) {
			http.Error(w, "admin access required", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// ── Tenant management handlers (called by you, the platform operator) ──────

// POST /tenants/create — provision a new tenant
func createTenantHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name          string `json:"name"`
		Slug          string `json:"slug"`
		AdminPassword string `json:"admin_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.Slug == "" || req.AdminPassword == "" {
		http.Error(w, "name, slug, and admin_password are required", http.StatusBadRequest)
		return
	}
	if len(req.AdminPassword) < 8 {
		http.Error(w, "admin_password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	tenant, err := provisionTenant(req.Name, req.Slug, req.AdminPassword)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("Tenant created: %s (%s)", tenant.Name, tenant.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tenant)
}

// GET /tenants — list all tenants (operator only)
func listTenantsHandler(w http.ResponseWriter, r *http.Request) {
	tenants, err := listTenants()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tenants)
}