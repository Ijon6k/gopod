package services

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gopod/internal/auth"
	"gopod/pkg/httputil"
)

// Handler handles service workload HTTP endpoints and CI/CD webhooks.
type Handler struct {
	service    *WorkloadService
	middleware *auth.Middleware
	deduper    *webhookDeduper
}

// NewHandler creates a new services domain handler.
func NewHandler(service *WorkloadService, mw *auth.Middleware) *Handler {
	return &Handler{
		service:    service,
		middleware: mw,
		deduper:    newWebhookDeduper(),
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
	// Streaming clients (EventSource) cannot set an Authorization header and
	// the URL query token is scoped to exactly these routes.
	mux.HandleFunc("GET /api/deployments/{id}/logs/stream", h.middleware.RequireAuthAllowQuery(h.handleStreamDeploymentLogs))
	mux.HandleFunc("POST /api/deployments/{id}/cancel", h.middleware.RequireAuth(h.handleCancelDeployment))
	mux.HandleFunc("POST /api/services/{id}/cancel", h.middleware.RequireAuth(h.handleCancelServiceDeployment))

	// Public CI/CD webhook endpoint (authenticated via unguessable webhook token)
	mux.HandleFunc("POST /api/deploy/webhook/{token}", h.handleWebhook)

	// Observability & Quadlet promotion
	mux.HandleFunc("GET /api/services/{id}/journal", h.middleware.RequireAuth(h.handleGetJournal))
	mux.HandleFunc("POST /api/services/{id}/quadlet/validate", h.middleware.RequireAuth(h.handleValidateQuadlet))
	mux.HandleFunc("POST /api/services/{id}/quadlet/preview", h.middleware.RequireAuth(h.handleQuadletPreview))
	mux.HandleFunc("POST /api/quadlet/validate", h.middleware.RequireAuth(h.handleValidateQuadletGlobal))
	mux.HandleFunc("POST /api/services/{id}/quadlet/demote", h.middleware.RequireAuth(h.handleDemoteQuadlet))
	mux.HandleFunc("POST /api/services/{id}/group-pod", h.middleware.RequireAuth(h.handleGroupIntoPod))
	mux.HandleFunc("POST /api/services/{id}/ungroup-pod", h.middleware.RequireAuth(h.handleUngroupFromPod))
	mux.HandleFunc("POST /api/services/{id}/drop-upload", h.middleware.RequireAuth(h.handleDropUpload))
	mux.HandleFunc("POST /api/services/validate-manifest", h.middleware.RequireAuth(h.handleValidateManifest))
	mux.HandleFunc("POST /api/services/{id}/validate-manifest", h.middleware.RequireAuth(h.handleValidateManifest))
	mux.HandleFunc("GET /api/system/linger", h.middleware.RequireAuth(h.handleGetLinger))

	// On-demand secret access. List/get responses never carry secret env
	// values or the webhook token; these endpoints hand them out explicitly.
	mux.HandleFunc("POST /api/services/{id}/env/reveal", h.middleware.RequireAuth(h.handleRevealEnv))
	mux.HandleFunc("GET /api/services/{id}/webhook", h.middleware.RequireAuth(h.handleRevealWebhook))
	mux.HandleFunc("POST /api/services/{id}/webhook/rotate", h.middleware.RequireAuth(h.handleRotateWebhook))
	mux.HandleFunc("POST /api/services/{id}/webhook/secret", h.middleware.RequireAuth(h.handleRotateWebhookSecret))

	// Cleanup engine: inventory + manual run (nightly job runs unattended).
	mux.HandleFunc("GET /api/system/gc/preview", h.middleware.RequireAuth(h.handleGCPreview))
	mux.HandleFunc("POST /api/system/gc", h.middleware.RequireAuth(h.handleRunGC))
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("projectId")
	list, err := h.service.ListServices(r.Context(), projectID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, RedactServices(list))
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var s Service
	if err := httputil.DecodeJSONBody(w, r, &s, 0); err != nil {
		httputil.WriteDecodeError(w, err)
		return
	}

	created, err := h.service.CreateService(r.Context(), s)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, RedactService(*created))
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
	httputil.WriteJSON(w, http.StatusOK, RedactService(*s))
}

