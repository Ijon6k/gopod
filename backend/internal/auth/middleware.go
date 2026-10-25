package auth

import (
	"net/http"
	"sync/atomic"

	"gopod/pkg/httputil"
)

// Middleware provides HTTP authentication guards.
type Middleware struct {
	authService *Service
	initialized atomic.Bool
}

// NewMiddleware creates a new auth middleware handler.
func NewMiddleware(authService *Service) *Middleware {
	return &Middleware{authService: authService}
}

// RequireAuth wraps an http.HandlerFunc to ensure the request is from an authenticated session.
// Credentials are resolved from the session cookie or the bearer header only.
func (m *Middleware) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return m.guard(false, next)
}

// RequireAuthAllowQuery additionally accepts a ?token= query parameter. It is
// reserved for the SSE and WebSocket endpoints whose clients (EventSource,
// browser WebSocket) cannot set an Authorization header; a query token is
// delegable through referer/proxy logs, so no ordinary JSON route may use it.
func (m *Middleware) RequireAuthAllowQuery(next http.HandlerFunc) http.HandlerFunc {
	return m.guard(true, next)
}

func (m *Middleware) guard(allowQueryToken bool, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !m.initialized.Load() {
			count, err := m.authService.CountUsers(r.Context())
			if err != nil {
				httputil.WriteError(w, http.StatusInternalServerError, "Cluster authorization error")
				return
			}
			if count == 0 {
				httputil.WriteError(w, http.StatusForbidden, "GOPOD cluster setup required. Please visit /setup.")
				return
			}
			m.initialized.Store(true)
		}

		var session *Session
		var user *User
		var err error
		if allowQueryToken {
			session, user, err = m.authService.GetSessionUserAllowQuery(r)
		} else {
			session, user, err = m.authService.GetSessionUser(r)
		}
		if err != nil || session == nil || user == nil {
			httputil.WriteError(w, http.StatusUnauthorized, "Authentication required. Please sign in.")
			return
		}

		next(w, r)
	}
}
