package projects

import (
	"net/http"

	"gopod/internal/auth"
	"gopod/pkg/httputil"
)

// Handler handles project workspace HTTP endpoints.
type Handler struct {
	service    *Service
	middleware *auth.Middleware
}

// NewHandler creates a new projects domain handler.
func NewHandler(service *Service, mw *auth.Middleware) *Handler {
	return &Handler{service: service, middleware: mw}
}

// RegisterRoutes registers project endpoints on the mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/projects", h.middleware.RequireAuth(h.handleList))
	mux.HandleFunc("POST /api/projects", h.middleware.RequireAuth(h.handleCreate))
	mux.HandleFunc("GET /api/projects/{id}", h.middleware.RequireAuth(h.handleGet))
	mux.HandleFunc("DELETE /api/projects/{id}", h.middleware.RequireAuth(h.handleDelete))
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	list, err := h.service.ListProjects(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if restricted, allowed := h.middleware.AllowedProjectSet(r); restricted {
		filtered := make([]Project, 0, len(list))
		for _, p := range list {
			if allowed[p.ID] {
				filtered = append(filtered, p)
			}
		}
		list = filtered
	}
	httputil.WriteJSON(w, http.StatusOK, list)
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var input CreateProjectInput
	if err := httputil.DecodeJSONBody(w, r, &input, 0); err != nil {
		httputil.WriteDecodeError(w, err)
		return
	}

	project, err := h.service.CreateProject(r.Context(), input)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if h.middleware != nil {
		h.middleware.GrantProjectOwner(r, project.ID)
	}

	httputil.WriteJSON(w, http.StatusCreated, project)
}

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if h.middleware != nil && !h.middleware.CanAccessProject(r, id) {
		httputil.WriteError(w, http.StatusForbidden, "Access to project denied")
		return
	}
	p, err := h.service.GetProject(r.Context(), id)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if p == nil {
		httputil.WriteError(w, http.StatusNotFound, "Project not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, p)
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if h.middleware != nil && !h.middleware.CanAccessProject(r, id) {
		httputil.WriteError(w, http.StatusForbidden, "Access to project denied")
		return
	}
	if err := h.service.DeleteProject(r.Context(), id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Project deleted"})
}
