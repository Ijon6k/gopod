package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"gopod/pkg/httputil"
)

const (
	webhookBodyLimit = 5 << 20
	webhookDedupeTTL = 10 * time.Minute
	webhookDedupeMax = 1024
)

var errInvalidWebhookSignature = errors.New("invalid webhook signature")

var webhookSkipKeywords = []string{
	"[skip ci]",
	"[ci skip]",
	"[no ci]",
	"[skip actions]",
	"[actions skip]",
}

// WebhookCommit carries the fields needed for watch-path and skip filters.
type WebhookCommit struct {
	ID       string   `json:"id"`
	Message  string   `json:"message"`
	Added    []string `json:"added"`
	Modified []string `json:"modified"`
	Removed  []string `json:"removed"`
}

// WebhookPushPayload matches GitHub, GitLab, and Gitea push events.
type WebhookPushPayload struct {
	Ref        string          `json:"ref"`
	Before     string          `json:"before"`
	After      string          `json:"after"`
	Created    bool            `json:"created"`
	Deleted    bool            `json:"deleted"`
	HeadCommit *WebhookCommit  `json:"head_commit"`
	Commits    []WebhookCommit `json:"commits"`
}

type webhookDeduper struct {
	mu   sync.Mutex
	seen map[string]time.Time
	ttl  time.Duration
	max  int
}

func newWebhookDeduper() *webhookDeduper {
	return &webhookDeduper{
		seen: make(map[string]time.Time),
		ttl:  webhookDedupeTTL,
		max:  webhookDedupeMax,
	}
}

func (d *webhookDeduper) firstSeen(key string) bool {
	if d == nil || key == "" {
		return true
	}
	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.seen == nil {
		d.seen = make(map[string]time.Time)
	}
	if at, ok := d.seen[key]; ok && now.Sub(at) < d.ttl {
		return false
	}
	if len(d.seen) >= d.max {
		d.evict(now)
	}
	d.seen[key] = now
	return true
}

func (d *webhookDeduper) evict(now time.Time) {
	for key, at := range d.seen {
		if now.Sub(at) >= d.ttl {
			delete(d.seen, key)
		}
	}
	for len(d.seen) >= d.max {
		var oldestKey string
		var oldest time.Time
		for key, at := range d.seen {
			if oldestKey == "" || at.Before(oldest) {
				oldestKey, oldest = key, at
			}
		}
		if oldestKey == "" {
			return
		}
		delete(d.seen, oldestKey)
	}
}

func (h *Handler) handleWebhook(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.PathValue("token"))
	if token == "" {
		httputil.WriteError(w, http.StatusBadRequest, "Missing webhook token")
		return
	}

	svc, err := h.service.GetServiceByWebhookToken(r.Context(), token)
	if err != nil || svc == nil {
		httputil.WriteError(w, http.StatusUnauthorized, "Invalid webhook token")
		return
	}
	if svc.WebhookSecretError {
		// Fail closed: an unreadable secret is not the same as "no secret".
		// Accepting unsigned deliveries here would turn a key-rotation mistake
		// into an authentication bypass.
		emitEngineEventSafe(r.Context(), "error", "service", svc.ID, svc.ProjectID, "webhook", "webhook.secret_unavailable",
			"Webhook delivery rejected because the signing secret cannot be decrypted", nil)
		httputil.WriteError(w, http.StatusServiceUnavailable, "Webhook signature secret is unavailable; rotate it to restore verification")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, webhookBodyLimit)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			httputil.WriteError(w, http.StatusRequestEntityTooLarge, "Webhook payload is too large")
			return
		}
		httputil.WriteError(w, http.StatusBadRequest, "Failed to read webhook payload")
		return
	}

	provider := normalizeWebhookProvider(svc.WebhookProvider)
	if provider == "" {
		provider = detectWebhookProvider(r.Header)
	}
	if err := verifyWebhookSignature(provider, svc.WebhookSecret, r.Header, body); err != nil {
		emitEngineEventSafe(r.Context(), "warn", "service", svc.ID, svc.ProjectID, "webhook", "webhook.rejected",
			"Webhook signature rejected", map[string]any{"provider": provider, "reason": err.Error()})
		httputil.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}

	event, hasEvent := webhookEventName(provider, r.Header)
	if hasEvent && !isPushEvent(provider, event) {
		if event == "ping" {
			httputil.WriteJSON(w, http.StatusOK, map[string]any{"status": "ok", "message": "Webhook is active"})
			return
		}
		writeWebhookIgnored(w, fmt.Sprintf("Ignored '%s' event; only push events deploy", event))
		return
	}

	var payload WebhookPushPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Invalid webhook payload")
		return
	}

	if !strings.HasPrefix(payload.Ref, "refs/heads/") {
		writeWebhookIgnored(w, "Ignored non-branch ref")
		return
	}
	if reason := webhookPushSkipReason(svc, &payload); reason != "" {
		writeWebhookIgnored(w, reason)
		return
	}
	branch := strings.TrimPrefix(payload.Ref, "refs/heads/")

	if delivery := webhookDeliveryID(r.Header); delivery != "" {
		if !h.deduper.firstSeen(token + "|" + delivery) {
			writeWebhookIgnored(w, "Duplicate webhook delivery")
			return
		}
	}

	emitEngineEventSafe(r.Context(), "info", "service", svc.ID, svc.ProjectID, "webhook", "webhook.received",
		fmt.Sprintf("Push received for branch '%s'", branch), map[string]any{
			"provider": provider,
			"delivery": webhookDeliveryID(r.Header),
			"ref":      payload.Ref,
			"after":    payload.After,
		})

	if _, err := h.service.DeployByWebhook(r.Context(), token); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httputil.WriteJSON(w, http.StatusAccepted, map[string]any{
		"message": "Deployment triggered successfully",
		"service": svc.Name,
		"branch":  branch,
		"commit":  payload.After,
	})
}

