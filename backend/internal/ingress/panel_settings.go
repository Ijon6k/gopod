package ingress

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"gopod/pkg/httputil"
)

// PanelSettings governs Caddy control-plane domain routing and catch-all :80 lock.
type PanelSettings struct {
	Domain       string `json:"domain"`
	LockCatchall bool   `json:"lockCatchall"`
	TLS          bool   `json:"tls"`
}

func sanitizePanelHost(raw string) string {
	clean := strings.ToLower(strings.TrimSpace(raw))
	if clean == "" {
		return ""
	}
	if strings.Contains(clean, "://") {
		if u, err := url.Parse(clean); err == nil && u.Hostname() != "" {
			clean = strings.ToLower(u.Hostname())
		}
	}
	clean = strings.TrimSuffix(clean, "/")
	return clean
}

// ValidatePanelSettings verifies a control-plane domain setting before saving or
// rendering into Caddyfile. Prevents lock-out with an empty domain and rejects
// injection metacharacters or wildcards.
func ValidatePanelSettings(ps *PanelSettings) error {
	clean := sanitizePanelHost(ps.Domain)
	if clean != "" {
		if strings.ContainsRune(clean, '*') {
			return fmt.Errorf("%w: panel domain cannot contain wildcard '*'", ErrValidation)
		}
		if strings.ContainsRune(clean, ':') || strings.ContainsAny(clean, caddyfileMetachars) {
			return fmt.Errorf("%w: panel domain %q must be a bare hostname without port or special characters", ErrValidation, ps.Domain)
		}
		if err := ValidateHostname(clean); err != nil {
			return err
		}
	}
	ps.Domain = clean
	if ps.LockCatchall && ps.Domain == "" {
		return fmt.Errorf("%w: set a panel domain before locking the :80 catch-all listener", ErrValidation)
	}
	return nil
}

// SetPanelSettings updates the in-memory panel routing configuration safely.
func (r *Reconciler) SetPanelSettings(ps PanelSettings) {
	r.mu.Lock()
	r.panelSettings = ps
	r.mu.Unlock()
}

// GetPanelSettings returns the current control-plane routing configuration.
func (r *Reconciler) GetPanelSettings() PanelSettings {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.panelSettings
}

// SetSettingsDB installs the database handle used to persist panel settings.
func (s *Service) SetSettingsDB(db *sql.DB) {
	s.settingsDB = db
}

// LoadPanelSettings loads persisted control-plane domain settings from SQLite
// and pushes them onto the Caddy reconciler.
func (s *Service) LoadPanelSettings(ctx context.Context) (PanelSettings, error) {
	var ps PanelSettings
	if s.settingsDB == nil {
		if s.reconciler != nil {
			ps = s.reconciler.GetPanelSettings()
		}
		return ps, nil
	}
	rows, err := s.settingsDB.QueryContext(ctx, `SELECT key, value FROM system_settings WHERE key IN ('panel_domain', 'panel_lock_catchall', 'panel_tls')`)
	if err != nil {
		return ps, err
	}
	defer rows.Close()

	for rows.Next() {
		var k, v string
		if scanErr := rows.Scan(&k, &v); scanErr != nil {
			continue
		}
		switch k {
		case "panel_domain":
			ps.Domain = v
		case "panel_lock_catchall":
			ps.LockCatchall = v == "true" || v == "1"
		case "panel_tls":
			ps.TLS = v == "true" || v == "1"
		}
	}
	_ = ValidatePanelSettings(&ps)
	if s.reconciler != nil {
		s.reconciler.SetPanelSettings(ps)
	}
	return ps, rows.Err()
}

// SavePanelSettings validates, persists, and reconciles control-plane domain settings.
func (s *Service) SavePanelSettings(ctx context.Context, ps PanelSettings) (PanelSettings, error) {
	if err := ValidatePanelSettings(&ps); err != nil {
		return PanelSettings{}, err
	}
	if s.settingsDB != nil {
		tx, err := s.settingsDB.BeginTx(ctx, nil)
		if err != nil {
			return PanelSettings{}, err
		}
		upsert := `
			INSERT INTO system_settings (key, value, updated_at)
			VALUES (?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP
		`
		pairs := [][2]string{
			{"panel_domain", ps.Domain},
			{"panel_lock_catchall", fmt.Sprintf("%t", ps.LockCatchall)},
			{"panel_tls", fmt.Sprintf("%t", ps.TLS)},
		}
		for _, kv := range pairs {
			if _, err := tx.ExecContext(ctx, upsert, kv[0], kv[1]); err != nil {
				_ = tx.Rollback()
				return PanelSettings{}, err
			}
		}
		if err := tx.Commit(); err != nil {
			return PanelSettings{}, err
		}
	}
	if s.reconciler != nil {
		s.reconciler.SetPanelSettings(ps)
		if s.repo != nil {
			domains, err := s.repo.List(ctx, "")
			if err == nil {
				_ = s.reconciler.Reconcile(ctx, domains)
			}
		}
	}
	return ps, nil
}

func (h *Handler) handleGetPanelSettings(w http.ResponseWriter, r *http.Request) {
	ps, err := h.service.LoadPanelSettings(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, ps)
}

func (h *Handler) handleUpdatePanelSettings(w http.ResponseWriter, r *http.Request) {
	var ps PanelSettings
	if err := httputil.DecodeJSONBody(w, r, &ps, 0); err != nil {
		httputil.WriteDecodeError(w, err)
		return
	}
	saved, err := h.service.SavePanelSettings(r.Context(), ps)
	if err != nil {
		if errors.Is(err, ErrValidation) {
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, saved)
}

