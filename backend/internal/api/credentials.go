package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"gopod/internal/db"
	"gopod/pkg/httputil"
)

// ── SSH Keys ──

func (h *Handler) handleListSSHKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := h.repo.ListSSHKeys(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if keys == nil {
		keys = []db.SSHKey{}
	}
	httputil.WriteJSON(w, http.StatusOK, keys)
}

func (h *Handler) handleCreateSSHKey(w http.ResponseWriter, r *http.Request) {
	var k db.SSHKey
	if err := json.NewDecoder(r.Body).Decode(&k); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	if k.ID == "" {
		b := make([]byte, 4)
		_, _ = rand.Read(b)
		k.ID = fmt.Sprintf("key-%s", hex.EncodeToString(b))
	}
	if k.Fingerprint == "" && k.PublicKey != "" {
		hash := sha256.Sum256([]byte(k.PublicKey))
		k.Fingerprint = fmt.Sprintf("SHA256:%s", hex.EncodeToString(hash[:16]))
	}
	if k.Type == "" {
		k.Type = "ed25519"
	}
	k.CreatedAt = time.Now()

	if err := h.repo.CreateSSHKey(r.Context(), k); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, k)
}

func (h *Handler) handleDeleteSSHKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.repo.DeleteSSHKey(r.Context(), id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "SSH key deleted"})
}

// ── Registries ──

func (h *Handler) handleListRegistries(w http.ResponseWriter, r *http.Request) {
	regs, err := h.repo.ListRegistries(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if regs == nil {
		regs = []db.ContainerRegistry{}
	}
	httputil.WriteJSON(w, http.StatusOK, regs)
}

func (h *Handler) handleCreateRegistry(w http.ResponseWriter, r *http.Request) {
	var reg db.ContainerRegistry
	if err := json.NewDecoder(r.Body).Decode(&reg); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	if reg.ID == "" {
		b := make([]byte, 4)
		_, _ = rand.Read(b)
		reg.ID = fmt.Sprintf("reg-%s", hex.EncodeToString(b))
	}
	reg.CreatedAt = time.Now()

	if err := h.repo.CreateRegistry(r.Context(), reg); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, reg)
}

func (h *Handler) handleDeleteRegistry(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.repo.DeleteRegistry(r.Context(), id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Registry deleted"})
}

// ── Secrets ──

func (h *Handler) handleListSecrets(w http.ResponseWriter, r *http.Request) {
	secrets, err := h.repo.ListSecrets(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if secrets == nil {
		secrets = []db.Secret{}
	}
	httputil.WriteJSON(w, http.StatusOK, secrets)
}

func (h *Handler) handleCreateSecret(w http.ResponseWriter, r *http.Request) {
	var s db.Secret
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	if s.ID == "" {
		b := make([]byte, 4)
		_, _ = rand.Read(b)
		s.ID = fmt.Sprintf("sec-%s", hex.EncodeToString(b))
	}
	if s.Driver == "" {
		s.Driver = "file"
	}
	s.CreatedAt = time.Now()

	if err := h.repo.CreateSecret(r.Context(), s); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, s)
}

func (h *Handler) handleDeleteSecret(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.repo.DeleteSecret(r.Context(), id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Secret deleted"})
}
