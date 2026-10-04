package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"gopod/internal/db"
	"gopod/pkg/httputil"
)

func (h *Handler) handleListDomains(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("projectId")
	domains, err := h.repo.ListDomains(r.Context(), projectID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, domains)
}

func (h *Handler) handleCreateDomain(w http.ResponseWriter, r *http.Request) {
	var d db.Domain
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Invalid domain payload")
		return
	}
	if d.ID == "" {
		d.ID = fmt.Sprintf("d-%d", time.Now().UnixNano())
	}
	d.CreatedAt = time.Now()

	if err := h.repo.CreateDomain(r.Context(), d); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.triggerCaddyReconcile(r.Context())
	httputil.WriteJSON(w, http.StatusCreated, d)
}

func (h *Handler) handleUpdateDomain(w http.ResponseWriter, r *http.Request) {
	var d db.Domain
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Invalid domain payload")
		return
	}
	d.ID = r.PathValue("id")

	if err := h.repo.UpdateDomain(r.Context(), d); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.triggerCaddyReconcile(r.Context())
	httputil.WriteJSON(w, http.StatusOK, d)
}

func (h *Handler) handleDeleteDomain(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.repo.DeleteDomain(r.Context(), id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.triggerCaddyReconcile(r.Context())
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Domain removed"})
}

func (h *Handler) triggerCaddyReconcile(ctx context.Context) {
	if h.caddy == nil || h.repo == nil {
		return
	}
	allDomains, err := h.repo.ListDomains(ctx, "")
	if err == nil {
		_ = h.caddy.Reconcile(ctx, allDomains)
	}
}
