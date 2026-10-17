package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
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
	mux.HandleFunc("POST /api/deployments/{id}/cancel", h.middleware.RequireAuth(h.handleCancelDeployment))
	mux.HandleFunc("POST /api/services/{id}/cancel", h.middleware.RequireAuth(h.handleCancelServiceDeployment))

	// Public CI/CD webhook endpoint (authenticated via unguessable webhook token)
	mux.HandleFunc("POST /api/deploy/webhook/{token}", h.handleWebhook)

	// Observability & Quadlet promotion
	mux.HandleFunc("GET /api/services/{id}/journal", h.middleware.RequireAuth(h.handleGetJournal))
	mux.HandleFunc("POST /api/services/{id}/quadlet/validate", h.middleware.RequireAuth(h.handleValidateQuadlet))
	mux.HandleFunc("POST /api/quadlet/validate", h.middleware.RequireAuth(h.handleValidateQuadletGlobal))
	mux.HandleFunc("POST /api/services/{id}/quadlet/demote", h.middleware.RequireAuth(h.handleDemoteQuadlet))
	mux.HandleFunc("POST /api/services/{id}/group-pod", h.middleware.RequireAuth(h.handleGroupIntoPod))
	mux.HandleFunc("POST /api/services/{id}/ungroup-pod", h.middleware.RequireAuth(h.handleUngroupFromPod))
	mux.HandleFunc("POST /api/services/{id}/drop-upload", h.middleware.RequireAuth(h.handleDropUpload))
	mux.HandleFunc("POST /api/services/validate-manifest", h.middleware.RequireAuth(h.handleValidateManifest))
	mux.HandleFunc("POST /api/services/{id}/validate-manifest", h.middleware.RequireAuth(h.handleValidateManifest))
	mux.HandleFunc("GET /api/system/linger", h.middleware.RequireAuth(h.handleGetLinger))
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
	if (s.Type == "quadlet" || s.RuntimeTarget == "quadlet" || strings.TrimSpace(s.QuadletConfig) != "") && h.service.deployer != nil {
		unit, file, active := h.service.deployer.DescribeQuadletSupervision(*s)
		s.QuadletUnit = unit
		s.QuadletFile = file
		s.QuadletActive = active
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
	deleteVolumes := r.URL.Query().Get("delete_volumes") == "true"
	if err := h.service.DeleteService(r.Context(), id, deleteVolumes); err != nil {
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
		Commit  string `json:"commit"`
		Image   string `json:"image"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
		if body.Trigger != "" {
			trigger = body.Trigger
		}
	}
	dep, err := h.service.StartDeploy(r.Context(), id, trigger, body.Commit, body.Image)
	if err != nil {
		if errors.Is(err, ErrDeployInProgress) {
			httputil.WriteError(w, http.StatusConflict, err.Error())
			return
		}
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusAccepted, dep)
}

func (h *Handler) handleCancelDeployment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	dep, err := h.service.CancelDeployment(r.Context(), id)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, dep)
}

func (h *Handler) handleCancelServiceDeployment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	svc, err := h.service.CancelServiceDeployment(r.Context(), id)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, svc)
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
			trimmed := strings.TrimSpace(l)
			if trimmed != "" {
				fmt.Fprintf(w, "data: %s\n\n", trimmed)
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
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				fmt.Fprintf(w, "data: %s\n\n", trimmed)
				flusher.Flush()
			}
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

func (h *Handler) handleGetJournal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	lines := 100
	if lStr := r.URL.Query().Get("lines"); lStr != "" {
		if val, err := strconv.Atoi(lStr); err == nil && val > 0 {
			lines = val
		}
	}
	out, err := h.service.GetServiceJournal(r.Context(), id, lines)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"serviceId": id,
		"journal":   out,
	})
}

func (h *Handler) handleValidateQuadlet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Content string            `json:"content"`
		Files   map[string]string `json:"files"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		httputil.WriteError(w, http.StatusBadRequest, "Invalid request payload: "+err.Error())
		return
	}

	content := req.Content
	if content == "" && len(req.Files) == 0 {
		svc, err := h.service.GetService(r.Context(), id)
		if err == nil && svc != nil {
			content = svc.QuadletConfig
		}
	}

	valid, out, err := h.service.ValidateQuadlet(r.Context(), content, req.Files)
	resp := map[string]interface{}{
		"valid":  valid,
		"output": out,
	}
	if err != nil {
		resp["error"] = err.Error()
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleValidateQuadletGlobal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Content string            `json:"content"`
		Files   map[string]string `json:"files"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Invalid request payload: "+err.Error())
		return
	}
	valid, out, err := h.service.ValidateQuadlet(r.Context(), req.Content, req.Files)
	resp := map[string]interface{}{
		"valid":  valid,
		"output": out,
	}
	if err != nil {
		resp["error"] = err.Error()
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleValidateManifest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type string `json:"type"` // "kubernetes" | "compose"
		Yaml string `json:"yaml"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		httputil.WriteError(w, http.StatusBadRequest, "Invalid request payload: "+err.Error())
		return
	}

	content := req.Yaml
	manifestType := strings.ToLower(strings.TrimSpace(req.Type))

	// If payload body was empty, try fetching service yaml if {id} was given
	id := r.PathValue("id")
	if strings.TrimSpace(content) == "" && id != "" {
		svc, err := h.service.GetService(r.Context(), id)
		if err == nil && svc != nil {
			if manifestType == "compose" || svc.Type == "compose" {
				content = svc.ComposeYaml
				manifestType = "compose"
			} else {
				content = svc.K8sYaml
				manifestType = "kubernetes"
			}
		}
	}

	var res ManifestValidationResult
	if manifestType == "compose" {
		res = ValidateComposeManifest(r.Context(), content)
	} else {
		res = ValidateKubernetesPodManifest(r.Context(), content)
	}

	httputil.WriteJSON(w, http.StatusOK, res)
}

func (h *Handler) handleDemoteQuadlet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	svc, err := h.service.DemoteQuadlet(r.Context(), id)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, svc)
}

func (h *Handler) handleGroupIntoPod(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		PodName   string   `json:"podName"`
		CreatePod bool     `json:"createPod"`
		Ports     []string `json:"ports"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Invalid request payload: "+err.Error())
		return
	}
	svc, err := h.service.GroupIntoPod(r.Context(), id, req.PodName, req.CreatePod, req.Ports)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, svc)
}

func (h *Handler) handleUngroupFromPod(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	svc, err := h.service.UngroupFromPod(r.Context(), id)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, svc)
}

func (h *Handler) handleGetLinger(w http.ResponseWriter, r *http.Request) {
	enabled, user := CheckSystemdLinger()
	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"linger": enabled,
		"user":   user,
	})
}
