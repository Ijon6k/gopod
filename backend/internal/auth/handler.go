package auth

import (
	"net/http"
	"net/mail"
	"strings"
	"time"

	"gopod/pkg/httputil"
)

// StatusResponse describes the cluster initialization and auth state.
type StatusResponse struct {
	Initialized   bool  `json:"initialized"`
	Authenticated bool  `json:"authenticated"`
	User          *User `json:"user,omitempty"`
}

// Handler handles authentication HTTP endpoints.
type Handler struct {
	service *Service
	limiter *LoginRateLimiter
}

// NewHandler creates a new auth HTTP handler.
func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
		limiter: NewLoginRateLimiter(5, time.Minute),
	}
}

// RegisterRoutes registers auth endpoints on the mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/auth/status", h.handleStatus)
	mux.HandleFunc("POST /api/auth/setup", h.handleSetup)
	mux.HandleFunc("POST /api/auth/login", h.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", h.handleLogout)
	mux.HandleFunc("GET /api/auth/me", h.handleMe)
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	count, err := h.service.CountUsers(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "Failed to inspect cluster: "+err.Error())
		return
	}

	initialized := count > 0
	session, user, _ := h.service.GetSessionUser(r)
	authenticated := session != nil && user != nil

	httputil.WriteJSON(w, http.StatusOK, StatusResponse{
		Initialized:   initialized,
		Authenticated: authenticated,
		User:          user,
	})
}

func (h *Handler) handleSetup(w http.ResponseWriter, r *http.Request) {
	var req SetupRequest
	if err := httputil.ParseJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	name := strings.TrimSpace(req.Name)
	email := strings.ToLower(strings.TrimSpace(req.Email))
	password := req.Password

	if name == "" || len(name) < 2 {
		httputil.WriteError(w, http.StatusBadRequest, "Administrator name must be at least 2 characters long")
		return
	}
	if len(name) > 64 {
		httputil.WriteError(w, http.StatusBadRequest, "Administrator name cannot exceed 64 characters")
		return
	}

	if email == "" {
		httputil.WriteError(w, http.StatusBadRequest, "Administrator email address is required")
		return
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		httputil.WriteError(w, http.StatusBadRequest, "Please provide a valid administrator email address")
		return
	}

	if len(password) < 8 {
		httputil.WriteError(w, http.StatusBadRequest, "Password must be at least 8 characters long")
		return
	}
	if len(password) > 128 {
		httputil.WriteError(w, http.StatusBadRequest, "Password cannot exceed 128 characters")
		return
	}

	user, token, err := h.service.SetupAdmin(r.Context(), name, email, password)
	if err != nil {
		httputil.WriteError(w, http.StatusForbidden, err.Error())
		return
	}

	setSessionCookie(w, r, token, 30*86400)
	httputil.WriteJSON(w, http.StatusCreated, AuthResponse{User: *user, Token: token})
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	if h.limiter != nil && !h.limiter.IsAllowed(r) {
		httputil.WriteError(w, http.StatusTooManyRequests, "Too many failed login attempts. Please wait a minute before retrying.")
		return
	}

	var req LoginRequest
	if err := httputil.ParseJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	password := req.Password

	if email == "" {
		httputil.WriteError(w, http.StatusBadRequest, "Email address is required")
		return
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		httputil.WriteError(w, http.StatusBadRequest, "Please enter a valid email address")
		return
	}

	if password == "" {
		httputil.WriteError(w, http.StatusBadRequest, "Password is required")
		return
	}

	user, token, err := h.service.Login(r.Context(), email, password)
	if err != nil {
		if h.limiter != nil {
			h.limiter.RecordFailure(r)
		}
		httputil.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}

	if h.limiter != nil {
		h.limiter.Reset(r)
	}

	setSessionCookie(w, r, token, 30*86400)
	httputil.WriteJSON(w, http.StatusOK, AuthResponse{User: *user, Token: token})
}

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := ExtractToken(r)
	if token != "" {
		_ = h.service.Logout(r.Context(), token)
	}

	setSessionCookie(w, r, "", -1)
	httputil.WriteJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (h *Handler) handleMe(w http.ResponseWriter, r *http.Request) {
	session, user, err := h.service.GetSessionUser(r)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if session == nil || user == nil {
		httputil.WriteError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{"user": user})
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, token string, maxAge int) {
	isSecure := r != nil && (r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https")
	http.SetCookie(w, &http.Cookie{
		Name:     "gopod_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   isSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
}
