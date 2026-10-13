package runtime

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
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
	mux.HandleFunc("POST /api/containers/prune", h.middleware.RequireAuth(h.handlePruneContainers))
	mux.HandleFunc("GET /api/containers/{id}/logs", h.middleware.RequireAuth(h.handleContainerLogs))
	mux.HandleFunc("GET /api/containers/{id}/exec", h.handleContainerExec) // auth handled inside WebSocket upgrade

	// Protected Podman resources
	mux.HandleFunc("GET /api/pods", h.middleware.RequireAuth(h.handleListPods))
	mux.HandleFunc("POST /api/pods", h.middleware.RequireAuth(h.handleCreatePod))
	mux.HandleFunc("POST /api/pods/{id}/start", h.middleware.RequireAuth(h.handlePodStart))
	mux.HandleFunc("POST /api/pods/{id}/stop", h.middleware.RequireAuth(h.handlePodStop))
	mux.HandleFunc("POST /api/pods/{id}/restart", h.middleware.RequireAuth(h.handlePodRestart))
	mux.HandleFunc("DELETE /api/pods/{id}", h.middleware.RequireAuth(h.handlePodDelete))
	mux.HandleFunc("GET /api/images", h.middleware.RequireAuth(h.handleListImages))
	mux.HandleFunc("POST /api/images/prune", h.middleware.RequireAuth(h.handlePruneImages))
	mux.HandleFunc("GET /api/volumes", h.middleware.RequireAuth(h.handleListVolumes))
	mux.HandleFunc("POST /api/volumes/prune", h.middleware.RequireAuth(h.handlePruneVolumes))
	mux.HandleFunc("GET /api/networks", h.middleware.RequireAuth(h.handleListNetworks))
	mux.HandleFunc("POST /api/system/prune", h.middleware.RequireAuth(h.handlePruneSystem))
	mux.HandleFunc("GET /api/system/df", h.middleware.RequireAuth(h.handleGetDiskUsage))

	// Protected Quadlet systemd resources
	mux.HandleFunc("GET /api/quadlets", h.middleware.RequireAuth(h.handleListQuadlets))
	mux.HandleFunc("POST /api/quadlets/{name}/start", h.middleware.RequireAuth(h.handleQuadletStart))
	mux.HandleFunc("POST /api/quadlets/{name}/stop", h.middleware.RequireAuth(h.handleQuadletStop))
	mux.HandleFunc("POST /api/quadlets/{name}/restart", h.middleware.RequireAuth(h.handleQuadletRestart))
	mux.HandleFunc("POST /api/quadlets/reload", h.middleware.RequireAuth(h.handleQuadletReload))
	mux.HandleFunc("GET /api/quadlets/{name}/content", h.middleware.RequireAuth(h.handleQuadletContent))
	mux.HandleFunc("DELETE /api/quadlets/{name}", h.middleware.RequireAuth(h.handleQuadletDelete))
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

func (h *Handler) handlePruneContainers(w http.ResponseWriter, r *http.Request) {
	if err := h.service.PruneContainers(r.Context()); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Containers pruned successfully"})
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

type createPodRequest struct {
	Name    string            `json:"name"`
	Ports   []string          `json:"ports"`
	Network string            `json:"network"`
	Labels  map[string]string `json:"labels"`
}

func (h *Handler) handleCreatePod(w http.ResponseWriter, r *http.Request) {
	var req createPodRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		httputil.WriteError(w, http.StatusBadRequest, "Pod name is required")
		return
	}
	id, err := h.service.CreatePod(r.Context(), req.Name, req.Ports, req.Network, req.Labels)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, map[string]string{"id": id, "name": req.Name})
}

func (h *Handler) handlePodStart(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.service.StartPod(r.Context(), id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "started", "id": id})
}

func (h *Handler) handlePodStop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.service.StopPod(r.Context(), id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "stopped", "id": id})
}

func (h *Handler) handlePodRestart(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.service.RestartPod(r.Context(), id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "restarted", "id": id})
}

func (h *Handler) handlePodDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	force := r.URL.Query().Get("force") == "true"
	if err := h.service.DeletePod(r.Context(), id, force); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted", "id": id})
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

// ── Quadlet Handlers ──

func (h *Handler) handleListQuadlets(w http.ResponseWriter, r *http.Request) {
	quadlets, err := h.service.ListQuadlets(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, quadlets)
}

func (h *Handler) handleQuadletStart(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := h.service.StartQuadlet(r.Context(), name); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Quadlet started successfully"})
}

func (h *Handler) handleQuadletStop(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := h.service.StopQuadlet(r.Context(), name); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Quadlet stopped successfully"})
}

func (h *Handler) handleQuadletRestart(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := h.service.RestartQuadlet(r.Context(), name); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Quadlet restarted successfully"})
}

func (h *Handler) handleQuadletReload(w http.ResponseWriter, r *http.Request) {
	if err := h.service.ReloadQuadlets(r.Context()); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Systemd user daemon reloaded"})
}

func (h *Handler) handleQuadletContent(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	content, err := h.service.GetQuadletContent(name)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"name": name, "content": content})
}

func (h *Handler) handleQuadletDelete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		httputil.WriteError(w, http.StatusBadRequest, "quadlet unit name is required")
		return
	}
	if err := h.service.DeleteQuadlet(r.Context(), name); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Quadlet unit removed successfully"})
}

