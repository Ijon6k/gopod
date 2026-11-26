package ingress

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

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

func (p *domainPayload) UnmarshalJSON(data []byte) error {
	type alias domainPayload
	var raw struct {
		alias
		RawPort json.RawMessage `json:"containerPort"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*p = domainPayload(raw.alias)
	if len(raw.RawPort) > 0 {
		var intPort int
		if err := json.Unmarshal(raw.RawPort, &intPort); err == nil {
			p.ContainerPort = intPort
		} else {
			var strPort string
			if err := json.Unmarshal(raw.RawPort, &strPort); err == nil {
				if parsed, err := strconv.Atoi(strings.TrimSpace(strPort)); err == nil {
					p.ContainerPort = parsed
				}
			}
		}
	}
	return nil
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
	mux.HandleFunc("POST /api/domains/verify-dns", h.middleware.RequirePermission(auth.PermissionManageDomains, h.handleVerifyDomainDNS))
	mux.HandleFunc("PUT /api/domains/{id}", h.middleware.RequireAuth(h.handleUpdateDomain))
	mux.HandleFunc("DELETE /api/domains/{id}", h.middleware.RequireAuth(h.handleDeleteDomain))
	mux.HandleFunc("POST /api/domains/{id}/revalidate", h.middleware.RequireAuth(h.handleRevalidateDomain))
	mux.HandleFunc("GET /api/traffic/requests", h.middleware.RequireAuth(h.handleTrafficRequests))
	mux.HandleFunc("GET /api/traffic/metrics", h.middleware.RequireAuth(h.handleTrafficMetrics))
	mux.HandleFunc("GET /api/ingress/caddyfile", h.middleware.RequireAdmin(h.handleGetCaddyfile))
	mux.HandleFunc("POST /api/ingress/reload", h.middleware.RequireAdmin(h.handleReloadCaddy))
	mux.HandleFunc("GET /api/system/panel-settings", h.middleware.RequireAdmin(h.handleGetPanelSettings))
	mux.HandleFunc("PUT /api/system/panel-settings", h.middleware.RequireAdmin(h.handleUpdatePanelSettings))
}

func (h *Handler) ensureDomainAccess(w http.ResponseWriter, r *http.Request, id string, requireManage bool) (*Domain, bool) {
	if requireManage && h.middleware != nil && !h.middleware.HasPermission(r, auth.PermissionManageDomains) {
		httputil.WriteError(w, http.StatusForbidden, "Permission 'canManageDomains' required")
		return nil, false
	}
	existing, err := h.service.GetDomain(r.Context(), id)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "Failed to verify domain access")
		return nil, false
	}
	if existing == nil {
		httputil.WriteError(w, http.StatusNotFound, "Domain rule not found")
		return nil, false
	}
	if h.middleware != nil && !h.middleware.CanAccessService(r, existing.ProjectID, existing.ServiceID) {
		httputil.WriteError(w, http.StatusForbidden, "Access to project denied")
		return nil, false
	}
	return existing, true
}

func (h *Handler) handleListDomains(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("projectId")
	if projectID != "" && h.middleware != nil && !h.middleware.CanAccessProject(r, projectID) {
		httputil.WriteError(w, http.StatusForbidden, "Access to project denied")
		return
	}
	domains, err := h.service.ListDomains(r.Context(), projectID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if h.middleware != nil {
		if restricted, allowed := h.middleware.AllowedProjectSet(r); restricted {
			filtered := make([]Domain, 0, len(domains))
			for _, d := range domains {
				if allowed[d.ProjectID] && h.middleware.CanAccessService(r, d.ProjectID, d.ServiceID) {
					filtered = append(filtered, d)
				}
			}
			domains = filtered
		}
	}
	httputil.WriteJSON(w, http.StatusOK, domains)
}

func (h *Handler) handleCreateDomain(w http.ResponseWriter, r *http.Request) {
	if h.middleware != nil && !h.middleware.HasPermission(r, auth.PermissionManageDomains) {
		httputil.WriteError(w, http.StatusForbidden, "Permission 'canManageDomains' required")
		return
	}
	var p domainPayload
	if err := httputil.DecodeJSONBody(w, r, &p, 0); err != nil {
		httputil.WriteDecodeError(w, err)
		return
	}
	if h.middleware != nil && !h.middleware.CanAccessService(r, p.ProjectID, p.ServiceID) {
		httputil.WriteError(w, http.StatusForbidden, "Access to project or service denied")
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
	existing, ok := h.ensureDomainAccess(w, r, id, true)
	if !ok {
		return
	}
	var p domainPayload
	if err := httputil.DecodeJSONBody(w, r, &p, 0); err != nil {
		httputil.WriteDecodeError(w, err)
		return
	}
	d := p.toDomain()
	d.ID = id
	// Prevent moving domain ownership across projects or to unpermitted services.
	d.ProjectID = existing.ProjectID
	if d.ServiceID == "" {
		d.ServiceID = existing.ServiceID
	}
	if h.middleware != nil && !h.middleware.CanAccessService(r, d.ProjectID, d.ServiceID) {
		httputil.WriteError(w, http.StatusForbidden, "Access to service denied")
		return
	}

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
	if _, ok := h.ensureDomainAccess(w, r, id, true); !ok {
		return
	}
	if err := h.service.DeleteDomain(r.Context(), id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Domain rule deleted"})
}

func (h *Handler) authorizeTrafficFilter(w http.ResponseWriter, r *http.Request, filter *TrafficFilter) (bool, map[string]bool, bool) {
	if h.middleware == nil {
		return false, nil, true
	}
	if !h.middleware.HasPermission(r, auth.PermissionViewMonitoring) {
		httputil.WriteError(w, http.StatusForbidden, "Permission 'canViewMonitoring' required")
		return true, nil, false
	}
	if filter.ProjectID != "" && filter.ProjectID != "all" {
		if !h.middleware.CanAccessProject(r, filter.ProjectID) {
			httputil.WriteError(w, http.StatusForbidden, "Access to project denied")
			return true, nil, false
		}
	}
	restricted, allowed := h.middleware.AllowedProjectSet(r)
	if restricted && filter.ServiceID != "" && filter.ServiceID != "all" && filter.ProjectID != "" {
		if !h.middleware.CanAccessService(r, filter.ProjectID, filter.ServiceID) {
			httputil.WriteError(w, http.StatusForbidden, "Access to service denied")
			return true, nil, false
		}
	}
	return restricted, allowed, true
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

	restricted, allowed, ok := h.authorizeTrafficFilter(w, r, &filter)
	if !ok {
		return
	}

	logs := h.service.GetTrafficRequests(filter)
	if restricted {
		filtered := make([]AccessLogItem, 0, len(logs))
		for _, item := range logs {
			if item.ProjectID != "" && allowed[item.ProjectID] && h.middleware.CanAccessService(r, item.ProjectID, item.ServiceID) {
				filtered = append(filtered, item)
			}
		}
		logs = filtered
	}
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

	restricted, allowed, ok := h.authorizeTrafficFilter(w, r, &filter)
	if !ok {
		return
	}
	if restricted {
		filter.Limit = 500
		logs := h.service.GetTrafficRequests(filter)
		var scoped []AccessLogItem
		for _, item := range logs {
			if item.ProjectID != "" && allowed[item.ProjectID] && h.middleware.CanAccessService(r, item.ProjectID, item.ServiceID) {
				scoped = append(scoped, item)
			}
		}
		httputil.WriteJSON(w, http.StatusOK, computeTrafficMetrics(scoped))
		return
	}

	metrics := h.service.GetTrafficMetrics(filter)
	httputil.WriteJSON(w, http.StatusOK, metrics)
}

func computeTrafficMetrics(items []AccessLogItem) TrafficMetrics {
	total := len(items)
	if total == 0 {
		return TrafficMetrics{SuccessRate: 100.0}
	}
	var s2xx3xx, s4xx, s5xx int
	var totalDuration float64
	for _, l := range items {
		if l.Status >= 200 && l.Status < 400 {
			s2xx3xx++
		} else if l.Status >= 400 && l.Status < 500 {
			s4xx++
		} else if l.Status >= 500 {
			s5xx++
		}
		totalDuration += l.DurationMs
	}
	return TrafficMetrics{
		TotalRequests: total,
		SuccessRate:   (float64(s2xx3xx) / float64(total)) * 100.0,
		Error4xx:      s4xx,
		Error5xx:      s5xx,
		AvgLatency:    totalDuration / float64(total),
	}
}

func (h *Handler) handleVerifyDomainDNS(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Hostname string `json:"hostname"`
	}
	if err := httputil.DecodeJSONBody(w, r, &body, 0); err != nil {
		httputil.WriteDecodeError(w, err)
		return
	}
	res, err := h.service.VerifyDomainDNS(r.Context(), body.Hostname)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, res)
}

func (h *Handler) handleRevalidateDomain(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.WriteError(w, http.StatusBadRequest, "Domain ID is required")
		return
	}
	if _, ok := h.ensureDomainAccess(w, r, id, true); !ok {
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
		// "Not applied" is an upstream-availability/config problem, not a
		// caller mistake: previously every outcome answered 200 "reloaded
		// successfully" (the visible contract change per the ingress audit).
		status := http.StatusInternalServerError
		if errors.Is(err, ErrCaddyNotApplied) {
			status = http.StatusServiceUnavailable
		}
		httputil.WriteError(w, status, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Caddy ingress reloaded successfully"})
}
