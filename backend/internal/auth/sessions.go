package auth

import (
	"context"
	"net/http"
	"strings"
	"unicode"

	"gopod/pkg/httputil"
)

const maxUserAgentLen = 256

// SanitizeUserAgent strips control characters and bounds length before storage
// so attacker-controlled User-Agent strings cannot inject newlines into logs
// or overflow the sessions table UI.
func SanitizeUserAgent(raw string) string {
	var b strings.Builder
	b.Grow(len(raw))
	count := 0
	for _, r := range strings.TrimSpace(raw) {
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
		count++
		if count >= maxUserAgentLen {
			break
		}
	}
	return b.String()
}

// DescribeUserAgent converts a User-Agent string into a concise browser + OS label.
func DescribeUserAgent(ua string) string {
	lower := strings.ToLower(ua)
	if lower == "" {
		return "API / Unknown client"
	}

	browser := "Browser"
	switch {
	case strings.Contains(lower, "curl/"):
		return "curl CLI"
	case strings.Contains(lower, "edg/") || strings.Contains(lower, "edge/"):
		browser = "Edge"
	case strings.Contains(lower, "firefox/"):
		browser = "Firefox"
	case strings.Contains(lower, "opr/") || strings.Contains(lower, "opera"):
		browser = "Opera"
	case strings.Contains(lower, "chrome/") || strings.Contains(lower, "crios"):
		browser = "Chrome"
	case strings.Contains(lower, "safari/"):
		browser = "Safari"
	}

	osName := "Unknown OS"
	switch {
	case strings.Contains(lower, "iphone") || strings.Contains(lower, "ipad"):
		osName = "iOS"
	case strings.Contains(lower, "android"):
		osName = "Android"
	case strings.Contains(lower, "windows"):
		osName = "Windows"
	case strings.Contains(lower, "mac os x") || strings.Contains(lower, "macintosh"):
		osName = "macOS"
	case strings.Contains(lower, "linux"):
		osName = "Linux"
	}

	return browser + " on " + osName
}

// ListUserSessions returns all non-expired sessions for userID, marking the
// session matching currentToken as IsCurrent.
func (s *Service) ListUserSessions(ctx context.Context, userID, currentToken string) ([]SessionSummary, error) {
	if s == nil || s.repo == nil {
		return []SessionSummary{}, nil
	}
	list, err := s.repo.ListUserSessions(ctx, userID)
	if err != nil {
		return nil, err
	}
	currentID, _ := s.repo.GetSessionIDByToken(ctx, currentToken)
	for i := range list {
		if currentID != "" && list[i].ID == currentID {
			list[i].IsCurrent = true
		}
	}
	return list, nil
}

// RevokeUserSession revokes sessionID only if owned by userID. Returns (deleted, isCurrent, err).
func (s *Service) RevokeUserSession(ctx context.Context, userID, sessionID, currentToken string) (bool, bool, error) {
	if s == nil || s.repo == nil {
		return false, false, nil
	}
	currentID, _ := s.repo.GetSessionIDByToken(ctx, currentToken)
	isCurrent := currentID != "" && currentID == sessionID
	ok, err := s.repo.DeleteSessionByID(ctx, sessionID, userID)
	return ok, isCurrent && ok, err
}

// RevokeOtherUserSessions invalidates every session of userID except currentToken.
func (s *Service) RevokeOtherUserSessions(ctx context.Context, userID, currentToken string) (int64, error) {
	if s == nil || s.repo == nil {
		return 0, nil
	}
	return s.repo.DeleteOtherSessions(ctx, userID, currentToken)
}

func (h *Handler) handleListMySessions(w http.ResponseWriter, r *http.Request) {
	_, user, err := h.service.GetSessionUser(r)
	if err != nil || user == nil {
		httputil.WriteError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	tok := ExtractTokenHeader(r)
	sessions, err := h.service.ListUserSessions(r.Context(), user.ID, tok)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "Failed to list active sessions")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, sessions)
}

func (h *Handler) handleRevokeMySession(w http.ResponseWriter, r *http.Request) {
	_, user, err := h.service.GetSessionUser(r)
	if err != nil || user == nil {
		httputil.WriteError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	sid := strings.TrimSpace(r.PathValue("id"))
	if sid == "" {
		httputil.WriteError(w, http.StatusBadRequest, "Session ID is required")
		return
	}
	tok := ExtractTokenHeader(r)
	deleted, wasCurrent, err := h.service.RevokeUserSession(r.Context(), user.ID, sid, tok)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "Failed to revoke session")
		return
	}
	if !deleted {
		httputil.WriteError(w, http.StatusNotFound, "Session not found")
		return
	}
	if wasCurrent {
		setSessionCookie(w, r, "", -1)
	}
	if h.service.audit != nil {
		_ = h.service.audit.Record(r.Context(), "auth.session_revoked", user.Email, sid, "security", ClientIP(r), "success")
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"success": true, "revokedCurrent": wasCurrent})
}

func (h *Handler) handleRevokeOtherSessions(w http.ResponseWriter, r *http.Request) {
	_, user, err := h.service.GetSessionUser(r)
	if err != nil || user == nil {
		httputil.WriteError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	tok := ExtractTokenHeader(r)
	count, err := h.service.RevokeOtherUserSessions(r.Context(), user.ID, tok)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "Failed to revoke other sessions")
		return
	}
	if h.service.audit != nil {
		_ = h.service.audit.Record(r.Context(), "auth.sessions_revoked_others", user.Email, user.ID, "security", ClientIP(r), "success")
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"success": true, "revokedCount": count})
}

