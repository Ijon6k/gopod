package ingress

import (
	"encoding/json"
	"net/http"
	"strconv"

	"gopod/internal/auth"
	"gopod/pkg/httputil"
)

// Handler handles reverse proxy routing rules and ingress telemetry.
type Handler struct {
	service    *Service
	middleware *auth.Middleware
}

// NewHandler creates a new ingress domain handler.
func NewHandler(service *Service, mw *auth.Middleware) *Handler {
	return &Handler{
		service:    service,
		middleware: mw,
	}
}

// RegisterRoutes registers ingress routes on the mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/domains", h.middleware.RequireAuth(h.handleListDomains))
	mux.HandleFunc("POST /api/domains", h.middleware.RequireAuth(h.handleCreateDomain))
	mux.HandleFunc("PUT /api/domains/{id}", h.middleware.RequireAuth(h.handleUpdateDomain))
	mux.HandleFunc("DELETE /api/domains/{id}", h.middleware.RequireAuth(h.handleDeleteDomain))
	mux.HandleFunc("GET /api/traffic/requests", h.middleware.RequireAuth(h.handleTrafficRequests))
}

func (h *Handler) handleListDomains(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("projectId")
	domains, err := h.service.ListDomains(r.Context(), projectID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, domains)
}

func (h *Handler) handleCreateDomain(w http.ResponseWriter, r *http.Request) {
	var d Domain
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	created, err := h.service.CreateDomain(r.Context(), d)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, created)
}

func (h *Handler) handleUpdateDomain(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var d Domain
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	d.ID = id

	if err := h.service.UpdateDomain(r.Context(), d); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, d)
}

func (h *Handler) handleDeleteDomain(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.service.DeleteDomain(r.Context(), id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Domain rule deleted"})
}

func (h *Handler) handleTrafficRequests(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	logs := h.service.GetTrafficRequests(limit)
	httputil.WriteJSON(w, http.StatusOK, logs)
}
