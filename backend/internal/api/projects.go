package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"gopod/internal/db"
	"gopod/pkg/httputil"
)

func (h *Handler) handleListProjects(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		httputil.WriteJSON(w, http.StatusOK, []interface{}{})
		return
	}
	projects, err := h.repo.ListProjects(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, projects)
}

func (h *Handler) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var p db.Project
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if p.ID == "" {
		p.ID = strings.ToLower(strings.ReplaceAll(p.Name, " ", "-"))
	}
	p.CreatedAt = time.Now()

	if err := h.repo.CreateProject(r.Context(), p); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, p)
}

func (h *Handler) handleGetProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := h.repo.GetProject(r.Context(), id)
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

func (h *Handler) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.repo.DeleteProject(r.Context(), id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Project deleted"})
}
