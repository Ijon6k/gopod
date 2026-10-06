package api

import (
	"encoding/json"
	"fmt"
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

	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		httputil.WriteError(w, http.StatusBadRequest, "Project name is required")
		return
	}

	if p.ID == "" {
		clean := strings.ToLower(p.Name)
		var b strings.Builder
		for _, r := range clean {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
				b.WriteRune(r)
			} else if r == ' ' || r == '_' {
				b.WriteRune('-')
			}
		}
		p.ID = strings.Trim(b.String(), "-")
		if p.ID == "" {
			p.ID = fmt.Sprintf("proj-%d", time.Now().Unix())
		}
	}
	p.CreatedAt = time.Now()

	if err := h.repo.CreateProject(r.Context(), p); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			httputil.WriteError(w, http.StatusConflict, "A project with this name or identifier already exists")
			return
		}
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
