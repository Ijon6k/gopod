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

	// Deployment control
	mux.HandleFunc("POST /api/services/{id}/deploy", h.middleware.RequireAuth(h.handleDeploy))
	mux.HandleFunc("POST /api/services/{id}/start", h.middleware.RequireAuth(h.handleStart))
	mux.HandleFunc("POST /api/services/{id}/stop", h.middleware.RequireAuth(h.handleStop))
	mux.HandleFunc("POST /api/services/{id}/restart", h.middleware.RequireAuth(h.handleRestart))
	mux.HandleFunc("GET /api/services/{id}/deployments", h.middleware.RequireAuth(h.handleListDeployments))
	mux.HandleFunc("GET /api/deployments/{id}", h.middleware.RequireAuth(h.handleGetDeployment))
	mux.HandleFunc("GET /api/deployments/{id}/logs", h.middleware.RequireAuth(h.handleGetDeploymentLogs))
	mux.HandleFunc("GET /api/deployments/{id}/logs/stream", h.middleware.RequireAuth(h.handleStreamDeploymentLogs))

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
	trigger := "manual"
	var body struct {
		Trigger string `json:"trigger"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err == nil && body.Trigger != "" {
		trigger = body.Trigger
	}
	dep, err := h.service.StartDeploy(r.Context(), id, trigger)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusAccepted, dep)
}

func (h *Handler) handleGetDeployment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	dep, err := h.service.GetDeployment(r.Context(), id)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if dep == nil {
		httputil.WriteError(w, http.StatusNotFound, "Deployment not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, dep)
}

func (h *Handler) handleGetDeploymentLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	logs, err := h.service.GetDeploymentLogs(r.Context(), id)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"deploymentId": id,
		"logs":         logs,
	})
}

func (h *Handler) handleStreamDeploymentLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Send current accumulated logs first
	currentLogs, _ := h.service.GetDeploymentLogs(r.Context(), id)
	if currentLogs != "" {
		lines := strings.Split(strings.TrimSpace(currentLogs), "\n")
		for _, l := range lines {
			if l != "" {
				data, _ := json.Marshal(map[string]string{"line": l})
				fmt.Fprintf(w, "data: %s\n\n", data)
			}
		}
		flusher.Flush()
	}

	ch, unsub := h.service.SubscribeLogs(id)
	defer unsub()

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case line, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(map[string]string{"line": strings.TrimSuffix(line, "\n")})
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
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

func (h *Handler) handleStart(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	svc, err := h.service.StartService(r.Context(), id)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, svc)
}

func (h *Handler) handleStop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	svc, err := h.service.StopService(r.Context(), id)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, svc)
}

func (h *Handler) handleRestart(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	svc, err := h.service.RestartService(r.Context(), id)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, svc)
}

