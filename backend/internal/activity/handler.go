package activity

import (
	"net/http"
	"strconv"
	"strings"

	"gopod/internal/auth"
	"gopod/pkg/httputil"
)

// Handler serves activity and engine-event read APIs.
type Handler struct {
	store      *Store
	middleware *auth.Middleware
}

// NewHandler creates a new activity HTTP handler.
func NewHandler(store *Store, mw *auth.Middleware) *Handler {
	return &Handler{store: store, middleware: mw}
}

// RegisterRoutes mirrors the audit route registration style.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/activity", h.middleware.RequireAuth(h.handleActivity))
	mux.HandleFunc("GET /api/engine/events", h.middleware.RequireAuth(h.handleEngineEvents))
}

func (h *Handler) authorizeScope(w http.ResponseWriter, r *http.Request, projectID string) (restricted bool, allowedProjects map[string]bool, ok bool) {
	if h.middleware == nil {
		return false, nil, true
	}
	if !h.middleware.HasPermission(r, auth.PermissionViewActivity) {
		httputil.WriteError(w, http.StatusForbidden, "Permission 'canViewActivity' required")
		return true, nil, false
	}
	if strings.TrimSpace(projectID) != "" {
		if !h.middleware.CanAccessProject(r, projectID) {
			httputil.WriteError(w, http.StatusForbidden, "Access to project denied")
			return true, nil, false
		}
	}
	restricted, allowed := h.middleware.AllowedProjectSet(r)
	return restricted, allowed, true
}

func (h *Handler) handleActivity(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	projectID := q.Get("projectId")
	restricted, allowed, ok := h.authorizeScope(w, r, projectID)
	if !ok {
		return
	}

	limit := 100
	if v := q.Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			if parsed > 500 {
				parsed = 500
			}
			limit = parsed
		}
	}
	offset := 0
	if v := q.Get("offset"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 0 {
			offset = parsed
		}
	}
	items, total, err := h.store.ListActivity(r.Context(),
		projectID,
		q.Get("serviceId"),
		q.Get("action"),
		q.Get("status"),
		limit, offset,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "Failed to retrieve activity log")
		return
	}
	if restricted {
		filtered := make([]ActivityEntry, 0, len(items))
		for _, item := range items {
			if item.ProjectID != "" && allowed[item.ProjectID] && h.middleware.CanAccessService(r, item.ProjectID, item.ServiceID) {
				filtered = append(filtered, item)
			}
		}
		items = filtered
		total = len(filtered)
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (h *Handler) handleEngineEvents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	projectID := q.Get("projectId")
	restricted, allowed, ok := h.authorizeScope(w, r, projectID)
	if !ok {
		return
	}

	limit := 100
	if v := q.Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			if parsed > 500 {
				parsed = 500
			}
			limit = parsed
		}
	}
	items, total, err := h.store.ListEngineEvents(r.Context(),
		q.Get("level"),
		q.Get("serviceId"),
		projectID,
		q.Get("since"),
		limit,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "Failed to retrieve engine events")
		return
	}
	if restricted {
		filtered := make([]EngineEvent, 0, len(items))
		for _, item := range items {
			if item.ProjectID != "" && allowed[item.ProjectID] && h.middleware.CanAccessService(r, item.ProjectID, item.ServiceID) {
				filtered = append(filtered, item)
			}
		}
		items = filtered
		total = len(filtered)
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}
