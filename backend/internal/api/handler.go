package api

import (
	"net/http"
	"time"

	"gopod/internal/caddy"
	"gopod/internal/db"
	"gopod/internal/podman"
	"gopod/internal/runner"
	"gopod/pkg/httputil"
)

// Handler serves HTTP endpoints for GoPod.
type Handler struct {
	client   *podman.Client
	repo     *db.Repository
	caddy    *caddy.Reconciler
	deployer *runner.Deployer
}

// NewHandler creates a new API handler with database and Caddy reconciliation.
func NewHandler(client *podman.Client, repo *db.Repository, caddyRec *caddy.Reconciler, dep *runner.Deployer) *Handler {
	return &Handler{
		client:   client,
		repo:     repo,
		caddy:    caddyRec,
		deployer: dep,
	}
}

// RegisterRoutes registers all modular API routes on mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// System & Monitoring
	mux.HandleFunc("GET /api/health", h.handleHealth)
	mux.HandleFunc("GET /api/system", h.handleSystem)
	mux.HandleFunc("GET /api/stats", h.handleStats)
	mux.HandleFunc("GET /api/containers", h.handleContainers)
	mux.HandleFunc("GET /api/stats/stream", h.handleStream)

	// Projects
	mux.HandleFunc("GET /api/projects", h.handleListProjects)
	mux.HandleFunc("POST /api/projects", h.handleCreateProject)
	mux.HandleFunc("GET /api/projects/{id}", h.handleGetProject)
	mux.HandleFunc("DELETE /api/projects/{id}", h.handleDeleteProject)

	// Services
	mux.HandleFunc("GET /api/services", h.handleListServices)
	mux.HandleFunc("POST /api/services", h.handleCreateService)
	mux.HandleFunc("GET /api/services/{id}", h.handleGetService)
	mux.HandleFunc("DELETE /api/services/{id}", h.handleDeleteService)
	mux.HandleFunc("POST /api/services/{id}/deploy", h.handleDeployService)

	// Domains
	mux.HandleFunc("GET /api/domains", h.handleListDomains)
	mux.HandleFunc("POST /api/domains", h.handleCreateDomain)
	mux.HandleFunc("PUT /api/domains/{id}", h.handleUpdateDomain)
	mux.HandleFunc("DELETE /api/domains/{id}", h.handleDeleteDomain)

	// Container & Podman Runtime
	mux.HandleFunc("POST /api/containers/{id}/start", h.handleContainerStart)
	mux.HandleFunc("POST /api/containers/{id}/stop", h.handleContainerStop)
	mux.HandleFunc("POST /api/containers/{id}/restart", h.handleContainerRestart)
	mux.HandleFunc("DELETE /api/containers/{id}", h.handleContainerDelete)
	mux.HandleFunc("GET /api/containers/{id}/logs", h.handleContainerLogs)
	mux.HandleFunc("GET /api/containers/{id}/exec", h.handleContainerExec)
	mux.HandleFunc("GET /api/pods", h.handleListPods)
	mux.HandleFunc("GET /api/images", h.handleListImages)
	mux.HandleFunc("POST /api/images/prune", h.handlePruneImages)
	mux.HandleFunc("GET /api/volumes", h.handleListVolumes)
	mux.HandleFunc("POST /api/volumes/prune", h.handlePruneVolumes)
	mux.HandleFunc("GET /api/networks", h.handleListNetworks)
	mux.HandleFunc("POST /api/system/prune", h.handlePruneSystem)

	// CI/CD Deploy Webhooks
	mux.HandleFunc("POST /api/deploy/webhook/{token}", h.handleDeployWebhook)

	// Credentials (SSH Keys, Registries, Secrets)
	mux.HandleFunc("GET /api/credentials/ssh-keys", h.handleListSSHKeys)
	mux.HandleFunc("POST /api/credentials/ssh-keys", h.handleCreateSSHKey)
	mux.HandleFunc("DELETE /api/credentials/ssh-keys/{id}", h.handleDeleteSSHKey)
	mux.HandleFunc("GET /api/credentials/registries", h.handleListRegistries)
	mux.HandleFunc("POST /api/credentials/registries", h.handleCreateRegistry)
	mux.HandleFunc("DELETE /api/credentials/registries/{id}", h.handleDeleteRegistry)
	mux.HandleFunc("GET /api/credentials/secrets", h.handleListSecrets)
	mux.HandleFunc("POST /api/credentials/secrets", h.handleCreateSecret)
	mux.HandleFunc("DELETE /api/credentials/secrets/{id}", h.handleDeleteSecret)

	// Volume Snapshots & Schedules
	mux.HandleFunc("GET /api/volumes/snapshots", h.handleListVolumeSnapshots)
	mux.HandleFunc("POST /api/volumes/snapshot", h.handleCreateVolumeSnapshot)
	mux.HandleFunc("DELETE /api/volumes/snapshots/{id}", h.handleDeleteVolumeSnapshot)
	mux.HandleFunc("GET /api/volumes/schedules", h.handleListVolumeSchedules)
	mux.HandleFunc("POST /api/volumes/schedules/{id}/toggle", h.handleToggleVolumeSchedule)

	// Traffic & Requests (Caddy Access Logs)
	mux.HandleFunc("GET /api/traffic/requests", h.handleTrafficRequests)
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"socket":    h.client.SocketPath(),
	})
}

func (h *Handler) handleSystem(w http.ResponseWriter, r *http.Request) {
	info, err := h.client.GetSystemInfo(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, info)
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.client.GetContainerStats(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, stats)
}

func (h *Handler) handleContainers(w http.ResponseWriter, r *http.Request) {
	containers, err := h.client.GetContainers(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, containers)
}
