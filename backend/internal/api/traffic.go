package api

import (
	"net/http"
	"strconv"

	"gopod/pkg/httputil"
)

func (h *Handler) handleTrafficRequests(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 50
	}

	logs := h.caddy.ReadAccessLogs(limit)
	httputil.WriteJSON(w, http.StatusOK, logs)
}
