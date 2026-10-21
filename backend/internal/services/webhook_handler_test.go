package services

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"gopod/internal/db"
)

func newWebhookTestHandler(t *testing.T, services ...Service) *Handler {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "webhook.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if _, err := database.DB.Exec(`INSERT INTO projects (id, name) VALUES ('p1', 'project')`); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	repo := NewSQLiteRepository(database.DB)
	for _, svc := range services {
		if err := repo.Create(context.Background(), svc); err != nil {
			t.Fatalf("create service %s: %v", svc.ID, err)
		}
	}
	return NewHandler(NewWorkloadService(repo, nil, nil), nil)
}

func webhookTestService(id, token string) Service {
	return Service{
		ID:           id,
		ProjectID:    "p1",
		Name:         id,
		Type:         "application",
		SourceType:   "git",
		Source:       "https://example.test/repo.git",
		GitBranch:    "main",
		WebhookToken: token,
		AutoDeploy:   true,
	}
}

func doWebhookRequest(t *testing.T, h *Handler, token, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/deploy/webhook/"+token, strings.NewReader(body))
	req.SetPathValue("token", token)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	h.handleWebhook(rec, req)
	return rec
}

func TestWebhookHandlerRejectsMalformedJSON(t *testing.T) {
	svc := webhookTestService("svc-malformed", "wh_malformed")
	h := newWebhookTestHandler(t, svc)
	rec := doWebhookRequest(t, h, svc.WebhookToken, `{"ref":`, map[string]string{"X-GitHub-Event": "push"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed JSON: got %d %s", rec.Code, rec.Body.String())
	}
}

func TestWebhookHandlerIgnoresNonPushPingAndPausedDelivery(t *testing.T) {
	svc := webhookTestService("svc-paused", "wh_paused")
	svc.AutoDeploy = false
	h := newWebhookTestHandler(t, svc)

	body := `{"ref":"refs/heads/main","before":"1111111111111111111111111111111111111111","after":"2222222222222222222222222222222222222222"}`

	rec := doWebhookRequest(t, h, svc.WebhookToken, body, map[string]string{"X-GitHub-Event": "issues"})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "only push events") {
		t.Fatalf("issues event: got %d %s", rec.Code, rec.Body.String())
	}

	rec = doWebhookRequest(t, h, svc.WebhookToken, `{"zen":"hi"}`, map[string]string{"X-GitHub-Event": "ping"})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Webhook is active") {
		t.Fatalf("ping event: got %d %s", rec.Code, rec.Body.String())
	}

	rec = doWebhookRequest(t, h, svc.WebhookToken, body, map[string]string{"X-GitHub-Event": "push"})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "disabled") {
		t.Fatalf("paused service: got %d %s", rec.Code, rec.Body.String())
	}
}

func TestWebhookHandlerFiltersRefsAndWatchPaths(t *testing.T) {
	svc := webhookTestService("svc-watch", "wh_watch")
	svc.WatchPaths = "src/**"
	h := newWebhookTestHandler(t, svc)

	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "branch mismatch",
			body: `{"ref":"refs/heads/dev","before":"1111111111111111111111111111111111111111","after":"2222222222222222222222222222222222222222"}`,
			want: "tracks 'main'",
		},
		{
			name: "deletion",
			body: `{"ref":"refs/heads/main","deleted":true,"before":"1111111111111111111111111111111111111111","after":"0000000000000000000000000000000000000000"}`,
			want: "deletion",
		},
		{
			name: "no-op",
			body: `{"ref":"refs/heads/main","before":"1111111111111111111111111111111111111111","after":"1111111111111111111111111111111111111111"}`,
			want: "without new commits",
		},
		{
			name: "outside watch paths",
			body: `{"ref":"refs/heads/main","before":"1111111111111111111111111111111111111111","after":"2222222222222222222222222222222222222222","commits":[{"modified":["docs/readme.md"]}]}`,
			want: "watch paths",
		},
	}
	for _, tc := range cases {
		rec := doWebhookRequest(t, h, svc.WebhookToken, tc.body, map[string]string{"X-GitHub-Event": "push"})
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), tc.want) {
			t.Fatalf("%s: got %d %s", tc.name, rec.Code, rec.Body.String())
		}
	}
}

func TestWebhookHandlerRequiresSignatureWhenSecretConfigured(t *testing.T) {
	svc := webhookTestService("svc-signed", "wh_signed")
	svc.WebhookProvider = "github"
	svc.WebhookSecret = "webhook-secret"
	svc.AutoDeploy = false
	h := newWebhookTestHandler(t, svc)

	body := `{"ref":"refs/heads/main","before":"1111111111111111111111111111111111111111","after":"2222222222222222222222222222222222222222"}`
	headers := map[string]string{"X-GitHub-Event": "push"}

	rec := doWebhookRequest(t, h, svc.WebhookToken, body, headers)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unsigned delivery: got %d %s", rec.Code, rec.Body.String())
	}

	headers["X-Hub-Signature-256"] = "sha256=deadbeef"
	rec = doWebhookRequest(t, h, svc.WebhookToken, body, headers)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("invalid signature: got %d %s", rec.Code, rec.Body.String())
	}

	headers["X-Hub-Signature-256"] = githubSignatureForTest("webhook-secret", []byte(body))
	rec = doWebhookRequest(t, h, svc.WebhookToken, body, headers)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "disabled") {
		t.Fatalf("valid signature: got %d %s", rec.Code, rec.Body.String())
	}
}

func TestWebhookHandlerRejectsUndecryptableSecret(t *testing.T) {
	svc := webhookTestService("svc-bad-secret", "wh_bad_secret")
	svc.WebhookProvider = "github"
	svc.WebhookSecret = "webhook-secret"
	h := newWebhookTestHandler(t, svc)

	repo, ok := h.service.repo.(*SQLiteRepository)
	if !ok {
		t.Fatal("expected SQLite repository")
	}
	if _, err := repo.db.Exec(`UPDATE services SET webhook_secret = 'enc:v1:not-hex' WHERE id = ?`, svc.ID); err != nil {
		t.Fatalf("corrupt webhook secret: %v", err)
	}

	body := `{"ref":"refs/heads/main","before":"1111111111111111111111111111111111111111","after":"2222222222222222222222222222222222222222"}`
	rec := doWebhookRequest(t, h, svc.WebhookToken, body, map[string]string{"X-GitHub-Event": "push"})
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("undecryptable secret must fail closed: got %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(strings.ToLower(rec.Body.String()), "secret") {
		t.Fatalf("expected a secret-related error, got %s", rec.Body.String())
	}
}

func TestRedactServicePreservesWebhookFlags(t *testing.T) {
	got := RedactService(Service{HasWebhook: true, HasWebhookSecret: true})
	if !got.HasWebhook || !got.HasWebhookSecret {
		t.Fatalf("resolved webhook flags were dropped: %+v", got)
	}
	if got.WebhookToken != "" || got.WebhookSecret != "" {
		t.Fatalf("redaction leaked webhook material: %+v", got)
	}

	got = RedactService(Service{WebhookSecretError: true})
	if !got.HasWebhookSecret {
		t.Fatal("an unreadable secret must still be reported as configured")
	}
}

func githubSignatureForTest(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
