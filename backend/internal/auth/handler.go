package auth

import (
	"net/http"
	"strings"

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
}

// NewHandler creates a new auth HTTP handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
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

	user, token, err := h.service.SetupAdmin(r.Context(), name, email, password)
	if err != nil {
		httputil.WriteError(w, http.StatusForbidden, err.Error())
		return
	}

	setSessionCookie(w, token, 30*86400)
	httputil.WriteJSON(w, http.StatusCreated, AuthResponse{User: *user, Token: token})
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
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

	user, token, err := h.service.Login(r.Context(), email, password)
	if err != nil {
		httputil.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}

	setSessionCookie(w, token, 30*86400)
	httputil.WriteJSON(w, http.StatusOK, AuthResponse{User: *user, Token: token})
}

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := ExtractToken(r)
	if token != "" {
		_ = h.service.Logout(r.Context(), token)
	}

	setSessionCookie(w, "", -1)
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

func setSessionCookie(w http.ResponseWriter, token string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     "gopod_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
}
