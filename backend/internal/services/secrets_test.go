package services

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"gopod/internal/db"
)

func newSecretsTestRepo(t *testing.T) (*WorkloadService, *SQLiteRepository, context.Context) {
	t.Helper()
	tempDB := filepath.Join(t.TempDir(), "secrets.db")
	database, err := db.Open(tempDB)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	repo := NewSQLiteRepository(database.DB)
	ctx := context.Background()
	if _, err := database.DB.Exec("INSERT INTO projects (id, name) VALUES ('proj-secrets', 'Secrets')"); err != nil {
		t.Fatalf("failed to insert project: %v", err)
	}
	return NewWorkloadService(repo, nil, nil), repo, ctx
}

// TestRedactServiceSerializationHasNoSecrets pins the API contract: a redacted
// service carries neither secret env values nor the webhook token, and reports
// token existence through hasWebhook.
func TestRedactServiceSerializationHasNoSecrets(t *testing.T) {
	svc := Service{
		ID:           "proj-app-1",
		Name:         "app",
		WebhookToken: "wh_supersecret",
		EnvVars: []EnvVar{
			{Key: "PUBLIC", Value: "keep-me", Secret: false},
			{Key: "PASSWORD", Value: "hunter2", Secret: true},
			{Key: "EMPTY_SECRET", Value: "", Secret: true},
		},
	}

	redacted := RedactService(svc)
	raw, err := json.Marshal(redacted)
	if err != nil {
		t.Fatalf("marshal redacted service: %v", err)
	}
	body := string(raw)

	if strings.Contains(body, "hunter2") {
		t.Fatalf("secret env value leaked in serialized service: %s", body)
	}
	if strings.Contains(body, "wh_supersecret") {
		t.Fatalf("webhook token leaked in serialized service: %s", body)
	}
	if !strings.Contains(body, "keep-me") {
		t.Fatalf("non-secret env value must stay readable: %s", body)
	}
	if !strings.Contains(body, EnvSecretMask) {
		t.Fatalf("expected secret mask in serialized service: %s", body)
	}
	if !redacted.HasWebhook {
		t.Fatalf("expected hasWebhook=true for a token-bearing service")
	}

	// The copied struct must not even hold the token.
	if redacted.WebhookToken != "" {
		t.Fatalf("redacted copy still carries the webhook token")
	}
	for _, v := range redacted.EnvVars {
		if v.Key == "EMPTY_SECRET" && v.Value != "" {
			t.Fatalf("empty secret must stay empty, got %q", v.Value)
		}
	}
}

