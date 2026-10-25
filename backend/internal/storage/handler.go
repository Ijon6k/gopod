package storage

import (
	"fmt"
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
	mux.HandleFunc("POST /api/storage/snapshots/{id}/restore", h.middleware.RequireAuth(h.handleRestoreSnapshot))
	mux.HandleFunc("GET /api/storage/snapshots/{id}/download", h.middleware.RequireAuth(h.handleDownloadSnapshot))
	mux.HandleFunc("DELETE /api/storage/snapshots/{id}", h.middleware.RequireAuth(h.handleDeleteSnapshot))
	mux.HandleFunc("POST /api/storage/database/dump", h.middleware.RequireAuth(h.handleCreateDatabaseDump))

	mux.HandleFunc("GET /api/storage/schedules", h.middleware.RequireAuth(h.handleListSchedules))
	mux.HandleFunc("POST /api/storage/schedules", h.middleware.RequireAuth(h.handleCreateSchedule))
	mux.HandleFunc("POST /api/storage/schedules/{id}/toggle", h.middleware.RequireAuth(h.handleToggleSchedule))
	mux.HandleFunc("DELETE /api/storage/schedules/{id}", h.middleware.RequireAuth(h.handleDeleteSchedule))

	mux.HandleFunc("GET /api/storage/destinations", h.middleware.RequireAuth(h.handleListDestinations))
	mux.HandleFunc("POST /api/storage/destinations", h.middleware.RequireAuth(h.handleCreateDestination))
	mux.HandleFunc("POST /api/storage/destinations/test", h.middleware.RequireAuth(h.handleTestDestinationStruct))
	mux.HandleFunc("POST /api/storage/destinations/{id}/test", h.middleware.RequireAuth(h.handleTestDestination))
	mux.HandleFunc("DELETE /api/storage/destinations/{id}", h.middleware.RequireAuth(h.handleDeleteDestination))

	// Frontend compatibility aliases (/api/volumes/*)
	mux.HandleFunc("GET /api/volumes/snapshots", h.middleware.RequireAuth(h.handleListSnapshots))
	mux.HandleFunc("POST /api/volumes/snapshots", h.middleware.RequireAuth(h.handleCreateSnapshot))
	mux.HandleFunc("POST /api/volumes/snapshot", h.middleware.RequireAuth(h.handleCreateSnapshot))
	mux.HandleFunc("POST /api/volumes/snapshots/{id}/restore", h.middleware.RequireAuth(h.handleRestoreSnapshot))
	mux.HandleFunc("GET /api/volumes/snapshots/{id}/download", h.middleware.RequireAuth(h.handleDownloadSnapshot))
	mux.HandleFunc("DELETE /api/volumes/snapshots/{id}", h.middleware.RequireAuth(h.handleDeleteSnapshot))
	mux.HandleFunc("POST /api/volumes/database/dump", h.middleware.RequireAuth(h.handleCreateDatabaseDump))
	mux.HandleFunc("GET /api/volumes/schedules", h.middleware.RequireAuth(h.handleListSchedules))
	mux.HandleFunc("POST /api/volumes/schedules", h.middleware.RequireAuth(h.handleCreateSchedule))
	mux.HandleFunc("POST /api/volumes/schedules/{id}/toggle", h.middleware.RequireAuth(h.handleToggleSchedule))
	mux.HandleFunc("DELETE /api/volumes/schedules/{id}", h.middleware.RequireAuth(h.handleDeleteSchedule))

	// Destination aliases on the same compatibility prefix.
	mux.HandleFunc("GET /api/volumes/destinations", h.middleware.RequireAuth(h.handleListDestinations))
	mux.HandleFunc("POST /api/volumes/destinations", h.middleware.RequireAuth(h.handleCreateDestination))
	mux.HandleFunc("POST /api/volumes/destinations/test", h.middleware.RequireAuth(h.handleTestDestinationStruct))
	mux.HandleFunc("POST /api/volumes/destinations/{id}/test", h.middleware.RequireAuth(h.handleTestDestination))
	mux.HandleFunc("DELETE /api/volumes/destinations/{id}", h.middleware.RequireAuth(h.handleDeleteDestination))
}