func (h *Handler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var s Service
	if err := httputil.DecodeJSONBody(w, r, &s, 0); err != nil {
		httputil.WriteDecodeError(w, err)
		return
	}
	s.ID = id

	if err := h.service.UpdateService(r.Context(), s); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if stored, err := h.service.GetService(r.Context(), id); err == nil && stored != nil {
		s.HasWebhook = stored.WebhookToken != ""
		s.HasWebhookSecret = stored.WebhookSecret != "" || stored.WebhookSecretError
		s.WebhookSecretError = stored.WebhookSecretError
	}
	httputil.WriteJSON(w, http.StatusOK, RedactService(s))
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
	if err := httputil.DecodeJSONBody(w, r, &body, 0); err == nil {
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
	httputil.WriteJSON(w, http.StatusOK, RedactService(*svc))
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
	if !ValidDeploymentLogID(id) {
		httputil.WriteError(w, http.StatusBadRequest, "Invalid deployment id")
		return
	}
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
	if !ValidDeploymentLogID(id) {
		httputil.WriteError(w, http.StatusBadRequest, "Invalid deployment id")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Escape the server-wide WriteDeadline: SSE keeps an open response by
	// design and the server's 60s WriteTimeout would otherwise sever every
	// stream a minute after it opened.
	if rc := http.NewResponseController(w); rc != nil {
		if err := rc.SetWriteDeadline(time.Time{}); err != nil {
			log.Printf("[SSE] could not clear write deadline for deployment log stream: %v", err)
		}
	}

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

	// Keep-alive comment ping so intermediaries do not drop a quiet stream;
	// a failed write means the client is gone.
	ping := time.NewTicker(30 * time.Second)
	defer ping.Stop()

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case <-ping.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
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
	httputil.WriteJSON(w, http.StatusOK, RedactService(*svc))
}

func (h *Handler) handleStop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	svc, err := h.service.StopService(r.Context(), id)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, RedactService(*svc))
}

func (h *Handler) handleRestart(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	svc, err := h.service.RestartService(r.Context(), id)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, RedactService(*svc))
}

func (h *Handler) handleGetJournal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	lines := 100
	if lStr := r.URL.Query().Get("lines"); lStr != "" {
		if val, err := strconv.Atoi(lStr); err == nil && val > 0 {
			lines = val
		}
	}
	// Bound the work a single request can ask journalctl for.
	const maxJournalLines = 5000
	if lines > maxJournalLines {
		lines = maxJournalLines
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
	if err := httputil.DecodeJSONBody(w, r, &req, 0); err != nil && !errors.Is(err, io.EOF) {
		httputil.WriteDecodeError(w, err)
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

// handleQuadletPreview returns generated unit files plus the real `quadlet
// -dryrun` verdict — data only, nothing is written to the systemd search path.
func (h *Handler) handleQuadletPreview(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	files, valid, dryrun, verr := h.service.PreviewQuadlet(r.Context(), id)
	if files == nil && verr != nil {
		httputil.WriteError(w, http.StatusBadRequest, verr.Error())
		return
	}
	resp := map[string]interface{}{
		"files":  files,
		"valid":  valid,
		"dryrun": dryrun,
	}
	if verr != nil {
		resp["error"] = verr.Error()
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleValidateQuadletGlobal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Content string            `json:"content"`
		Files   map[string]string `json:"files"`
	}
	if err := httputil.DecodeJSONBody(w, r, &req, 0); err != nil {
		httputil.WriteDecodeError(w, err)
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
	if err := httputil.DecodeJSONBody(w, r, &req, 0); err != nil && !errors.Is(err, io.EOF) {
		httputil.WriteDecodeError(w, err)
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
	httputil.WriteJSON(w, http.StatusOK, RedactService(*svc))
}

func (h *Handler) handleGroupIntoPod(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		PodName   string   `json:"podName"`
		CreatePod bool     `json:"createPod"`
		Ports     []string `json:"ports"`
	}
	if err := httputil.DecodeJSONBody(w, r, &req, 0); err != nil {
		httputil.WriteDecodeError(w, err)
		return
	}
	svc, err := h.service.GroupIntoPod(r.Context(), id, req.PodName, req.CreatePod, req.Ports)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, RedactService(*svc))
}

func (h *Handler) handleUngroupFromPod(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	svc, err := h.service.UngroupFromPod(r.Context(), id)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, RedactService(*svc))
}

// handleRevealEnv returns decrypted values for the requested env keys. This is
// the only read path that hands out secret values; list/get never do.
func (h *Handler) handleRevealEnv(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Keys []string `json:"keys"`
	}
	if err := httputil.DecodeJSONBody(w, r, &req, 0); err != nil {
		httputil.WriteDecodeError(w, err)
		return
	}
	if len(req.Keys) == 0 {
		httputil.WriteError(w, http.StatusBadRequest, "At least one key is required")
		return
	}
	if len(req.Keys) > 100 {
		httputil.WriteError(w, http.StatusBadRequest, "At most 100 keys can be revealed per request")
		return
	}
	values, err := h.service.RevealEnvValues(r.Context(), id, req.Keys)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		httputil.WriteError(w, status, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{"values": values})
}

