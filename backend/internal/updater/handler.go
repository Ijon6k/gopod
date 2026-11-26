package updater

import (
	"errors"
	"net/http"
	"strings"

	"gopod/internal/auth"
	"gopod/pkg/httputil"
)

// Handler exposes read-only version metadata and admin-only update checks & execution.
type Handler struct {
	service    *Service
	middleware *auth.Middleware
}

// NewHandler creates an HTTP handler for version checking and self-updates.
func NewHandler(svc *Service, mw *auth.Middleware) *Handler {
	return &Handler{service: svc, middleware: mw}
}

// RegisterRoutes registers updater endpoints on mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/system/version", h.middleware.RequireAuth(h.handleVersion))
	mux.HandleFunc("GET /api/system/updates", h.middleware.RequireAdmin(h.handleCheckUpdates))
	mux.HandleFunc("POST /api/system/update", h.middleware.RequireAdmin(h.handleTriggerUpdate))
}

func (h *Handler) handleVersion(w http.ResponseWriter, r *http.Request) {
	ver, commit, buildDate := resolveBuildMetadata()
	self := h.service.InspectSelf(r.Context())
	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"version":     ver,
		"buildCommit": commit,
		"buildDate":   buildDate,
		"channel":     ValidateChannel(Channel),
		"mode":        self.Mode,
		"isDevBuild":  self.IsDevBuild,
	})
}

func (h *Handler) handleCheckUpdates(w http.ResponseWriter, r *http.Request) {
	channel := r.URL.Query().Get("channel")
	force := r.URL.Query().Get("force") == "true"
	res, err := h.service.CheckForUpdates(r.Context(), channel, force)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, res)
}

func (h *Handler) handleTriggerUpdate(w http.ResponseWriter, r *http.Request) {
	// CSRF defense-in-depth: reject cross-site browser requests even if Origin slipped past CORS.
	if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" && !httputil.IsOriginAllowed(origin, r.Host) {
		httputil.WriteError(w, http.StatusForbidden, "Untrusted request origin")
		return
	}

	var req struct {
		Tag                  string `json:"tag"`
		Channel              string `json:"channel"`
		ConfirmSwitchFromDev bool   `json:"confirmSwitchFromDev"`
	}
	if r.Body != nil {
		_ = httputil.DecodeJSONBody(w, r, &req, 0)
	}

	actorEmail := "admin"
	if user, ok := auth.UserFromContext(r.Context()); ok && user != nil {
		actorEmail = user.Email
	}
	st, err := h.service.StartUpdate(r.Context(), req.Tag, req.Channel, req.ConfirmSwitchFromDev, actorEmail, auth.ClientIP(r))
	if err != nil {
		switch {
		case errors.Is(err, ErrUpdateInProgress):
			httputil.WriteError(w, http.StatusConflict, err.Error())
		case errors.Is(err, ErrDevBuildProtected):
			httputil.WriteError(w, http.StatusConflict, err.Error())
		default:
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	httputil.WriteJSON(w, http.StatusAccepted, st)
}