// TestUpdateServiceKeepsMaskedSecretValue: the UI re-submits the whole env list
// on every save; a masked or empty secret value must keep the stored value
// (same contract as the "leave blank to keep" basic-auth password).
func TestUpdateServiceKeepsMaskedSecretValue(t *testing.T) {
	ws, repo, ctx := newSecretsTestRepo(t)

	created, err := ws.CreateService(ctx, Service{
		ID:        "proj-masked-1",
		ProjectID: "proj-secrets",
		Name:      "masked",
		Type:      "image",
		Image:     "nginx:alpine",
		EnvVars: []EnvVar{
			{Key: "DB_PASSWORD", Value: "real-secret", Secret: true},
			{Key: "PUBLIC", Value: "plain", Secret: false},
		},
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	// Round-trip the payload the UI would send: mask sentinel for the secret.
	payload := *created
	payload.EnvVars = []EnvVar{
		{Key: "DB_PASSWORD", Value: EnvSecretMask, Secret: true},
		{Key: "PUBLIC", Value: "plain-edited", Secret: false},
	}
	if err := ws.UpdateService(ctx, payload); err != nil {
		t.Fatalf("update service: %v", err)
	}

	stored, err := repo.Get(ctx, created.ID)
	if err != nil || stored == nil {
		t.Fatalf("get service: %v", err)
	}
	values := map[string]string{}
	for _, v := range stored.EnvVars {
		values[v.Key] = v.Value
	}
	if values["DB_PASSWORD"] != "real-secret" {
		t.Fatalf("masked secret was overwritten: %q", values["DB_PASSWORD"])
	}
	if values["PUBLIC"] != "plain-edited" {
		t.Fatalf("non-secret edit lost: %q", values["PUBLIC"])
	}

	// Empty value variant keeps the secret too.
	payload.EnvVars = []EnvVar{
		{Key: "DB_PASSWORD", Value: "", Secret: true},
		{Key: "PUBLIC", Value: "plain-edited", Secret: false},
	}
	if err := ws.UpdateService(ctx, payload); err != nil {
		t.Fatalf("update service (empty value): %v", err)
	}
	stored, _ = repo.Get(ctx, created.ID)
	for _, v := range stored.EnvVars {
		if v.Key == "DB_PASSWORD" && v.Value != "real-secret" {
			t.Fatalf("empty secret value must keep the stored one, got %q", v.Value)
		}
	}

	// A real new value replaces the stored one.
	payload.EnvVars = []EnvVar{
		{Key: "DB_PASSWORD", Value: "rotated-secret", Secret: true},
	}
	if err := ws.UpdateService(ctx, payload); err != nil {
		t.Fatalf("update service (rotate): %v", err)
	}
	stored, _ = repo.Get(ctx, created.ID)
	for _, v := range stored.EnvVars {
		if v.Key == "DB_PASSWORD" && v.Value != "rotated-secret" {
			t.Fatalf("expected rotated secret, got %q", v.Value)
		}
	}
}

// TestUpdateServiceKeepsMaskWhenSecretFlagDropped: the mask sentinel always
// means "keep the stored value", even when a lossy client forgets the secret
// flag — and such a client must not downgrade the stored var to non-secret.
func TestUpdateServiceKeepsMaskWhenSecretFlagDropped(t *testing.T) {
	ws, repo, ctx := newSecretsTestRepo(t)

	created, err := ws.CreateService(ctx, Service{
		ID:        "proj-lossy-1",
		ProjectID: "proj-secrets",
		Name:      "lossy",
		Type:      "image",
		Image:     "nginx:alpine",
		EnvVars: []EnvVar{
			{Key: "API_TOKEN", Value: "real-token", Secret: true},
		},
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	payload := *created
	payload.EnvVars = []EnvVar{
		{Key: "API_TOKEN", Value: EnvSecretMask, Secret: false},
	}
	if err := ws.UpdateService(ctx, payload); err != nil {
		t.Fatalf("update service: %v", err)
	}

	stored, err := repo.Get(ctx, created.ID)
	if err != nil || stored == nil {
		t.Fatalf("get service: %v", err)
	}
	for _, v := range stored.EnvVars {
		if v.Key != "API_TOKEN" {
			continue
		}
		if v.Value != "real-token" {
			t.Fatalf("mask without the secret flag overwrote the stored value: %q", v.Value)
		}
		if !v.Secret {
			t.Fatalf("stored secret must not be downgraded to a plain var")
		}
		return
	}
	t.Fatalf("API_TOKEN env var missing after update")
}

// TestUpdateServiceRefusesToWipeUndecryptableEnv: when the stored blob cannot
// be decrypted (master key replaced), an update that carries no env list must
// not blank the row — the ciphertext may still be recoverable.
func TestUpdateServiceRefusesToWipeUndecryptableEnv(t *testing.T) {
	ws, repo, ctx := newSecretsTestRepo(t)

	created, err := ws.CreateService(ctx, Service{
		ID:        "proj-corrupt-1",
		ProjectID: "proj-secrets",
		Name:      "corrupt",
		Type:      "image",
		Image:     "nginx:alpine",
		EnvVars:   []EnvVar{{Key: "SECRET", Value: "value", Secret: true}},
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	// Simulate a blob written with a different master key.
	if _, err := repo.db.ExecContext(ctx, "UPDATE services SET env_vars = 'enc:v1:AQIDBAUGBwgJCgsMDQ4PEA==' WHERE id = ?", created.ID); err != nil {
		t.Fatalf("craft undecryptable row: %v", err)
	}

	stored, err := repo.Get(ctx, created.ID)
	if err != nil || stored == nil {
		t.Fatalf("get service: %v", err)
	}
	if !stored.EnvDecryptionError {
		t.Fatalf("expected EnvDecryptionError to be surfaced")
	}

	payload := *created
	payload.EnvVars = nil
	if err := ws.UpdateService(ctx, payload); err == nil {
		t.Fatalf("expected update with empty env list to be refused")
	}

	// Editor noise (blank line entry) must not defeat the refusal either.
	payload.EnvVars = []EnvVar{{Key: "", Value: "", Secret: false}}
	if err := ws.UpdateService(ctx, payload); err == nil {
		t.Fatalf("expected update with blank-line-only env list to be refused")
	}

	// A stale client that still holds a masked value cannot be honoured: the
	// stored secret is unreadable, so persisting the literal mask would look
	// like a saved secret.
	payload.EnvVars = []EnvVar{{Key: "SECRET", Value: EnvSecretMask, Secret: true}}
	if err := ws.UpdateService(ctx, payload); err == nil {
		t.Fatalf("expected update carrying the mask sentinel to be refused")
	}

	// Same class of "keep" request: a secret-flagged value with no content.
	payload.EnvVars = []EnvVar{{Key: "SECRET", Value: "", Secret: true}}
	if err := ws.UpdateService(ctx, payload); err == nil {
		t.Fatalf("expected update with empty secret value to be refused")
	}

	// All refusals above must leave the ciphertext untouched.
	still, err := repo.Get(ctx, created.ID)
	if err != nil || still == nil {
		t.Fatalf("get service after refusals: %v", err)
	}
	if !still.EnvDecryptionError {
		t.Fatalf("expected stored blob to remain undecryptable after refused updates")
	}

	// Sending an explicit new list is allowed (deliberate replacement). The UI
	// round-trips envDecryptionError from GET, so the explicit write must clear
	// the read-side flag instead of preserving the old ciphertext.
	payload.EnvVars = []EnvVar{{Key: "SECRET", Value: "fresh", Secret: true}}
	payload.EnvDecryptionError = true
	if err := ws.UpdateService(ctx, payload); err != nil {
		t.Fatalf("explicit replacement should be allowed: %v", err)
	}
	replaced, err := repo.Get(ctx, created.ID)
	if err != nil || replaced == nil {
		t.Fatalf("get service after replacement: %v", err)
	}
	if replaced.EnvDecryptionError {
		t.Fatal("replacement should clear the decryption error")
	}
	if len(replaced.EnvVars) != 1 || replaced.EnvVars[0].Value != "fresh" {
		t.Fatalf("replacement was not persisted: %+v", replaced.EnvVars)
	}
}

// TestDeployServiceRefusesUndecryptableEnv: deploying with an unreadable env
// blob would silently drop authentication secrets, so the deployer must stop
// before touching the runtime.
func TestDeployServiceRefusesUndecryptableEnv(t *testing.T) {
	d := NewDeployer(nil, nil, nil, t.TempDir())
	_, err := d.DeployService(context.Background(), Service{
		ID:                 "svc-env",
		ProjectID:          "proj-env",
		Name:               "env-app",
		Type:               "image",
		Image:              "nginx:alpine",
		EnvDecryptionError: true,
	}, "dep-env")
	if err == nil || !strings.Contains(err.Error(), "environment variables cannot be decrypted") {
		t.Fatalf("expected a clear env decryption error, got %v", err)
	}
}

// TestRepositoryUpdatePreservesUndecryptableEnv: internal status-only updates
// must not replace an unreadable env blob with an empty list.
func TestRepositoryUpdatePreservesUndecryptableEnv(t *testing.T) {
	ws, repo, ctx := newSecretsTestRepo(t)

	created, err := ws.CreateService(ctx, Service{
		ID:        "proj-corrupt-repo-1",
		ProjectID: "proj-secrets",
		Name:      "corrupt-repo",
		Type:      "image",
		Image:     "nginx:alpine",
		EnvVars:   []EnvVar{{Key: "SECRET", Value: "value", Secret: true}},
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	if _, err := repo.db.ExecContext(ctx, "UPDATE services SET env_vars = 'enc:v1:AQIDBAUGBwgJCgsMDQ4PEA==' WHERE id = ?", created.ID); err != nil {
		t.Fatalf("craft undecryptable row: %v", err)
	}

	stored, err := repo.Get(ctx, created.ID)
	if err != nil || stored == nil {
		t.Fatalf("get service: %v", err)
	}
	if !stored.EnvDecryptionError {
		t.Fatal("expected EnvDecryptionError")
	}
	var before string
	if err := repo.db.QueryRowContext(ctx, "SELECT env_vars FROM services WHERE id = ?", created.ID).Scan(&before); err != nil {
		t.Fatalf("read env ciphertext: %v", err)
	}

	stored.Status = "running"
	if err := repo.Update(ctx, *stored); err != nil {
		t.Fatalf("status update: %v", err)
	}

	var after string
	if err := repo.db.QueryRowContext(ctx, "SELECT env_vars FROM services WHERE id = ?", created.ID).Scan(&after); err != nil {
		t.Fatalf("read env ciphertext after update: %v", err)
	}
	if after != before {
		t.Fatal("status-only update replaced the undecryptable env ciphertext")
	}
}

// TestGetServiceLazyWebhookTokenPreservesUndecryptableEnv: the lazy token
// backfill must use the targeted setter, not a full row update.
func TestGetServiceLazyWebhookTokenPreservesUndecryptableEnv(t *testing.T) {
	ws, repo, ctx := newSecretsTestRepo(t)

	created, err := ws.CreateService(ctx, Service{
		ID:        "proj-corrupt-token-1",
		ProjectID: "proj-secrets",
		Name:      "corrupt-token",
		Type:      "image",
		Image:     "nginx:alpine",
		EnvVars:   []EnvVar{{Key: "SECRET", Value: "value", Secret: true}},
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	if _, err := repo.db.ExecContext(ctx, "UPDATE services SET env_vars = 'enc:v1:AQIDBAUGBwgJCgsMDQ4PEA==', webhook_token = '' WHERE id = ?", created.ID); err != nil {
		t.Fatalf("craft undecryptable row: %v", err)
	}
	var before string
	if err := repo.db.QueryRowContext(ctx, "SELECT env_vars FROM services WHERE id = ?", created.ID).Scan(&before); err != nil {
		t.Fatalf("read env ciphertext: %v", err)
	}

	got, err := ws.GetService(ctx, created.ID)
	if err != nil || got == nil {
		t.Fatalf("get service: %v", err)
	}
	if got.WebhookToken == "" {
		t.Fatal("expected lazy webhook token generation")
	}
	var after string
	if err := repo.db.QueryRowContext(ctx, "SELECT env_vars FROM services WHERE id = ?", created.ID).Scan(&after); err != nil {
		t.Fatalf("read env ciphertext after get: %v", err)
	}
	if after != before {
		t.Fatal("lazy webhook token generation replaced the undecryptable env ciphertext")
	}
}

func TestRevealEnvValuesAndRotateWebhook(t *testing.T) {
	ws, repo, ctx := newSecretsTestRepo(t)

	created, err := ws.CreateService(ctx, Service{
		ID:        "proj-reveal-1",
		ProjectID: "proj-secrets",
		Name:      "reveal",
		Type:      "image",
		Image:     "nginx:alpine",
		EnvVars: []EnvVar{
			{Key: "TOKEN", Value: "token-value", Secret: true},
			{Key: "PLAIN", Value: "plain-value", Secret: false},
		},
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	values, err := ws.RevealEnvValues(ctx, created.ID, []string{"TOKEN", "MISSING"})
	if err != nil {
		t.Fatalf("reveal env: %v", err)
	}
	if values["TOKEN"] != "token-value" {
		t.Fatalf("reveal returned %q", values["TOKEN"])
	}
	if _, ok := values["MISSING"]; ok {
		t.Fatalf("reveal must not invent values for unknown keys")
	}

	// Webhook token: reveal generates one on demand.
	first, err := ws.WebhookTokenFor(ctx, created.ID)
	if err != nil {
		t.Fatalf("reveal webhook: %v", err)
	}
	if first == "" {
		t.Fatalf("expected a generated webhook token")
	}
	again, _ := ws.WebhookTokenFor(ctx, created.ID)
	if again != first {
		t.Fatalf("reveal must be stable until rotation")
	}

	rotated, err := ws.RotateWebhookToken(ctx, created.ID)
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if rotated == first || rotated == "" {
		t.Fatalf("rotation must issue a fresh token, got %q", rotated)
	}
	if _, err := repo.GetByWebhookToken(ctx, first); err != nil {
		t.Fatalf("legacy lookup error: %v", err)
	}
	if old, _ := repo.GetByWebhookToken(ctx, first); old != nil {
		t.Fatalf("old token must stop resolving after rotation")
	}
	matched, err := repo.GetByWebhookToken(ctx, rotated)
	if err != nil || matched == nil || matched.ID != created.ID {
		t.Fatalf("new token must resolve to the service (got %+v, err %v)", matched, err)
	}
}

// TestGetServiceByWebhookTokenRejectsServiceID pins that a service ID is not
// accepted as a webhook token.
func TestGetServiceByWebhookTokenRejectsServiceID(t *testing.T) {
	ws, _, ctx := newSecretsTestRepo(t)

	created, err := ws.CreateService(ctx, Service{
		ID:        "proj-idfallback-1",
		ProjectID: "proj-secrets",
		Name:      "idfallback",
		Type:      "image",
		Image:     "nginx:alpine",
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	got, err := ws.GetServiceByWebhookToken(ctx, created.ID)
	if err != nil {
		t.Fatalf("lookup error: %v", err)
	}
	if got != nil {
		t.Fatalf("the service ID must not be accepted as a webhook token")
	}
}

// TestCreateServiceIDEntropy: the ID suffix went from 3 to 8 random bytes.
func TestCreateServiceIDEntropy(t *testing.T) {
	ws, _, ctx := newSecretsTestRepo(t)

	created, err := ws.CreateService(ctx, Service{
		ProjectID: "proj-secrets",
		Name:      "entropy",
		Type:      "image",
		Image:     "nginx:alpine",
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	parts := strings.Split(created.ID, "-")
	suffix := parts[len(parts)-1]
	if len(suffix) != 16 {
		t.Fatalf("expected 16 hex chars of entropy in ID %q, got suffix %q", created.ID, suffix)
	}
}
