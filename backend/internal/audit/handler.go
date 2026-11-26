package audit

import (
	"encoding/json"
	"net/http"
	"strconv"

	"gopod/internal/auth"
)

// Handler handles HTTP requests for audit logs.
type Handler struct {
	service    *Service
	middleware *auth.Middleware
}

// NewHandler creates a new audit HTTP handler.
func NewHandler(service *Service, mw *auth.Middleware) *Handler {
	return &Handler{service: service, middleware: mw}
}

// RegisterRoutes registers audit routes (admin-only).
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/audit-log", h.middleware.RequireAdmin(h.handleAuditLogs))
}

func (h *Handler) handleAuditLogs(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			if parsed > 500 {
				parsed = 500
			}
			limit = parsed
		}
	}

	logs, err := h.service.List(r.Context(), category, limit)
	if err != nil {
		http.Error(w, "Failed to retrieve audit logs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(logs)
}
