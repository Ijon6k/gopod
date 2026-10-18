package ingress

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// fakeRepo is an in-memory Repository for service-level tests.
type fakeRepo struct {
	mu      sync.Mutex
	domains map[string]Domain
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{domains: map[string]Domain{}}
}

func (f *fakeRepo) List(ctx context.Context, projectID string) ([]Domain, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Domain
	for _, d := range f.domains {
		if projectID == "" || d.ProjectID == projectID {
			out = append(out, d)
		}
	}
	return out, nil
}

func (f *fakeRepo) Get(ctx context.Context, id string) (*Domain, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	d, ok := f.domains[id]
	if !ok {
		return nil, nil
	}
	return &d, nil
}

func (f *fakeRepo) Create(ctx context.Context, d Domain) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.domains[d.ID] = d
	return nil
}

func (f *fakeRepo) Update(ctx context.Context, d Domain) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.domains[d.ID] = d
	return nil
}

func (f *fakeRepo) Delete(ctx context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.domains, id)
	return nil
}

func (f *fakeRepo) stored(t *testing.T, id string) Domain {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.domains[id]
}

func newTestDomain() Domain {
	return Domain{
		Hostname:      "app.example.com",
		ProjectID:     "proj-1",
		ServiceID:     "web-gui",
		ContainerPort: 8080,
		TLS:           true,
		BasicAuth:     true,
		BasicAuthUser: "admin",
		BasicAuthPass: "s3cret",
	}
}

func TestCreateDomainHashesBasicAuthPassword(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, nil)

	created, err := svc.CreateDomain(context.Background(), newTestDomain())
	if err != nil {
		t.Fatalf("CreateDomain: %v", err)
	}

	stored := repo.stored(t, created.ID)
	if !strings.HasPrefix(stored.BasicAuthPass, "$2") {
		t.Fatalf("stored password must be a bcrypt hash, got %q", stored.BasicAuthPass)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(stored.BasicAuthPass), []byte("s3cret")); err != nil {
		t.Fatalf("stored hash does not match the original password: %v", err)
	}
	if !created.HasBasicAuth {
		t.Fatalf("created domain must report hasBasicAuth=true")
	}

	raw, err := json.Marshal(created)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), "s3cret") || strings.Contains(string(raw), "basicAuthPass") {
		t.Fatalf("API JSON must never contain the password or its field, got %s", raw)
	}
	if !strings.Contains(string(raw), `"hasBasicAuth":true`) {
		t.Fatalf("API JSON must expose hasBasicAuth, got %s", raw)
	}
}

func TestCreateDomainRejectsPoisonedPayloads(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, nil)

	cases := map[string]Domain{
		"injection target": func() Domain { d := newTestDomain(); d.TargetContainer = "foo}\nroute /pwn { * }"; return d }(),
		"injection svcid":  func() Domain { d := newTestDomain(); d.ServiceID = "x; tls off"; return d }(),
		"bad port":         func() Domain { d := newTestDomain(); d.ContainerPort = 0; return d }(),
		"bad hostname":     func() Domain { d := newTestDomain(); d.Hostname = "foo bar.example.com"; return d }(),
	}

	for name, payload := range cases {
		if _, err := svc.CreateDomain(context.Background(), payload); !errors.Is(err, ErrValidation) {
			t.Errorf("%s: expected ErrValidation, got %v", name, err)
		}
	}
	if len(repo.domains) != 0 {
		t.Fatalf("no domain may be stored for rejected payloads, got %d", len(repo.domains))
	}
}

func TestUpdateDomainKeepsPasswordWhenEmpty(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, nil)

	created, err := svc.CreateDomain(context.Background(), newTestDomain())
	if err != nil {
		t.Fatalf("CreateDomain: %v", err)
	}
	originalHash := repo.stored(t, created.ID).BasicAuthPass

	// The UI edit payload carries the full object but never re-sends the password.
	payload := *created
	payload.BasicAuthPass = ""
	payload.PathPrefix = "/api"

	updated, err := svc.UpdateDomain(context.Background(), payload)
	if err != nil {
		t.Fatalf("UpdateDomain: %v", err)
	}
	if got := repo.stored(t, created.ID).BasicAuthPass; got != originalHash {
		t.Fatalf("empty password must keep the stored hash, got %q want %q", got, originalHash)
	}
	if !updated.HasBasicAuth {
		t.Fatalf("updated domain must still report hasBasicAuth=true")
	}
	if updated.PathPrefix != "/api" {
		t.Fatalf("update must persist other fields, got pathPrefix %q", updated.PathPrefix)
	}
}

