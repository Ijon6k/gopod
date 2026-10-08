package storage

import (
	"encoding/json"
	"net/http"

	"gopod/internal/auth"
	"gopod/pkg/httputil"
)

// Handler handles storage snapshots and backup schedules.
type Handler struct {
	service    *Service
	middleware *auth.Middleware
}

// NewHandler creates a new storage domain handler.
func NewHandler(service *Service, mw *auth.Middleware) *Handler {
	return &Handler{
		service:    service,
		middleware: mw,
	}
}

// RegisterRoutes registers storage endpoints on the mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/storage/snapshots", h.middleware.RequireAuth(h.handleListSnapshots))
	mux.HandleFunc("POST /api/storage/snapshots", h.middleware.RequireAuth(h.handleCreateSnapshot))
	mux.HandleFunc("DELETE /api/storage/snapshots/{id}", h.middleware.RequireAuth(h.handleDeleteSnapshot))

	mux.HandleFunc("GET /api/storage/schedules", h.middleware.RequireAuth(h.handleListSchedules))
	mux.HandleFunc("POST /api/storage/schedules", h.middleware.RequireAuth(h.handleCreateSchedule))
	mux.HandleFunc("POST /api/storage/schedules/{id}/toggle", h.middleware.RequireAuth(h.handleToggleSchedule))
	mux.HandleFunc("DELETE /api/storage/schedules/{id}", h.middleware.RequireAuth(h.handleDeleteSchedule))

	// Frontend compatibility aliases (/api/volumes/*)
	mux.HandleFunc("GET /api/volumes/snapshots", h.middleware.RequireAuth(h.handleListSnapshots))
	mux.HandleFunc("POST /api/volumes/snapshots", h.middleware.RequireAuth(h.handleCreateSnapshot))
	mux.HandleFunc("POST /api/volumes/snapshot", h.middleware.RequireAuth(h.handleCreateSnapshot))
	mux.HandleFunc("DELETE /api/volumes/snapshots/{id}", h.middleware.RequireAuth(h.handleDeleteSnapshot))
	mux.HandleFunc("GET /api/volumes/schedules", h.middleware.RequireAuth(h.handleListSchedules))
	mux.HandleFunc("POST /api/volumes/schedules", h.middleware.RequireAuth(h.handleCreateSchedule))
	mux.HandleFunc("POST /api/volumes/schedules/{id}/toggle", h.middleware.RequireAuth(h.handleToggleSchedule))
	mux.HandleFunc("DELETE /api/volumes/schedules/{id}", h.middleware.RequireAuth(h.handleDeleteSchedule))
}

// ── Snapshots ──

func (h *Handler) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("projectId")
	snapshots, err := h.service.ListSnapshots(r.Context(), projectID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, snapshots)
}

func (h *Handler) handleCreateSnapshot(w http.ResponseWriter, r *http.Request) {
	var snap VolumeSnapshot
	if err := json.NewDecoder(r.Body).Decode(&snap); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	created, err := h.service.CreateSnapshot(r.Context(), snap)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, created)
}

func (h *Handler) handleDeleteSnapshot(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.service.DeleteSnapshot(r.Context(), id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Snapshot deleted"})
}

// ── Schedules ──

func (h *Handler) handleListSchedules(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("projectId")
	schedules, err := h.service.ListSchedules(r.Context(), projectID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, schedules)
}

func (h *Handler) handleCreateSchedule(w http.ResponseWriter, r *http.Request) {
	var sched VolumeSchedule
	if err := json.NewDecoder(r.Body).Decode(&sched); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	created, err := h.service.CreateSchedule(r.Context(), sched)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, created)
}

func (h *Handler) handleDeleteSchedule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.service.DeleteSchedule(r.Context(), id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Schedule deleted"})
}

func (h *Handler) handleToggleSchedule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sched, err := h.service.ToggleSchedule(r.Context(), id)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, sched)
}