func webhookPushSkipReason(svc *Service, payload *WebhookPushPayload) string {
	branch := strings.TrimPrefix(payload.Ref, "refs/heads/")
	targetBranch := strings.TrimSpace(svc.GitBranch)
	if targetBranch == "" {
		targetBranch = strings.TrimSpace(svc.Branch)
	}
	if targetBranch == "" {
		targetBranch = "main"
	}
	if branch != targetBranch {
		return fmt.Sprintf("Ignored push for branch '%s'; service tracks '%s'", branch, targetBranch)
	}
	if payload.Deleted || isZeroSHA(payload.After) {
		return "Ignored branch deletion"
	}
	if payload.Before != "" && payload.After != "" && strings.EqualFold(payload.Before, payload.After) {
		return "Ignored push without new commits"
	}
	if !svc.AutoDeploy {
		return "Automatic deployments are disabled"
	}
	if keyword, skip := skippedCommitMessage(payload); skip {
		return fmt.Sprintf("Ignored commit containing %s", keyword)
	}
	if !shouldDeployForWatchPaths(svc.WatchPaths, changedFiles(payload)) {
		return "Push did not touch the configured watch paths"
	}
	return ""
}

func writeWebhookIgnored(w http.ResponseWriter, message string) {
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"status": "ignored", "message": message})
}

func normalizeWebhookProvider(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "github":
		return "github"
	case "gitlab":
		return "gitlab"
	case "gitea":
		return "gitea"
	default:
		return ""
	}
}

func detectWebhookProvider(h http.Header) string {
	switch {
	case h.Get("X-Gitea-Signature") != "" || h.Get("X-Gitea-Event") != "" || h.Get("X-Gitea-Delivery") != "":
		return "gitea"
	case h.Get("X-Gitlab-Token") != "" || h.Get("X-Gitlab-Event") != "" || h.Get("X-Gitlab-Event-UUID") != "" || h.Get("Webhook-Signature") != "":
		return "gitlab"
	case h.Get("X-Hub-Signature-256") != "" || h.Get("X-GitHub-Event") != "" || h.Get("X-GitHub-Delivery") != "":
		return "github"
	default:
		return ""
	}
}

func verifyWebhookSignature(provider, secret string, headers http.Header, body []byte) error {
	if secret == "" {
		return nil
	}
	if provider == "" {
		provider = detectWebhookProvider(headers)
	}
	switch provider {
	case "github":
		return matchHMACSHA256(headers.Get("X-Hub-Signature-256"), "sha256=", secret, body)
	case "gitea":
		if sig := headers.Get("X-Gitea-Signature"); sig != "" {
			return matchHexHMAC(sig, secret, body)
		}
		return matchHMACSHA256(headers.Get("X-Hub-Signature-256"), "sha256=", secret, body)
	case "gitlab":
		if headers.Get("X-Gitlab-Token") == "" && headers.Get("Webhook-Signature") != "" {
			return errors.New("GitLab signing tokens are not supported; configure a Secret token")
		}
		return matchPlainSecret(headers.Get("X-Gitlab-Token"), secret)
	default:
		return fmt.Errorf("unsupported webhook provider %q", provider)
	}
}