func TestUpdateDomainHashesNewPassword(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, nil)

	created, err := svc.CreateDomain(context.Background(), newTestDomain())
	if err != nil {
		t.Fatalf("CreateDomain: %v", err)
	}
	originalHash := repo.stored(t, created.ID).BasicAuthPass

	payload := *created
	payload.BasicAuthPass = "second-secret"
	if _, err := svc.UpdateDomain(context.Background(), payload); err != nil {
		t.Fatalf("UpdateDomain: %v", err)
	}

	stored := repo.stored(t, created.ID)
	if stored.BasicAuthPass == originalHash {
		t.Fatalf("stored hash must change when a new password is provided")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(stored.BasicAuthPass), []byte("second-secret")); err != nil {
		t.Fatalf("stored hash does not match the new password: %v", err)
	}
}

func TestUpdateDomainRejectsInvalidTargetWithoutPersisting(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, nil)

	created, err := svc.CreateDomain(context.Background(), newTestDomain())
	if err != nil {
		t.Fatalf("CreateDomain: %v", err)
	}

	payload := *created
	payload.TargetContainer = "foo}"

	if _, err := svc.UpdateDomain(context.Background(), payload); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
	if got := repo.stored(t, created.ID).TargetContainer; got != "" {
		t.Fatalf("rejected update must not persist, stored target %q", got)
	}
}

func TestMigrateBasicAuthHashesOnce(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, nil)

	// Simulate a legacy row created before write-time hashing.
	legacy := Domain{
		ID: "legacy-1", Hostname: "core.localhost", ServiceID: "core",
		ContainerPort: 80, BasicAuth: true, BasicAuthUser: "admin", BasicAuthPass: "123",
	}
	if err := repo.Create(context.Background(), legacy); err != nil {
		t.Fatalf("seed: %v", err)
	}

	domains, err := repo.List(context.Background(), "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	svc.MigrateBasicAuthHashes(context.Background(), domains)

	migrated := repo.stored(t, "legacy-1")
	if !strings.HasPrefix(migrated.BasicAuthPass, "$2") {
		t.Fatalf("legacy password must be hashed, got %q", migrated.BasicAuthPass)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(migrated.BasicAuthPass), []byte("123")); err != nil {
		t.Fatalf("migrated hash does not match the legacy password: %v", err)
	}
	if !domains[0].HasBasicAuth {
		t.Fatalf("in-place slice must expose hasBasicAuth after migration")
	}

	// Second run must be a no-op.
	domains2, _ := repo.List(context.Background(), "")
	svc.MigrateBasicAuthHashes(context.Background(), domains2)
	if again := repo.stored(t, "legacy-1"); again.BasicAuthPass != migrated.BasicAuthPass {
		t.Fatalf("migration must be idempotent: hash changed on second run")
	}
}

func TestDomainPayloadAcceptsWriteOnlyPassword(t *testing.T) {
	var p domainPayload
	body := []byte(`{"hostname":"app.example.com","containerPort":80,"serviceId":"web-gui","basicAuth":true,"basicAuthUser":"admin","basicAuthPass":"secret"}`)
	if err := json.Unmarshal(body, &p); err != nil {
		t.Fatalf("decode: %v", err)
	}
	d := p.toDomain()
	if d.BasicAuthPass != "secret" {
		t.Fatalf("request DTO must accept basicAuthPass, got %q", d.BasicAuthPass)
	}

	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), "secret") || strings.Contains(string(raw), "basicAuthPass") {
		t.Fatalf("serialized domain must not leak the password, got %s", raw)
	}
}
