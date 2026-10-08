package credentials

import (
	"encoding/json"
	"net/http"

	"gopod/internal/auth"
	"gopod/pkg/httputil"
)

// Handler handles secrets, SSH deploy keys, and registry credentials.
type Handler struct {
	service    *Service
	middleware *auth.Middleware
}

// NewHandler creates a new credentials domain handler.
func NewHandler(service *Service, mw *auth.Middleware) *Handler {
	return &Handler{
		service:    service,
		middleware: mw,
	}
}

// RegisterRoutes registers credentials routes on the mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/credentials/ssh-keys", h.middleware.RequireAuth(h.handleListSSHKeys))
	mux.HandleFunc("POST /api/credentials/ssh-keys", h.middleware.RequireAuth(h.handleCreateSSHKey))
	mux.HandleFunc("POST /api/credentials/ssh-keys/generate", h.middleware.RequireAuth(h.handleGenerateSSHKey))
	mux.HandleFunc("DELETE /api/credentials/ssh-keys/{id}", h.middleware.RequireAuth(h.handleDeleteSSHKey))

	mux.HandleFunc("GET /api/credentials/registries", h.middleware.RequireAuth(h.handleListRegistries))
	mux.HandleFunc("POST /api/credentials/registries", h.middleware.RequireAuth(h.handleCreateRegistry))
	mux.HandleFunc("DELETE /api/credentials/registries/{id}", h.middleware.RequireAuth(h.handleDeleteRegistry))

	mux.HandleFunc("GET /api/credentials/secrets", h.middleware.RequireAuth(h.handleListSecrets))
	mux.HandleFunc("POST /api/credentials/secrets", h.middleware.RequireAuth(h.handleCreateSecret))
	mux.HandleFunc("DELETE /api/credentials/secrets/{id}", h.middleware.RequireAuth(h.handleDeleteSecret))
}

// ── SSH Keys ──

func (h *Handler) handleListSSHKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := h.service.ListSSHKeys(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, keys)
}

func (h *Handler) handleCreateSSHKey(w http.ResponseWriter, r *http.Request) {
	var k SSHKey
	if err := json.NewDecoder(r.Body).Decode(&k); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	created, err := h.service.CreateSSHKey(r.Context(), k)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, created)
}

func (h *Handler) handleGenerateSSHKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	key, err := h.service.GenerateSSHKeyPair(r.Context(), req.Name)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, key)
}

func (h *Handler) handleDeleteSSHKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.service.DeleteSSHKey(r.Context(), id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "SSH key deleted"})
}

// ── Registries ──

func (h *Handler) handleListRegistries(w http.ResponseWriter, r *http.Request) {
	registries, err := h.service.ListRegistries(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, registries)
}

func (h *Handler) handleCreateRegistry(w http.ResponseWriter, r *http.Request) {
	var reg ContainerRegistry
	if err := json.NewDecoder(r.Body).Decode(&reg); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	created, err := h.service.CreateRegistry(r.Context(), reg)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, created)
}

func (h *Handler) handleDeleteRegistry(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.service.DeleteRegistry(r.Context(), id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Registry credentials deleted"})
}

// ── Secrets ──

func (h *Handler) handleListSecrets(w http.ResponseWriter, r *http.Request) {
	secrets, err := h.service.ListSecrets(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, secrets)
}

func (h *Handler) handleCreateSecret(w http.ResponseWriter, r *http.Request) {
	var sec Secret
	if err := json.NewDecoder(r.Body).Decode(&sec); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	created, err := h.service.CreateSecret(r.Context(), sec)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, created)
}

func (h *Handler) handleDeleteSecret(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.service.DeleteSecret(r.Context(), id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Secret deleted"})
}
