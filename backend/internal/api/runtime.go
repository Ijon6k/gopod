package api

import (
	"net/http"
	"strconv"

	"gopod/pkg/httputil"
)

// handleContainerStart starts a container by ID.
func (h *Handler) handleContainerStart(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.WriteError(w, http.StatusBadRequest, "Missing container id")
		return
	}
	if err := h.client.StartContainer(r.Context(), id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Container started", "id": id})
}

// handleContainerStop stops a running container.
func (h *Handler) handleContainerStop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.WriteError(w, http.StatusBadRequest, "Missing container id")
		return
	}
	if err := h.client.StopContainer(r.Context(), id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Container stopped", "id": id})
}

// handleContainerRestart restarts a container.
func (h *Handler) handleContainerRestart(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.WriteError(w, http.StatusBadRequest, "Missing container id")
		return
	}
	if err := h.client.RestartContainer(r.Context(), id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Container restarted", "id": id})
}

// handleContainerDelete deletes a container.
func (h *Handler) handleContainerDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.WriteError(w, http.StatusBadRequest, "Missing container id")
		return
	}
	force := r.URL.Query().Get("force") == "true"
	if err := h.client.DeleteContainer(r.Context(), id, force); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Container deleted", "id": id})
}

// handleContainerLogs returns container logs.
func (h *Handler) handleContainerLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.WriteError(w, http.StatusBadRequest, "Missing container id")
		return
	}
	tail := 100
	if tailParam := r.URL.Query().Get("tail"); tailParam != "" {
		if t, err := strconv.Atoi(tailParam); err == nil && t > 0 {
			tail = t
		}
	}

	logs, err := h.client.GetContainerLogs(r.Context(), id, tail)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"id":   id,
		"logs": logs,
		"tail": tail,
	})
}

// handleListPods returns all Podman pods.
func (h *Handler) handleListPods(w http.ResponseWriter, r *http.Request) {
	pods, err := h.client.GetPods(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, pods)
}

// handleListImages returns all container images in local storage.
func (h *Handler) handleListImages(w http.ResponseWriter, r *http.Request) {
	images, err := h.client.GetImages(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, images)
}

// handlePruneImages removes unused images.
func (h *Handler) handlePruneImages(w http.ResponseWriter, r *http.Request) {
	all := r.URL.Query().Get("all") == "true"
	if err := h.client.PruneImages(r.Context(), all); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Images pruned successfully"})
}

// handleListVolumes returns all Podman storage volumes.
func (h *Handler) handleListVolumes(w http.ResponseWriter, r *http.Request) {
	vols, err := h.client.GetVolumes(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, vols)
}

// handlePruneVolumes removes unused storage volumes.
func (h *Handler) handlePruneVolumes(w http.ResponseWriter, r *http.Request) {
	if err := h.client.PruneVolumes(r.Context()); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Volumes pruned successfully"})
}

// handleListNetworks returns all Podman networks.
func (h *Handler) handleListNetworks(w http.ResponseWriter, r *http.Request) {
	nets, err := h.client.GetNetworks(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, nets)
}

// handlePruneSystem cleans up stopped containers, unused networks, and dangling images.
func (h *Handler) handlePruneSystem(w http.ResponseWriter, r *http.Request) {
	if err := h.client.PruneSystem(r.Context()); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "System prune completed successfully"})
}