// handleRevealWebhook returns the webhook token on demand. The token is never
// part of list/get responses.
func (h *Handler) handleRevealWebhook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	token, err := h.service.WebhookTokenFor(r.Context(), id)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		httputil.WriteError(w, status, err.Error())
		return
	}
	provider := ""
	hasSecret := false
	secretError := false
	if svc, err := h.service.GetService(r.Context(), id); err == nil && svc != nil {
		provider = svc.WebhookProvider
		hasSecret = svc.WebhookSecret != "" || svc.WebhookSecretError
		secretError = svc.WebhookSecretError
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"webhookToken":       token,
		"hasWebhook":         token != "",
		"webhookProvider":    provider,
		"hasWebhookSecret":   hasSecret,
		"webhookSecretError": secretError,
	})
}

// handleRotateWebhook issues a fresh webhook token; older tokens stop working.
func (h *Handler) handleRotateWebhook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	token, err := h.service.RotateWebhookToken(r.Context(), id)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		httputil.WriteError(w, status, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"webhookToken": token,
		"hasWebhook":   true,
	})
}

// handleRotateWebhookSecret issues a fresh signature secret and returns it once.
func (h *Handler) handleRotateWebhookSecret(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	secret, err := h.service.RotateWebhookSecret(r.Context(), id)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		httputil.WriteError(w, status, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"webhookSecret":    secret,
		"hasWebhookSecret": true,
	})
}

func (h *Handler) handleGetLinger(w http.ResponseWriter, r *http.Request) {
	enabled, user := CheckSystemdLinger()
	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"linger": enabled,
		"user":   user,
	})
}

// handleGCPreview returns a read-only inventory of reclaimable artifacts.
func (h *Handler) handleGCPreview(w http.ResponseWriter, r *http.Request) {
	preview, err := h.service.GCPreview(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, preview)
}

type gcRunRequest struct {
	Scope          string   `json:"scope"`
	IncludeVolumes bool     `json:"includeVolumes"`
	Volumes        []string `json:"volumes"`
}

// handleRunGC executes one cleanup pass. A run that finds a deployment in
// flight returns 200 with skipped=true rather than an error: skipping is the
// correct, safe outcome.
func (h *Handler) handleRunGC(w http.ResponseWriter, r *http.Request) {
	var req gcRunRequest
	if r.Body != nil {
		if err := httputil.DecodeJSONBody(w, r, &req, 0); err != nil && !errors.Is(err, io.EOF) {
			httputil.WriteDecodeError(w, err)
			return
		}
	}
	scope, err := ParseGCScope(req.Scope)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.service.RunGC(r.Context(), scope, req.IncludeVolumes, req.Volumes)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, result)
}
