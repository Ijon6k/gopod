package ingress

import (
	"encoding/json"
	"errors"
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

// domainPayload is the create/update request body. The basic-auth password is
// write-only: it is accepted on input here and never serialized back in
// responses (Domain.BasicAuthPass is tagged json:"-").
type domainPayload struct {
	Domain
	BasicAuthPass string `json:"basicAuthPass"`
}

func (p domainPayload) toDomain() Domain {
	d := p.Domain
	d.BasicAuthPass = p.BasicAuthPass
	return d
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
	mux.HandleFunc("POST /api/domains/{id}/revalidate", h.middleware.RequireAuth(h.handleRevalidateDomain))
	mux.HandleFunc("GET /api/traffic/requests", h.middleware.RequireAuth(h.handleTrafficRequests))
	mux.HandleFunc("GET /api/traffic/metrics", h.middleware.RequireAuth(h.handleTrafficMetrics))
	mux.HandleFunc("GET /api/ingress/caddyfile", h.middleware.RequireAuth(h.handleGetCaddyfile))
	mux.HandleFunc("POST /api/ingress/reload", h.middleware.RequireAuth(h.handleReloadCaddy))
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
	var p domainPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	created, err := h.service.CreateDomain(r.Context(), p.toDomain())
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, created)
}

func (h *Handler) handleUpdateDomain(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var p domainPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	d := p.toDomain()
	d.ID = id

	updated, err := h.service.UpdateDomain(r.Context(), d)
	if err != nil {
		if errors.Is(err, ErrValidation) {
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, updated)
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

	excludeDashboard := true
	if ex := r.URL.Query().Get("excludeDashboard"); ex == "false" || ex == "0" {
		excludeDashboard = false
	}

	filter := TrafficFilter{
		Limit:            limit,
		ProjectID:        r.URL.Query().Get("projectId"),
		ServiceID:        r.URL.Query().Get("serviceId"),
		StatusGroup:      r.URL.Query().Get("status"),
		TimeRange:        r.URL.Query().Get("timeRange"),
		Search:           r.URL.Query().Get("search"),
		ExcludeDashboard: excludeDashboard,
	}
	if filter.ProjectID == "" {
		filter.ProjectID = r.URL.Query().Get("project_id")
	}
	if filter.ServiceID == "" {
		filter.ServiceID = r.URL.Query().Get("service_id")
	}

	logs := h.service.GetTrafficRequests(filter)
	httputil.WriteJSON(w, http.StatusOK, logs)
}

func (h *Handler) handleTrafficMetrics(w http.ResponseWriter, r *http.Request) {
	excludeDashboard := true
	if ex := r.URL.Query().Get("excludeDashboard"); ex == "false" || ex == "0" {
		excludeDashboard = false
	}

	filter := TrafficFilter{
		ProjectID:        r.URL.Query().Get("projectId"),
		ServiceID:        r.URL.Query().Get("serviceId"),
		StatusGroup:      r.URL.Query().Get("status"),
		TimeRange:        r.URL.Query().Get("timeRange"),
		Search:           r.URL.Query().Get("search"),
		ExcludeDashboard: excludeDashboard,
	}
	if filter.ProjectID == "" {
		filter.ProjectID = r.URL.Query().Get("project_id")
	}
	if filter.ServiceID == "" {
		filter.ServiceID = r.URL.Query().Get("service_id")
	}

	metrics := h.service.GetTrafficMetrics(filter)
	httputil.WriteJSON(w, http.StatusOK, metrics)
}

func (h *Handler) handleRevalidateDomain(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.WriteError(w, http.StatusBadRequest, "Domain ID is required")
		return
	}
	domain, err := h.service.RevalidateDomain(r.Context(), id)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, domain)
}

func (h *Handler) handleGetCaddyfile(w http.ResponseWriter, r *http.Request) {
	caddyfile := h.service.GetCaddyfile()
	adminURL := "http://127.0.0.1:2019"
	httpPort := 80
	httpsPort := 443
	if h.service.reconciler != nil {
		adminURL = h.service.reconciler.GetAdminURL()
		httpPort, httpsPort = h.service.reconciler.GetIngressPorts()
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"caddyfile": caddyfile,
		"status":    "active",
		"adminUrl":  adminURL,
		"httpPort":  httpPort,
		"httpsPort": httpsPort,
	})
}

func (h *Handler) handleReloadCaddy(w http.ResponseWriter, r *http.Request) {
	if err := h.service.ReloadCaddy(r.Context()); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Caddy ingress reloaded successfully"})
}
