package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"testing"
	"time"
)

func TestVerifyWebhookSignatureGitHubVector(t *testing.T) {
	const secret = "It's a Secret to Everybody"
	body := []byte("Hello, World!")
	const signature = "sha256=757107ea0eb2509fc211221cce984b8a37570b6d7586c22c46f4379c8b043e17"

	headers := http.Header{}
	headers.Set("X-Hub-Signature-256", signature)
	if err := verifyWebhookSignature("github", secret, headers, body); err != nil {
		t.Fatalf("valid GitHub signature rejected: %v", err)
	}
	tampered := []byte("Hello, World?")
	if err := verifyWebhookSignature("github", secret, headers, tampered); err == nil {
		t.Fatal("tampered body accepted")
	}
	headers.Set("X-Hub-Signature-256", "sha256=deadbeef")
	if err := verifyWebhookSignature("github", secret, headers, body); err == nil {
		t.Fatal("invalid signature accepted")
	}
	if err := verifyWebhookSignature("github", secret, http.Header{}, body); err == nil {
		t.Fatal("missing signature accepted")
	}
}

func TestVerifyWebhookSignatureGitea(t *testing.T) {
	secret := "gitea-secret"
	body := []byte(`{"ref":"refs/heads/main"}`)

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	headers := http.Header{}
	headers.Set("X-Gitea-Signature", hex.EncodeToString(mac.Sum(nil)))

	if err := verifyWebhookSignature("gitea", secret, headers, body); err != nil {
		t.Fatalf("valid Gitea signature rejected: %v", err)
	}
	if err := verifyWebhookSignature("gitea", "wrong", headers, body); err == nil {
		t.Fatal("Gitea signature with the wrong secret accepted")
	}
	headers.Set("X-Gitea-Signature", "not-hex")
	if err := verifyWebhookSignature("gitea", secret, headers, body); err == nil {
		t.Fatal("malformed Gitea signature accepted")
	}
}

func TestVerifyWebhookSignatureGitLab(t *testing.T) {
	body := []byte(`{"ref":"refs/heads/main"}`)
	headers := http.Header{}
	headers.Set("X-Gitlab-Token", "legacy-secret")

	if err := verifyWebhookSignature("gitlab", "legacy-secret", headers, body); err != nil {
		t.Fatalf("valid GitLab token rejected: %v", err)
	}
	if err := verifyWebhookSignature("gitlab", "other", headers, body); err == nil {
		t.Fatal("GitLab token mismatch accepted")
	}

	headers.Del("X-Gitlab-Token")
	headers.Set("Webhook-Signature", "v1,abc")
	if err := verifyWebhookSignature("gitlab", "legacy-secret", headers, body); err == nil {
		t.Fatal("GitLab signing token accepted without support")
	}
}

func TestVerifyWebhookSignatureNoSecretKeepsLegacy(t *testing.T) {
	if err := verifyWebhookSignature("github", "", http.Header{}, []byte("{}")); err != nil {
		t.Fatalf("legacy webhook must remain accepted: %v", err)
	}
}

func TestDetectWebhookProvider(t *testing.T) {
	cases := []struct {
		header string
		value  string
		want   string
	}{
		{"X-GitHub-Delivery", "abc", "github"},
		{"X-Gitea-Event", "push", "gitea"},
		{"X-Gitlab-Event", "Push Hook", "gitlab"},
	}
	for _, tc := range cases {
		headers := http.Header{}
		headers.Set(tc.header, tc.value)
		if got := detectWebhookProvider(headers); got != tc.want {
			t.Errorf("detectWebhookProvider(%s) = %q, want %q", tc.header, got, tc.want)
		}
	}
	if got := detectWebhookProvider(http.Header{}); got != "" {
		t.Fatalf("expected no provider, got %q", got)
	}
}

func TestWebhookEventFiltering(t *testing.T) {
	github := http.Header{}
	github.Set("X-GitHub-Event", "ping")
	event, ok := webhookEventName("github", github)
	if !ok || isPushEvent("github", event) {
		t.Fatalf("ping must not count as a push: event=%q", event)
	}

	gitlab := http.Header{}
	gitlab.Set("X-Gitlab-Event", "Push Hook")
	event, ok = webhookEventName("gitlab", gitlab)
	if !ok || !isPushEvent("gitlab", event) {
		t.Fatalf("GitLab Push Hook must count as a push: event=%q", event)
	}

	gitea := http.Header{}
	gitea.Set("X-Gitea-Event", "issues")
	event, ok = webhookEventName("gitea", gitea)
	if !ok || isPushEvent("gitea", event) {
		t.Fatalf("issues event must not count as a push: event=%q", event)
	}

	event, ok = webhookEventName("github", http.Header{})
	if ok || event != "" {
		t.Fatalf("expected no event header, got %q", event)
	}
}

