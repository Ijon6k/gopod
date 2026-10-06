package runtime

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
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
			sys, sysErr := h.service.GetSystemInfo(ctx)
			stats, statsErr := h.service.GetContainerStats(ctx)

			payload := map[string]interface{}{
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			}
			if sysErr == nil {
				payload["system"] = sys
			}
			if statsErr == nil {
				payload["stats"] = stats
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
