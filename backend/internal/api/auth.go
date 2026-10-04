package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"gopod/internal/db"
	"gopod/pkg/httputil"
)

type AuthStatusResponse struct {
	Initialized   bool     `json:"initialized"`
	Authenticated bool     `json:"authenticated"`
	User          *db.User `json:"user,omitempty"`
}

type AuthSetupRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	User  *db.User `json:"user"`
	Token string   `json:"token"`
}

func generateRandomToken(bytesLen int) (string, error) {
	b := make([]byte, bytesLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// extractToken retrieves session token from Cookie or Bearer header
func extractToken(r *http.Request) string {
	if cookie, err := r.Cookie("gopod_session"); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	}
	return ""
}

func (h *Handler) getSessionUser(r *http.Request) (*db.Session, *db.User, error) {
	if h.repo == nil {
		return nil, nil, fmt.Errorf("database not initialized")
	}
	token := extractToken(r)
	if token == "" {
		return nil, nil, nil
	}
	return h.repo.GetSession(r.Context(), token)
}

// handleAuthStatus checks if GOPOD has been set up with an initial admin, and if the requester is authenticated
func (h *Handler) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		httputil.WriteJSON(w, http.StatusOK, AuthStatusResponse{
			Initialized:   false,
			Authenticated: false,
		})
		return
	}

	count, err := h.repo.CountUsers(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "Failed to inspect system status: "+err.Error())
		return
	}

	initialized := count > 0
	session, user, _ := h.getSessionUser(r)
	authenticated := session != nil && user != nil

	httputil.WriteJSON(w, http.StatusOK, AuthStatusResponse{
		Initialized:   initialized,
		Authenticated: authenticated,
		User:          user,
	})
}

// handleAuthSetup handles first-time admin registration. Strictly prohibited if count > 0.
func (h *Handler) handleAuthSetup(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		httputil.WriteError(w, http.StatusInternalServerError, "Database unavailable")
		return
	}

	count, err := h.repo.CountUsers(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if count > 0 {
		httputil.WriteError(w, http.StatusForbidden, "GOPOD is already initialized. Please sign in.")
		return
	}

	var req AuthSetupRequest
	if err := httputil.ParseJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	name := strings.TrimSpace(req.Name)
	email := strings.ToLower(strings.TrimSpace(req.Email))
	password := req.Password

	if name == "" {
		httputil.WriteError(w, http.StatusBadRequest, "Administrator name is required")
		return
	}
	if email == "" || !strings.Contains(email, "@") {
		httputil.WriteError(w, http.StatusBadRequest, "Valid email address is required")
		return
	}
	if len(password) < 8 {
		httputil.WriteError(w, http.StatusBadRequest, "Password must be at least 8 characters long")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	randHex, _ := generateRandomToken(8)
	user := db.User{
		ID:           "usr_" + randHex,
		Email:        email,
		PasswordHash: string(hash),
		Name:         name,
		Role:         "admin",
		CreatedAt:    time.Now(),
	}

	if err := h.repo.CreateUser(r.Context(), user); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "Failed to create user: "+err.Error())
		return
	}

	token, err := generateRandomToken(32)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "Failed to generate session token")
		return
	}

	session := db.Session{
		Token:     token,
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
		CreatedAt: time.Now(),
	}

	if err := h.repo.CreateSession(r.Context(), session); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "Failed to create session: "+err.Error())
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "gopod_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   30 * 86400,
	})

	httputil.WriteJSON(w, http.StatusCreated, AuthResponse{
		User:  &user,
		Token: token,
	})
}

// handleAuthLogin authenticates an existing user
func (h *Handler) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		httputil.WriteError(w, http.StatusInternalServerError, "Database unavailable")
		return
	}

	var req AuthLoginRequest
	if err := httputil.ParseJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	password := req.Password

	if email == "" || password == "" {
		httputil.WriteError(w, http.StatusBadRequest, "Email and password are required")
		return
	}

	user, err := h.repo.GetUserByEmail(r.Context(), email)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "Database error: "+err.Error())
		return
	}
	if user == nil {
		httputil.WriteError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		httputil.WriteError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	token, err := generateRandomToken(32)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "Failed to generate session token")
		return
	}

	session := db.Session{
		Token:     token,
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
		CreatedAt: time.Now(),
	}

	if err := h.repo.CreateSession(r.Context(), session); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "Failed to create session: "+err.Error())
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "gopod_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   30 * 86400,
	})

	httputil.WriteJSON(w, http.StatusOK, AuthResponse{
		User:  user,
		Token: token,
	})
}

// handleAuthLogout invalidates current session
func (h *Handler) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	token := extractToken(r)
	if token != "" && h.repo != nil {
		_ = h.repo.DeleteSession(r.Context(), token)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "gopod_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	httputil.WriteJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// handleAuthMe returns the currently authenticated user
func (h *Handler) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	session, user, err := h.getSessionUser(r)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if session == nil || user == nil {
		httputil.WriteError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"user": user,
	})
}