func TestWebhookPushSkipReason(t *testing.T) {
	sha := "0123456789abcdef0123456789abcdef01234567"
	other := "fedcba9876543210fedcba9876543210fedcba98"
	payload := &WebhookPushPayload{
		Ref:    "refs/heads/main",
		Before: sha,
		After:  other,
	}
	svc := &Service{AutoDeploy: true}

	if reason := webhookPushSkipReason(svc, payload); reason != "" {
		t.Fatalf("matching push skipped: %s", reason)
	}

	branchMismatch := *payload
	branchMismatch.Ref = "refs/heads/dev"
	if reason := webhookPushSkipReason(svc, &branchMismatch); reason == "" {
		t.Fatal("branch mismatch must be skipped")
	}

	deleted := *payload
	deleted.After = "0000000000000000000000000000000000000000"
	if reason := webhookPushSkipReason(svc, &deleted); reason == "" {
		t.Fatal("branch deletion must be skipped")
	}

	noop := *payload
	noop.After = noop.Before
	if reason := webhookPushSkipReason(svc, &noop); reason == "" {
		t.Fatal("no-op push must be skipped")
	}

	paused := *svc
	paused.AutoDeploy = false
	if reason := webhookPushSkipReason(&paused, payload); reason == "" {
		t.Fatal("auto-deploy disabled must be skipped")
	}

	skip := *payload
	skip.HeadCommit = &WebhookCommit{Message: "docs: update [skip ci]"}
	if reason := webhookPushSkipReason(svc, &skip); reason == "" {
		t.Fatal("skip-ci commit must be skipped")
	}

	watch := *svc
	watch.WatchPaths = "src/**"
	apiOnly := *payload
	apiOnly.Commits = []WebhookCommit{{Modified: []string{"docs/readme.md"}}}
	if reason := webhookPushSkipReason(&watch, &apiOnly); reason == "" {
		t.Fatal("push outside watch paths must be skipped")
	}
	src := *payload
	src.Commits = []WebhookCommit{{Modified: []string{"src/main.go"}}}
	if reason := webhookPushSkipReason(&watch, &src); reason != "" {
		t.Fatalf("push inside watch paths skipped: %s", reason)
	}
}

func TestShouldDeployForWatchPaths(t *testing.T) {
	cases := []struct {
		patterns string
		files    []string
		want     bool
	}{
		{"", []string{"anything.go"}, true},
		{"src/**, packages/app/**", []string{"src/main.go"}, true},
		{"src/**, packages/app/**", []string{"packages/app/index.ts"}, true},
		{"src/**, packages/app/**", []string{"README.md"}, false},
		{"src/**, packages/app/**", nil, true},
		{"**/*.md", []string{"README.md"}, true},
		{"*.md", []string{"docs/README.md"}, false},
		{"src/**", []string{"src/generated/deep/file.go"}, true},
	}
	for _, tc := range cases {
		if got := shouldDeployForWatchPaths(tc.patterns, tc.files); got != tc.want {
			t.Errorf("shouldDeployForWatchPaths(%q, %v) = %v, want %v", tc.patterns, tc.files, got, tc.want)
		}
	}
}

func TestWebhookDeduper(t *testing.T) {
	d := newWebhookDeduper()
	if !d.firstSeen("token|delivery-1") {
		t.Fatal("first delivery rejected")
	}
	if d.firstSeen("token|delivery-1") {
		t.Fatal("duplicate delivery accepted")
	}
	if !d.firstSeen("token|delivery-2") {
		t.Fatal("second delivery rejected")
	}
	d.ttl = time.Nanosecond
	time.Sleep(time.Millisecond)
	if !d.firstSeen("token|delivery-1") {
		t.Fatal("expired delivery not accepted")
	}
}

func TestWebhookDeliveryID(t *testing.T) {
	headers := http.Header{}
	headers.Set("X-GitHub-Delivery", "abc-123")
	if got := webhookDeliveryID(headers); got != "X-GitHub-Delivery:abc-123" {
		t.Fatalf("unexpected delivery id: %q", got)
	}
	headers = http.Header{}
	headers.Set("X-Gitea-Delivery", "gitea-1")
	if got := webhookDeliveryID(headers); got != "X-Gitea-Delivery:gitea-1" {
		t.Fatalf("unexpected delivery id: %q", got)
	}
	if got := webhookDeliveryID(http.Header{}); got != "" {
		t.Fatalf("expected empty delivery id, got %q", got)
	}
}
