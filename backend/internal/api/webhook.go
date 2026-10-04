package api

import (
	"context"
	"log"
	"net/http"
	"time"

	"gopod/pkg/httputil"
)

// handleDeployWebhook handles Git push notifications from GitHub, GitLab, Gitea.
func (h *Handler) handleDeployWebhook(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		httputil.WriteError(w, http.StatusBadRequest, "Missing webhook token")
		return
	}

	// Look up service matching webhook token
	s, err := h.repo.GetServiceByWebhookToken(r.Context(), token)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if s == nil {
		// Fallback: check if token directly matches service ID
		s, _ = h.repo.GetService(r.Context(), token)
		if s == nil {
			httputil.WriteError(w, http.StatusNotFound, "No matching service for webhook token")
			return
		}
	}

	log.Printf("🔔 [WEBHOOK] Deploy triggered for service '%s' (project: %s)", s.Name, s.ProjectID)

	// Trigger deployment asynchronously in background
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		containerName, err := h.deployer.DeployService(ctx, *s)
		if err != nil {
			log.Printf("❌ [WEBHOOK] Deploy failed for '%s': %v", s.Name, err)
			return
		}
		log.Printf("✅ [WEBHOOK] Deploy succeeded for '%s': container %s", s.Name, containerName)
	}()

	httputil.WriteJSON(w, http.StatusAccepted, map[string]interface{}{
		"status":      "deploy_triggered",
		"serviceId":   s.ID,
		"serviceName": s.Name,
		"projectId":   s.ProjectID,
		"triggeredAt": time.Now().UTC().Format(time.RFC3339),
	})
}
