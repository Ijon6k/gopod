package runtime

import (
	"net/http"
	"strconv"
	"time"

	"gopod/internal/auth"
	"gopod/pkg/httputil"
)

// Handler handles container runtime, host telemetry, and resource management.
type Handler struct {
	service    *Service
	middleware *auth.Middleware
}

// NewHandler creates a new runtime domain handler.
func NewHandler(service *Service, mw *auth.Middleware) *Handler {
	return &Handler{
		service:    service,
		middleware: mw,
	}
}

// RegisterRoutes registers runtime endpoints on the mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Public health check
	mux.HandleFunc("GET /api/health", h.handleHealth)

	// Protected host & container telemetry
	mux.HandleFunc("GET /api/system", h.middleware.RequireAuth(h.handleSystem))
	mux.HandleFunc("GET /api/stats", h.middleware.RequireAuth(h.handleStats))
	mux.HandleFunc("GET /api/containers", h.middleware.RequireAuth(h.handleContainers))
	mux.HandleFunc("GET /api/stats/stream", h.middleware.RequireAuth(h.handleStream))

	// Protected container actions
	mux.HandleFunc("POST /api/containers/{id}/start", h.middleware.RequireAuth(h.handleContainerStart))
	mux.HandleFunc("POST /api/containers/{id}/stop", h.middleware.RequireAuth(h.handleContainerStop))
	mux.HandleFunc("POST /api/containers/{id}/restart", h.middleware.RequireAuth(h.handleContainerRestart))
	mux.HandleFunc("DELETE /api/containers/{id}", h.middleware.RequireAuth(h.handleContainerDelete))
	mux.HandleFunc("GET /api/containers/{id}/logs", h.middleware.RequireAuth(h.handleContainerLogs))
	mux.HandleFunc("GET /api/containers/{id}/exec", h.handleContainerExec) // auth handled inside WebSocket upgrade

	// Protected Podman resources
	mux.HandleFunc("GET /api/pods", h.middleware.RequireAuth(h.handleListPods))
	mux.HandleFunc("GET /api/images", h.middleware.RequireAuth(h.handleListImages))
	mux.HandleFunc("POST /api/images/prune", h.middleware.RequireAuth(h.handlePruneImages))
	mux.HandleFunc("GET /api/volumes", h.middleware.RequireAuth(h.handleListVolumes))
	mux.HandleFunc("POST /api/volumes/prune", h.middleware.RequireAuth(h.handlePruneVolumes))
	mux.HandleFunc("GET /api/networks", h.middleware.RequireAuth(h.handleListNetworks))
	mux.HandleFunc("POST /api/system/prune", h.middleware.RequireAuth(h.handlePruneSystem))
	mux.HandleFunc("GET /api/system/df", h.middleware.RequireAuth(h.handleGetDiskUsage))
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"socket":    h.service.SocketPath(),
	})
}

func (h *Handler) handleSystem(w http.ResponseWriter, r *http.Request) {
	info, err := h.service.GetSystemInfo(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, info)
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.service.GetContainerStats(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, stats)
}

func (h *Handler) handleContainers(w http.ResponseWriter, r *http.Request) {
	containers, err := h.service.GetContainers(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, containers)
}

func (h *Handler) handleContainerStart(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.service.StartContainer(r.Context(), id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Container started"})
}

func (h *Handler) handleContainerStop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.service.StopContainer(r.Context(), id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Container stopped"})
}

func (h *Handler) handleContainerRestart(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.service.RestartContainer(r.Context(), id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Container restarted"})
}

func (h *Handler) handleContainerDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	force := r.URL.Query().Get("force") == "true"
	if err := h.service.DeleteContainer(r.Context(), id, force); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Container removed"})
}

func (h *Handler) handleContainerLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tail := 100
	if tStr := r.URL.Query().Get("tail"); tStr != "" {
		if t, err := strconv.Atoi(tStr); err == nil && t > 0 {
			tail = t
		}
	}
	logs, err := h.service.GetContainerLogs(r.Context(), id, tail)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"logs": logs})
}

func (h *Handler) handleListPods(w http.ResponseWriter, r *http.Request) {
	pods, err := h.service.GetPods(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, pods)
}

func (h *Handler) handleListImages(w http.ResponseWriter, r *http.Request) {
	images, err := h.service.GetImages(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, images)
}

func (h *Handler) handlePruneImages(w http.ResponseWriter, r *http.Request) {
	all := r.URL.Query().Get("all") == "true"
	if err := h.service.PruneImages(r.Context(), all); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Images pruned successfully"})
}

func (h *Handler) handleListVolumes(w http.ResponseWriter, r *http.Request) {
	volumes, err := h.service.GetVolumes(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, volumes)
}

func (h *Handler) handlePruneVolumes(w http.ResponseWriter, r *http.Request) {
	if err := h.service.PruneVolumes(r.Context()); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Volumes pruned successfully"})
}

func (h *Handler) handleListNetworks(w http.ResponseWriter, r *http.Request) {
	networks, err := h.service.GetNetworks(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, networks)
}

func (h *Handler) handlePruneSystem(w http.ResponseWriter, r *http.Request) {
	if err := h.service.PruneSystem(r.Context()); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "System pruned successfully"})
}

func (h *Handler) handleGetDiskUsage(w http.ResponseWriter, r *http.Request) {
	df, err := h.service.GetDiskUsage(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, df)
}

