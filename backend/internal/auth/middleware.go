package auth

import (
	"net/http"

	"gopod/pkg/httputil"
)

// Middleware provides HTTP authentication guards.
type Middleware struct {
	authService *Service
}

// NewMiddleware creates a new auth middleware handler.
func NewMiddleware(authService *Service) *Middleware {
	return &Middleware{authService: authService}
}

// RequireAuth wraps an http.HandlerFunc to ensure the request is from an authenticated session.
func (m *Middleware) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		count, err := m.authService.CountUsers(r.Context())
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "Cluster authorization error")
			return
		}
		if count == 0 {
			httputil.WriteError(w, http.StatusForbidden, "GOPOD cluster setup required. Please visit /setup.")
			return
		}

		session, user, err := m.authService.GetSessionUser(r)
		if err != nil || session == nil || user == nil {
			httputil.WriteError(w, http.StatusUnauthorized, "Authentication required. Please sign in.")
			return
		}

		next(w, r)
	}
}
