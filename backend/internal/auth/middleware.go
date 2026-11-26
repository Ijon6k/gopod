package auth

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"

	"gopod/pkg/httputil"
)

type contextKey string

const userContextKey contextKey = "gopod_auth_user"

// WithUser returns a copy of ctx carrying the authenticated User.
func WithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// UserFromContext returns the authenticated User placed into ctx by auth middleware.
func UserFromContext(ctx context.Context) (*User, bool) {
	if ctx == nil {
		return nil, false
	}
	u, ok := ctx.Value(userContextKey).(*User)
	return u, ok && u != nil
}

// Middleware provides HTTP authentication and authorization guards.
type Middleware struct {
	authService *Service
	access      *AccessChecker
	initialized atomic.Bool
}

// NewMiddleware creates a new auth middleware handler.
func NewMiddleware(authService *Service) *Middleware {
	return &Middleware{authService: authService}
}

// SetAccessChecker attaches the project-membership & RBAC store.
func (m *Middleware) SetAccessChecker(ac *AccessChecker) {
	m.access = ac
}

// HasPermission reports whether the authenticated caller holds perm.
// Fails closed when no user exists in context or user is unauthenticated.
func (m *Middleware) HasPermission(r *http.Request, perm Permission) bool {
	if r == nil {
		return false
	}
	user, ok := UserFromContext(r.Context())
	if !ok {
		return false
	}
	return user.HasPermission(perm)
}

// RequirePermission enforces authentication plus the specified capability perm
// (administrators implicitly pass).
func (m *Middleware) RequirePermission(perm Permission, next http.HandlerFunc) http.HandlerFunc {
	return m.guard(false, func(w http.ResponseWriter, r *http.Request) {
		if !m.HasPermission(r, perm) {
			httputil.WriteError(w, http.StatusForbidden, "Insufficient permissions for this action")
			return
		}
		next(w, r)
	})
}

// CanAccessProject reports whether the caller in r's context may read/mutate projectID.
// Fails closed if no authenticated user exists in context.
func (m *Middleware) CanAccessProject(r *http.Request, projectID string) bool {
	if r == nil {
		return false
	}
	user, ok := UserFromContext(r.Context())
	if !ok {
		return false
	}
	if user.IsAdmin() {
		return true
	}
	if m == nil || m.access == nil || strings.TrimSpace(projectID) == "" {
		return false
	}
	allowed, err := m.access.CanAccessProject(r.Context(), user.ID, projectID)
	return err == nil && allowed
}

// CanAccessService reports whether the caller in r's context may access serviceID
// within projectID. Fails closed on missing user or DB errors.
func (m *Middleware) CanAccessService(r *http.Request, projectID, serviceID string) bool {
	if r == nil {
		return false
	}
	user, ok := UserFromContext(r.Context())
	if !ok {
		return false
	}
	if user.IsAdmin() {
		return true
	}
	if m == nil || m.access == nil || strings.TrimSpace(projectID) == "" {
		return false
	}
	allowed, err := m.access.CanAccessService(r.Context(), user.ID, projectID, serviceID)
	return err == nil && allowed
}

// AllowedProjectSet reports whether the caller is scoped to specific projects and returns
// the allowed set. When restricted == false, the caller sees all projects.
// Fails closed when no user exists in context.
func (m *Middleware) AllowedProjectSet(r *http.Request) (restricted bool, allowed map[string]bool) {
	if r == nil {
		return true, map[string]bool{}
	}
	user, ok := UserFromContext(r.Context())
	if !ok {
		return true, map[string]bool{}
	}
	if user.IsAdmin() {
		return false, nil
	}
	if m == nil || m.access == nil {
		return true, map[string]bool{}
	}
	ids, err := m.access.ListUserProjects(r.Context(), user.ID)
	if err != nil {
		return true, map[string]bool{}
	}
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return true, set
}

// AllowedServiceSet reports whether the caller is restricted to specific service IDs
// inside projectID.
func (m *Middleware) AllowedServiceSet(r *http.Request, projectID string) (restricted bool, allowed map[string]bool) {
	if r == nil {
		return true, map[string]bool{}
	}
	user, ok := UserFromContext(r.Context())
	if !ok {
		return true, map[string]bool{}
	}
	if user.IsAdmin() {
		return false, nil
	}
	if m == nil || m.access == nil || strings.TrimSpace(projectID) == "" {
		return true, map[string]bool{}
	}
	rFlag, set, err := m.access.AllowedServiceSet(r.Context(), user.ID, projectID)
	if err != nil {
		return true, map[string]bool{}
	}
	return rFlag, set
}

// GrantProjectOwner records project access for the current user when creating a project
// as a non-admin member so newly created workspaces stay immediately accessible.
func (m *Middleware) GrantProjectOwner(r *http.Request, projectID string) {
	if r == nil || m == nil || m.access == nil || strings.TrimSpace(projectID) == "" {
		return
	}
	if user, ok := UserFromContext(r.Context()); ok && user != nil && !user.IsAdmin() {
		_ = m.access.GrantProjectAccess(r.Context(), projectID, user.ID, "owner")
	}
}

// RequireAuth wraps an http.HandlerFunc to ensure the request is from an authenticated session.
// Credentials are resolved from the session cookie or the bearer header only.
func (m *Middleware) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return m.guard(false, next)
}

// RequireAdmin wraps an http.HandlerFunc to ensure the caller is authenticated AND holds
// the "admin" role. Non-admin members receive 403 Forbidden.
func (m *Middleware) RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return m.guard(false, func(w http.ResponseWriter, r *http.Request) {
		user, ok := UserFromContext(r.Context())
		if !ok || !user.IsAdmin() {
			httputil.WriteError(w, http.StatusForbidden, "Administrator privileges required for this action")
			return
		}
		next(w, r)
	})
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
			grant := r.URL.Query().Get("grant")
			if grant == "" {
				if q := r.URL.Query().Get("token"); strings.HasPrefix(q, "grant_") {
					grant = q
				}
			}
			if grant != "" && m.authService != nil {
				if u, ok := m.authService.ValidateStreamGrant(r.Context(), grant); ok {
					user = u
					session = &Session{UserID: user.ID}
				}
			}
		}

		if user == nil {
			session, user, err = m.authService.GetSessionUser(r)
		}

		if err != nil || session == nil || user == nil {
			httputil.WriteError(w, http.StatusUnauthorized, "Authentication required. Please sign in.")
			return
		}

		next(w, r.WithContext(WithUser(r.Context(), user)))
	}
}
