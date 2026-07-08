package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var jwtSecret []byte
var errUnauthorized = errors.New("unauthorized")

func initJWT() {
	secret := os.Getenv("AURIGON_JWT_SECRET")
	if secret == "" {
		secret = "aurigon-dev-secret-change-in-production-32chars"
		log.Println("WARNING: Using default JWT secret. Set AURIGON_JWT_SECRET in production.")
	}
	if len(secret) < 32 {
		log.Fatal("AURIGON_JWT_SECRET must be at least 32 characters.")
	}
	jwtSecret = []byte(secret)
}

// ── Rate limiting ──────────────────────────────────────────────────────────

type rateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
}

var loginLimiter = &rateLimiter{attempts: map[string][]time.Time{}}

func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	window := now.Add(-15 * time.Minute)
	attempts := rl.attempts[key]
	var recent []time.Time
	for _, t := range attempts {
		if t.After(window) {
			recent = append(recent, t)
		}
	}
	rl.attempts[key] = recent
	if len(recent) >= 5 {
		return false
	}
	rl.attempts[key] = append(rl.attempts[key], now)
	return true
}

// ── Login ──────────────────────────────────────────────────────────────────

type LoginRequest struct {
	TenantSlug string `json:"tenant_slug"` // which tenant to log into
	Username   string `json:"username"`
	Password   string `json:"password"`
}

type LoginResponse struct {
	Token      string `json:"token"`
	Username   string `json:"username"`
	Role       string `json:"role"`
	TenantID   string `json:"tenant_id"`
	TenantName string `json:"tenant_name"`
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	// Rate limit by IP
	ip := r.RemoteAddr
	if !loginLimiter.allow(ip) {
		http.Error(w, "too many login attempts — try again in 15 minutes", http.StatusTooManyRequests)
		return
	}

	if req.TenantSlug == "" {
		http.Error(w, "tenant_slug is required", http.StatusBadRequest)
		return
	}

	// Look up tenant
	tenant, err := getTenantBySlug(req.TenantSlug)
	if err != nil {
		// Don't reveal whether tenant exists
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	// Get tenant's database
	db, err := getTenantDB(tenant.ID)
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	// Look up user in tenant's database
	var hashedPassword, role string
	err = db.QueryRow(`SELECT password, role FROM users WHERE username = ?`, req.Username).
		Scan(&hashedPassword, &role)
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(req.Password)); err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	// Issue JWT with tenant_id embedded
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":       req.Username,
		"username":  req.Username,
		"role":      role,
		"tenant_id": tenant.ID,
		"exp":       time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		http.Error(w, "failed to generate token", http.StatusInternalServerError)
		return
	}

	log.Printf("Login: %s @ %s (%s)", req.Username, tenant.Slug, tenant.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LoginResponse{
		Token:      tokenString,
		Username:   req.Username,
		Role:       role,
		TenantID:   tenant.ID,
		TenantName: tenant.Name,
	})
}

// ── Change password ────────────────────────────────────────────────────────

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func changePasswordHandler(w http.ResponseWriter, r *http.Request) {
	username := usernameFromCtx(r)
	db := dbFromCtx(r)
	if username == "" || db == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if len(req.NewPassword) < 8 {
		http.Error(w, "new password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	var hashedPassword string
	if err := db.QueryRow(`SELECT password FROM users WHERE username = ?`, username).
		Scan(&hashedPassword); err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(req.CurrentPassword)); err != nil {
		http.Error(w, "current password is incorrect", http.StatusUnauthorized)
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "failed to hash password", http.StatusInternalServerError)
		return
	}

	db.Exec(`UPDATE users SET password = ? WHERE username = ?`, string(newHash), username)
	log.Printf("Password changed: %s @ %s", username, tenantIDFromCtx(r))
	w.WriteHeader(http.StatusOK)
}

// ── JWT helpers ────────────────────────────────────────────────────────────

func parseJWT(r *http.Request) (jwt.MapClaims, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, errUnauthorized
	}
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return nil, errUnauthorized
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errUnauthorized
	}
	return claims, nil
}

// getUsernameFromToken is kept for backward compat — prefer usernameFromCtx(r)
func getUsernameFromToken(r *http.Request) string {
	claims, err := parseJWT(r)
	if err != nil {
		return ""
	}
	sub, _ := claims["sub"].(string)
	return sub
}

// authMiddleware validates JWT and injects tenant context.
// Use tenantMiddleware instead — this is kept for routes that don't need tenant DB.
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return tenantMiddleware(next)
}

// adminOnly blocks non-admin users — use after tenantMiddleware.
func adminOnly(next http.HandlerFunc) http.HandlerFunc {
	return adminOnlyMiddleware(next)
}

// corsMiddleware handles CORS preflight and headers.
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Agent-Key, X-Deploy-Key")
		}
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}