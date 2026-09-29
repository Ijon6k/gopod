package api

import (
	"encoding/json"
	"net/http"
	"time"

	"gopod/internal/podman"
)

// Handler serves HTTP endpoints for GoPod.
type Handler struct {
	client *podman.Client
}

// NewHandler creates a new API handler.
func NewHandler(client *podman.Client) *Handler {
	return &Handler{client: client}
}

// RegisterRoutes registers all API routes on mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/health", h.handleHealth)
	mux.HandleFunc("GET /api/system", h.handleSystem)
	mux.HandleFunc("GET /api/stats", h.handleStats)
	mux.HandleFunc("GET /api/containers", h.handleContainers)
	mux.HandleFunc("GET /api/stats/stream", h.handleStream)
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"socket":    h.client.SocketPath(),
	})
}

func (h *Handler) handleSystem(w http.ResponseWriter, r *http.Request) {
	info, err := h.client.GetSystemInfo(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.client.GetContainerStats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (h *Handler) handleContainers(w http.ResponseWriter, r *http.Request) {
	containers, err := h.client.GetContainers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, containers)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
