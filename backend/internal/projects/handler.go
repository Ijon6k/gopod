package projects

import (
	"encoding/json"
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
	projects, err := h.service.ListProjects(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, projects)
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var input CreateProjectInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	project, err := h.service.CreateProject(r.Context(), input)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, project)
}

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
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
	if err := h.service.DeleteProject(r.Context(), id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Project deleted"})
}
