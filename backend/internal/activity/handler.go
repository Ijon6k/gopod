package activity

import (
	"net/http"
	"strconv"

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

func (h *Handler) handleActivity(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := 100
	if v := q.Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
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
		q.Get("projectId"),
		q.Get("serviceId"),
		q.Get("action"),
		q.Get("status"),
		limit, offset,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (h *Handler) handleEngineEvents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := 100
	if v := q.Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	items, total, err := h.store.ListEngineEvents(r.Context(),
		q.Get("level"),
		q.Get("serviceId"),
		q.Get("projectId"),
		q.Get("since"),
		limit,
	)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}
