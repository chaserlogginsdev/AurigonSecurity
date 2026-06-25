package main

import (
	"encoding/json"
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

func initJWT() {
	secret := os.Getenv("AURIGON_JWT_SECRET")
	if secret == "" {
		secret = "aurigon-dev-secret-change-in-production"
		log.Println("WARNING: AURIGON_JWT_SECRET is not set.")
		log.Println("WARNING: Using insecure default secret. Set this variable before deploying.")
		log.Println("WARNING: Generate one with: openssl rand -hex 32")
	} else {
		if len(secret) < 32 {
			log.Fatalf("AURIGON_JWT_SECRET must be at least 32 characters. Generate one with: openssl rand -hex 32")
		}
		log.Println("JWT secret loaded from environment.")
	}
	jwtSecret = []byte(secret)
}

// ── Rate limiting ─────────────────────────────────────────────────────────────

type loginAttempt struct {
	count    int
	lockedAt time.Time
}

var (
	loginAttempts = map[string]*loginAttempt{}
	loginMu       sync.Mutex
)

func getClientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return strings.Split(ip, ",")[0]
	}
	return strings.Split(r.RemoteAddr, ":")[0]
}

func isRateLimited(ip string) bool {
	loginMu.Lock()
	defer loginMu.Unlock()
	a, ok := loginAttempts[ip]
	if !ok {
		return false
	}
	if !a.lockedAt.IsZero() {
		if time.Since(a.lockedAt) > 15*time.Minute {
			delete(loginAttempts, ip)
			return false
		}
		return true
	}
	return false
}

func recordFailedLogin(ip string) {
	loginMu.Lock()
	defer loginMu.Unlock()
	a, ok := loginAttempts[ip]
	if !ok {
		a = &loginAttempt{}
		loginAttempts[ip] = a
	}
	a.count++
	log.Printf("Failed login attempt %d/5 from IP: %s\n", a.count, ip)
	if a.count >= 5 {
		a.lockedAt = time.Now()
		log.Printf("Rate limit triggered for IP: %s — locked for 15 minutes\n", ip)
	}
}

func clearLoginAttempts(ip string) {
	loginMu.Lock()
	defer loginMu.Unlock()
	delete(loginAttempts, ip)
}

// ── Login ─────────────────────────────────────────────────────────────────────

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token    string `json:"token"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	ip := getClientIP(r)

	if isRateLimited(ip) {
		http.Error(w, "too many failed attempts — try again in 15 minutes", http.StatusTooManyRequests)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	var id int
	var hashedPassword, role string
	err := db.QueryRow(`SELECT id, password, role FROM users WHERE username = ?`, req.Username).
		Scan(&id, &hashedPassword, &role)
	if err != nil {
		recordFailedLogin(ip)
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(req.Password)); err != nil {
		recordFailedLogin(ip)
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	clearLoginAttempts(ip)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  req.Username,
		"role": role,
		"exp":  time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		http.Error(w, "failed to generate token", http.StatusInternalServerError)
		return
	}

	log.Printf("Login: %s from %s\n", req.Username, ip)
	json.NewEncoder(w).Encode(LoginResponse{Token: tokenString, Username: req.Username, Role: role})
}

// ── Change password ───────────────────────────────────────────────────────────

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func changePasswordHandler(w http.ResponseWriter, r *http.Request) {
	username := getUsernameFromToken(r)
	if username == "" {
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
	err := db.QueryRow(`SELECT password FROM users WHERE username = ?`, username).
		Scan(&hashedPassword)
	if err != nil {
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
	log.Printf("Password changed for user: %s\n", username)
	w.WriteHeader(http.StatusOK)
}

// ── JWT middleware ────────────────────────────────────────────────────────────

func getUsernameFromToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return ""
	}
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return ""
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return ""
	}
	sub, _ := claims["sub"].(string)
	return sub
}

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return jwtSecret, nil
		})
		if err != nil || !token.Valid {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}