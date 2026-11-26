package notifications

import (
	"net/http"

	"gopod/internal/auth"
	"gopod/pkg/httputil"
)

// Handler exposes notification channel management APIs (admin-only).
type Handler struct {
	service    *Service
	middleware *auth.Middleware
}

// NewHandler creates a new notifications HTTP handler.
func NewHandler(service *Service, mw *auth.Middleware) *Handler {
	return &Handler{service: service, middleware: mw}
}

// RegisterRoutes mounts notification endpoints guarded by administrator RBAC.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/notifications", h.middleware.RequireAdmin(h.handleList))
	mux.HandleFunc("POST /api/notifications", h.middleware.RequireAdmin(h.handleCreate))
	mux.HandleFunc("PUT /api/notifications/{id}", h.middleware.RequireAdmin(h.handleUpdate))
	mux.HandleFunc("DELETE /api/notifications/{id}", h.middleware.RequireAdmin(h.handleDelete))
	mux.HandleFunc("POST /api/notifications/{id}/test", h.middleware.RequireAdmin(h.handleTest))
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	list, err := h.service.ListChannels(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, list)
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req UpsertChannelRequest
	if err := httputil.DecodeJSONBody(w, r, &req, 0); err != nil {
		httputil.WriteDecodeError(w, err)
		return
	}
	created, err := h.service.CreateChannel(r.Context(), req)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, created)
}

func (h *Handler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req UpsertChannelRequest
	if err := httputil.DecodeJSONBody(w, r, &req, 0); err != nil {
		httputil.WriteDecodeError(w, err)
		return
	}
	updated, err := h.service.UpdateChannel(r.Context(), id, req)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, updated)
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.service.DeleteChannel(r.Context(), id); err != nil {
		httputil.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Notification channel deleted"})
}

func (h *Handler) handleTest(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.service.TestChannel(r.Context(), id); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Test notification delivered"})
}

