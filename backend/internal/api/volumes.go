package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"gopod/internal/db"
	"gopod/pkg/httputil"
)

func (h *Handler) handleListVolumeSnapshots(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("projectId")
	snapshots, err := h.repo.ListVolumeSnapshots(r.Context(), projectID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, snapshots)
}

func (h *Handler) handleCreateVolumeSnapshot(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProjectID  string `json:"projectId"`
		ServiceID  string `json:"serviceId"`
		VolumeName string `json:"volumeName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	filename, sizeBytes, _ := h.deployer.SnapshotVolume(r.Context(), req.VolumeName)
	snapshot := db.VolumeSnapshot{
		ID:          fmt.Sprintf("snap-%d", time.Now().UnixNano()),
		ProjectID:   req.ProjectID,
		ServiceID:   req.ServiceID,
		VolumeName:  req.VolumeName,
		Filename:    filename,
		Size:        fmt.Sprintf("%.1f MB", float64(sizeBytes)/(1024*1024)),
		SizeBytes:   sizeBytes,
		Status:      "completed",
		Compression: "zstd",
		CreatedAt:   time.Now(),
	}

	if err := h.repo.CreateVolumeSnapshot(r.Context(), snapshot); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, snapshot)
}

func (h *Handler) handleDeleteVolumeSnapshot(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.repo.DeleteVolumeSnapshot(r.Context(), id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Snapshot deleted"})
}

func (h *Handler) handleListVolumeSchedules(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("projectId")
	schedules, err := h.repo.ListVolumeSchedules(r.Context(), projectID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, schedules)
}

func (h *Handler) handleToggleVolumeSchedule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.repo.ToggleVolumeSchedule(r.Context(), id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Schedule updated"})
}
