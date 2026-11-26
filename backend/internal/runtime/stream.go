package runtime

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"gopod/internal/auth"
)

func (h *Handler) handleStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Escape the server-wide WriteDeadline: this stream lives as long as its
	// client, while the server defaults to cutting responses after 60s.
	if rc := http.NewResponseController(w); rc != nil {
		if err := rc.SetWriteDeadline(time.Time{}); err != nil {
			log.Printf("[SSE] could not clear write deadline for stats stream: %v", err)
		}
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Initial ping
	fmt.Fprintf(w, "event: ready\ndata: {\"status\":\"connected\"}\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			ctx := r.Context()
			payload := map[string]interface{}{
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			}
			if h.middleware == nil || h.middleware.HasPermission(r, auth.PermissionViewMonitoring) || h.middleware.HasPermission(r, auth.PermissionAccessRuntime) {
				if sys, sysErr := h.service.GetSystemInfo(ctx); sysErr == nil {
					payload["system"] = sys
				}
			}
			if stats, statsErr := h.service.GetContainerStats(ctx); statsErr == nil {
				payload["stats"] = h.filterStatsForCaller(r, stats)
			}

			dataBytes, err := json.Marshal(payload)
			if err != nil {
				continue
			}

			fmt.Fprintf(w, "data: %s\n\n", dataBytes)
			flusher.Flush()
		}
	}
}
