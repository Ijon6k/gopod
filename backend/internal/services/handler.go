package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"gopod/internal/auth"
	"gopod/pkg/httputil"
)

// Handler handles service workload HTTP endpoints and CI/CD webhooks.
type Handler struct {
	service    *WorkloadService
	middleware *auth.Middleware
}

// NewHandler creates a new services domain handler.
func NewHandler(service *WorkloadService, mw *auth.Middleware) *Handler {
	return &Handler{
		service:    service,
		middleware: mw,
	}
}

// RegisterRoutes registers service endpoints on the mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/services", h.middleware.RequireAuth(h.handleList))
	mux.HandleFunc("POST /api/services", h.middleware.RequireAuth(h.handleCreate))
	mux.HandleFunc("GET /api/services/{id}", h.middleware.RequireAuth(h.handleGet))
	mux.HandleFunc("PUT /api/services/{id}", h.middleware.RequireAuth(h.handleUpdate))
	mux.HandleFunc("DELETE /api/services/{id}", h.middleware.RequireAuth(h.handleDelete))
	mux.HandleFunc("POST /api/services/{id}/deploy", h.middleware.RequireAuth(h.handleDeploy))
	mux.HandleFunc("GET /api/services/{id}/deployments", h.middleware.RequireAuth(h.handleListDeployments))

	// Public CI/CD webhook endpoint (authenticated via unguessable webhook token)
	mux.HandleFunc("POST /api/deploy/webhook/{token}", h.handleWebhook)
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("projectId")
	list, err := h.service.ListServices(r.Context(), projectID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, list)
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var s Service
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Invalid request payload: "+err.Error())
		return
	}

	created, err := h.service.CreateService(r.Context(), s)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, created)
}

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s, err := h.service.GetService(r.Context(), id)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if s == nil {
		httputil.WriteError(w, http.StatusNotFound, "Service not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, s)
}

func (h *Handler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var s Service
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	s.ID = id

	if err := h.service.UpdateService(r.Context(), s); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, s)
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.service.DeleteService(r.Context(), id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Service deleted"})
}

func (h *Handler) handleDeploy(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	containerName, err := h.service.Deploy(r.Context(), id)
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"status":    "error",
			"error":     err.Error(),
			"container": containerName,
		})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "running",
		"container": containerName,
		"message":   "Service deployed successfully",
	})
}

// WebhookPushPayload matches standard GitHub, GitLab, and Gitea webhook push events.
type WebhookPushPayload struct {
	Ref        string `json:"ref"`
	After      string `json:"after"`
	HeadCommit *struct {
		ID      string `json:"id"`
		Message string `json:"message"`
	} `json:"head_commit"`
}

func (h *Handler) handleWebhook(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		httputil.WriteError(w, http.StatusBadRequest, "Missing webhook token")
		return
	}

	var payload WebhookPushPayload
	_ = json.NewDecoder(r.Body).Decode(&payload)

	// Validate webhook token first
	svc, err := h.service.GetServiceByWebhookToken(r.Context(), token)
	if err != nil || svc == nil {
		httputil.WriteError(w, http.StatusUnauthorized, "Invalid webhook token")
		return
	}

	// Filter by branch if Git push ref is present in payload
	if payload.Ref != "" {
		branch := strings.TrimPrefix(payload.Ref, "refs/heads/")
		targetBranch := svc.GitBranch
		if targetBranch == "" {
			targetBranch = "main"
		}
		if branch != targetBranch {
			httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{
				"status":  "ignored",
				"message": fmt.Sprintf("Ignored push for branch '%s'; service tracks '%s'", branch, targetBranch),
			})
			return
		}
	}

	_, err = h.service.DeployByWebhook(r.Context(), token)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httputil.WriteJSON(w, http.StatusAccepted, map[string]interface{}{
		"message": "Deployment triggered successfully",
		"service": svc.Name,
	})
}

func (h *Handler) handleListDeployments(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	deployments, err := h.service.ListDeployments(r.Context(), id)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, deployments)
}
