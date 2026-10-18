package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gopod/internal/services"
	"gopod/internal/testutil"
)

// setupAuthedServer boots the full server on a temp DB and completes the
// first-run setup, returning the session token.
func setupAuthedServer(t *testing.T) (http.Handler, string) {
	t.Helper()
	handler, _ := setupTestServer(t)

	setupPayload := map[string]string{
		"name":     "Cluster Administrator",
		"email":    "admin@gopod.internal",
		"password": "MasterAdminPass123!",
	}
	body, _ := json.Marshal(setupPayload)
	req := httptest.NewRequest("POST", "/api/auth/setup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
		t.Fatalf("setup failed: %d %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil || resp.Token == "" {
		t.Fatalf("setup token missing: %v", err)
	}
	return handler, resp.Token
}

func doAuthedJSON(t *testing.T, handler http.Handler, method, path string, body []byte, token string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := testutil.NewAuthRequest(method, path, reader, token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// TestSecretsRedactionEndToEnd walks the real HTTP surface: secret env values
// and webhook tokens must never appear in list/get/create/update/project
// responses, while the explicit reveal/rotate endpoints hand them out.
func TestSecretsRedactionEndToEnd(t *testing.T) {
	handler, token := setupAuthedServer(t)

	// Project
	projBody, _ := json.Marshal(map[string]string{"name": "Redaction Project"})
	rec := doAuthedJSON(t, handler, "POST", "/api/projects", projBody, token)
	if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
		t.Fatalf("create project: %d %s", rec.Code, rec.Body.String())
	}
	var proj struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&proj); err != nil || proj.ID == "" {
		t.Fatalf("decode project: %v", err)
	}

	// Service with one secret and one plain env var.
	svcBody, _ := json.Marshal(map[string]interface{}{
		"projectId": proj.ID,
		"name":      "secrets-app",
		"type":      "image",
		"image":     "nginx:alpine",
		"envVars": []map[string]interface{}{
			{"key": "PUBLIC", "value": "plain-value", "secret": false},
			{"key": "PASSWORD", "value": "super-secret-value", "secret": true},
		},
	})
	rec = doAuthedJSON(t, handler, "POST", "/api/services", svcBody, token)
	if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
		t.Fatalf("create service: %d %s", rec.Code, rec.Body.String())
	}
	createRaw := rec.Body.String()
	if strings.Contains(createRaw, "super-secret-value") {
		t.Fatalf("create response leaked the secret value: %s", createRaw)
	}
	if strings.Contains(createRaw, `"webhookToken"`) {
		t.Fatalf("create response leaked the webhook token: %s", createRaw)
	}
	var created struct {
		ID         string `json:"id"`
		HasWebhook bool   `json:"hasWebhook"`
		EnvVars    []struct {
			Key    string `json:"key"`
			Value  string `json:"value"`
			Secret bool   `json:"secret"`
		} `json:"envVars"`
	}
	if err := json.NewDecoder(strings.NewReader(createRaw)).Decode(&created); err != nil || created.ID == "" {
		t.Fatalf("decode created service: %v", err)
	}
	if !created.HasWebhook {
		t.Fatalf("expected hasWebhook=true in create response")
	}

	for _, v := range created.EnvVars {
		if v.Key == "PASSWORD" && v.Value != services.EnvSecretMask {
			t.Fatalf("expected masked secret in create response, got %q", v.Value)
		}
		if v.Key == "PUBLIC" && v.Value != "plain-value" {
			t.Fatalf("non-secret value must stay readable, got %q", v.Value)
		}
	}

	// GET single + list keep hiding secrets.
	rec = doAuthedJSON(t, handler, "GET", "/api/services/"+created.ID, nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("get service: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "super-secret-value") {
		t.Fatalf("get response leaked the secret value")
	}

	rec = doAuthedJSON(t, handler, "GET", "/api/services", nil, token)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "super-secret-value") {
		t.Fatalf("list response leaked the secret value (code %d)", rec.Code)
	}

	// Projects embed services: same redaction must apply there.
	rec = doAuthedJSON(t, handler, "GET", "/api/projects", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("list projects: %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "super-secret-value") {
		t.Fatalf("project response leaked the embedded service secret")
	}

	// Explicit reveal returns the real value.
	revealBody, _ := json.Marshal(map[string]interface{}{"keys": []string{"PASSWORD"}})
	rec = doAuthedJSON(t, handler, "POST", "/api/services/"+created.ID+"/env/reveal", revealBody, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("reveal env: %d %s", rec.Code, rec.Body.String())
	}
	var revealResp struct {
		Values map[string]string `json:"values"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&revealResp); err != nil {
		t.Fatalf("decode reveal response: %v", err)
	}
	if revealResp.Values["PASSWORD"] != "super-secret-value" {
		t.Fatalf("reveal did not return the real secret: %+v", revealResp.Values)
	}

	// Webhook token: reveal, rotate, old token stops working.
	rec = doAuthedJSON(t, handler, "GET", "/api/services/"+created.ID+"/webhook", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("reveal webhook: %d %s", rec.Code, rec.Body.String())
	}
	var whResp struct {
		WebhookToken string `json:"webhookToken"`
		HasWebhook   bool   `json:"hasWebhook"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&whResp); err != nil || !whResp.HasWebhook || whResp.WebhookToken == "" {
		t.Fatalf("webhook reveal failed: %+v err=%v", whResp, err)
	}
	oldToken := whResp.WebhookToken

	rec = doAuthedJSON(t, handler, "POST", "/api/services/"+created.ID+"/webhook/rotate", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("rotate webhook: %d %s", rec.Code, rec.Body.String())
	}
	var rotated struct {
		WebhookToken string `json:"webhookToken"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&rotated); err != nil || rotated.WebhookToken == "" {
		t.Fatalf("rotate response missing token: %v", err)
	}
	if rotated.WebhookToken == oldToken {
		t.Fatalf("rotation returned the same token")
	}

	// Any push branch other than the tracked one short-circuits before deploy,
	// so this exercises token validation without touching podman.
	pushBody, _ := json.Marshal(map[string]string{"ref": "refs/heads/other-branch"})
	req := httptest.NewRequest("POST", "/api/deploy/webhook/"+rotated.WebhookToken, bytes.NewReader(pushBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("valid rotated token must be accepted: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "ignored") {
		t.Fatalf("expected ignored-branch response, got %s", rec.Body.String())
	}

	req = httptest.NewRequest("POST", "/api/deploy/webhook/"+oldToken, bytes.NewReader(pushBody))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("old webhook token must be rejected after rotation, got %d", rec.Code)
	}

	// The service ID must not work as a webhook token.
	req = httptest.NewRequest("POST", "/api/deploy/webhook/"+created.ID, bytes.NewReader(pushBody))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("service ID must not authenticate as a webhook token, got %d", rec.Code)
	}

	// UI roundtrip: PUT back the redacted object; the masked secret must be
	// preserved (keep-by-key), not overwritten by the sentinel.
	rec = doAuthedJSON(t, handler, "GET", "/api/services/"+created.ID, nil, token)
	var roundtrip map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&roundtrip); err != nil {
		t.Fatalf("decode service for roundtrip: %v", err)
	}
	roundtripBody, _ := json.Marshal(roundtrip)
	rec = doAuthedJSON(t, handler, "PUT", "/api/services/"+created.ID, roundtripBody, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("update roundtrip: %d %s", rec.Code, rec.Body.String())
	}
	rec = doAuthedJSON(t, handler, "POST", "/api/services/"+created.ID+"/env/reveal", revealBody, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("reveal after roundtrip: %d", rec.Code)
	}
	var afterRoundtrip struct {
		Values map[string]string `json:"values"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&afterRoundtrip); err != nil {
		t.Fatalf("decode reveal response: %v", err)
	}
	if afterRoundtrip.Values["PASSWORD"] != "super-secret-value" {
		t.Fatalf("redacted roundtrip destroyed the secret: %+v", afterRoundtrip.Values)
	}

	// Oversized JSON body is rejected with 413.
	bigBody, _ := json.Marshal(map[string]string{"name": strings.Repeat("x", 1<<20+64)})
	rec = doAuthedJSON(t, handler, "POST", "/api/projects", bigBody, token)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized body must map to 413, got %d", rec.Code)
	}

	// Deployment log ids with path characters are rejected before touching disk.
	rec = doAuthedJSON(t, handler, "GET", "/api/deployments/dep.1.log/logs", nil, token)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid deployment id must map to 400, got %d", rec.Code)
	}
}