// ── Snapshots ──

func (h *Handler) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("projectId")
	serviceID := r.URL.Query().Get("serviceId")
	snapshots, err := h.service.ListSnapshots(r.Context(), projectID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if serviceID != "" {
		filtered := make([]VolumeSnapshot, 0, len(snapshots))
		for _, s := range snapshots {
			if s.ServiceID == serviceID {
				filtered = append(filtered, s)
			}
		}
		snapshots = filtered
	}
	httputil.WriteJSON(w, http.StatusOK, snapshots)
}

func (h *Handler) handleCreateDatabaseDump(w http.ResponseWriter, r *http.Request) {
	var req DatabaseDumpRequest
	if err := httputil.DecodeJSONBody(w, r, &req, 0); err != nil {
		httputil.WriteDecodeError(w, err)
		return
	}
	snap, err := h.service.CreateDatabaseDump(r.Context(), req)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, snap)
}

func (h *Handler) handleCreateSnapshot(w http.ResponseWriter, r *http.Request) {
	var snap VolumeSnapshot
	if err := httputil.DecodeJSONBody(w, r, &snap, 0); err != nil {
		httputil.WriteDecodeError(w, err)
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

func (h *Handler) handleRestoreSnapshot(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.service.RestoreSnapshot(r.Context(), id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Snapshot restored successfully"})
}

func (h *Handler) handleDownloadSnapshot(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	filePath, filename, err := h.service.GetSnapshotFilePath(r.Context(), id)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	http.ServeFile(w, r, filePath)
}

// ── Schedules ──

func (h *Handler) handleListSchedules(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("projectId")
	serviceID := r.URL.Query().Get("serviceId")
	schedules, err := h.service.ListSchedules(r.Context(), projectID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if serviceID != "" {
		filtered := make([]VolumeSchedule, 0, len(schedules))
		for _, s := range schedules {
			if s.ServiceID == serviceID {
				filtered = append(filtered, s)
			}
		}
		schedules = filtered
	}
	httputil.WriteJSON(w, http.StatusOK, schedules)
}

func (h *Handler) handleCreateSchedule(w http.ResponseWriter, r *http.Request) {
	var sched VolumeSchedule
	if err := httputil.DecodeJSONBody(w, r, &sched, 0); err != nil {
		httputil.WriteDecodeError(w, err)
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

// ── Destinations ──

// handleListDestinations returns every destination with its secret redacted.
func (h *Handler) handleListDestinations(w http.ResponseWriter, r *http.Request) {
	dests, err := h.service.ListDestinations(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, dests)
}

// handleCreateDestination stores a new S3-compatible destination. The secret
// in the response is empty by contract: writes are accepted, reads are not.
func (h *Handler) handleCreateDestination(w http.ResponseWriter, r *http.Request) {
	var d BackupDestination
	if err := httputil.DecodeJSONBody(w, r, &d, 0); err != nil {
		httputil.WriteDecodeError(w, err)
		return
	}
	created, err := h.service.CreateDestination(r.Context(), d)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, created)
}

// handleDeleteDestination removes the destination; schedules referencing it
// degrade to local-only snapshots.
func (h *Handler) handleDeleteDestination(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.service.DeleteDestination(r.Context(), id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Destination deleted"})
}

// handleTestDestinationStruct probes an unsaved destination from a form.
func (h *Handler) handleTestDestinationStruct(w http.ResponseWriter, r *http.Request) {
	var d BackupDestination
	if err := httputil.DecodeJSONBody(w, r, &d, 0); err != nil {
		httputil.WriteDecodeError(w, err)
		return
	}
	if err := h.service.TestDestinationStruct(r.Context(), d); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Destination reachable"})
}

// handleTestDestination probes a saved destination by id. An optional
// secretKey body field re-checks credentials before they are persisted.
func (h *Handler) handleTestDestination(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		SecretKey string `json:"secretKey"`
	}
	if r.ContentLength > 0 {
		if err := httputil.DecodeJSONBody(w, r, &body, 0); err != nil {
			httputil.WriteDecodeError(w, err)
			return
		}
	}
	if err := h.service.TestDestination(r.Context(), id, body.SecretKey); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Destination reachable"})
}