func matchHMACSHA256(header, prefix, secret string, body []byte) error {
	if !strings.HasPrefix(header, prefix) {
		return errInvalidWebhookSignature
	}
	got, err := hex.DecodeString(strings.TrimPrefix(header, prefix))
	if err != nil {
		return errInvalidWebhookSignature
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	if !hmac.Equal(got, mac.Sum(nil)) {
		return errInvalidWebhookSignature
	}
	return nil
}

func matchHexHMAC(header, secret string, body []byte) error {
	got, err := hex.DecodeString(strings.TrimSpace(header))
	if err != nil {
		return errInvalidWebhookSignature
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	if !hmac.Equal(got, mac.Sum(nil)) {
		return errInvalidWebhookSignature
	}
	return nil
}

func matchPlainSecret(header, secret string) error {
	if subtle.ConstantTimeCompare([]byte(header), []byte(secret)) != 1 {
		return errInvalidWebhookSignature
	}
	return nil
}

func webhookEventName(provider string, h http.Header) (string, bool) {
	var raw string
	switch provider {
	case "github":
		raw = h.Get("X-GitHub-Event")
	case "gitlab":
		raw = h.Get("X-Gitlab-Event")
	case "gitea":
		raw = h.Get("X-Gitea-Event")
	default:
		raw = h.Get("X-GitHub-Event")
		if raw == "" {
			raw = h.Get("X-Gitlab-Event")
		}
		if raw == "" {
			raw = h.Get("X-Gitea-Event")
		}
	}
	raw = strings.ToLower(strings.TrimSpace(raw))
	return raw, raw != ""
}

func isPushEvent(provider, event string) bool {
	if provider == "gitlab" {
		return event == "push hook"
	}
	return event == "push"
}

func webhookDeliveryID(h http.Header) string {
	for _, name := range []string{
		"X-GitHub-Delivery",
		"X-Gitea-Delivery",
		"X-Gitlab-Event-UUID",
		"X-Gitlab-Webhook-UUID",
		"Idempotency-Key",
		"Webhook-Id",
	} {
		if value := strings.TrimSpace(h.Get(name)); value != "" {
			return name + ":" + value
		}
	}
	return ""
}

func isZeroSHA(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	for _, r := range value {
		if r != '0' {
			return false
		}
	}
	return true
}

func skippedCommitMessage(payload *WebhookPushPayload) (string, bool) {
	messages := make([]string, 0, len(payload.Commits)+1)
	if payload.HeadCommit != nil && payload.HeadCommit.Message != "" {
		messages = append(messages, payload.HeadCommit.Message)
	}
	for _, commit := range payload.Commits {
		if commit.Message != "" {
			messages = append(messages, commit.Message)
		}
	}
	for _, message := range messages {
		lower := strings.ToLower(message)
		for _, keyword := range webhookSkipKeywords {
			if strings.Contains(lower, keyword) {
				return keyword, true
			}
		}
	}
	return "", false
}

func changedFiles(payload *WebhookPushPayload) []string {
	var files []string
	for _, commit := range payload.Commits {
		files = append(files, commit.Added...)
		files = append(files, commit.Modified...)
		files = append(files, commit.Removed...)
	}
	return files
}

func shouldDeployForWatchPaths(raw string, files []string) bool {
	if len(files) == 0 {
		return true
	}
	patterns := splitWatchPaths(raw)
	if len(patterns) == 0 {
		return true
	}
	valid := 0
	for _, pattern := range patterns {
		re, err := compileGlob(pattern)
		if err != nil {
			continue
		}
		valid++
		for _, file := range files {
			if re.MatchString(strings.TrimPrefix(file, "./")) {
				return true
			}
		}
	}
	return valid == 0
}

func splitWatchPaths(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r'
	})
	patterns := make([]string, 0, len(fields))
	for _, field := range fields {
		if pattern := strings.TrimSpace(field); pattern != "" {
			patterns = append(patterns, pattern)
		}
	}
	return patterns
}

func compileGlob(pattern string) (*regexp.Regexp, error) {
	pattern = strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(pattern), "./"), "/")
	if pattern == "" {
		return nil, errors.New("empty pattern")
	}
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				if i+2 < len(pattern) && pattern[i+2] == '/' {
					b.WriteString("(?:.*/)?")
					i += 2
				} else {
					b.WriteString(".*")
					i++
				}
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(pattern[i])))
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}
