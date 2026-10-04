package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"gopod/internal/db"
	"gopod/pkg/httputil"
)

func (h *Handler) handleListServices(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("projectId")
	services, err := h.repo.ListServices(r.Context(), projectID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, services)
}

func (h *Handler) handleCreateService(w http.ResponseWriter, r *http.Request) {
	var s db.Service
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	if s.ID == "" {
		s.ID = strings.ToLower(s.ProjectID + "-" + strings.ReplaceAll(s.Name, " ", "-"))
	}
	s.CreatedAt = time.Now()

	if err := h.repo.CreateService(r.Context(), s); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, s)
}

func (h *Handler) handleGetService(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s, err := h.repo.GetService(r.Context(), id)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if s == nil {
		httputil.WriteError(w, http.StatusNotFound, "Service not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, s)
}

func (h *Handler) handleDeleteService(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.repo.DeleteService(r.Context(), id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Service deleted"})
}
