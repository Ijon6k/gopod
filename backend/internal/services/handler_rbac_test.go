package services

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"gopod/internal/auth"
	"gopod/internal/db"
)

func TestServiceAndDeploymentIDORProtection(t *testing.T) {
	tempDir := t.TempDir()
	database, err := db.Open(filepath.Join(tempDir, "services-rbac.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	ctx := context.Background()
	_, _ = database.DB.ExecContext(ctx, `INSERT INTO projects (id, name) VALUES ('proj-a', 'Alpha'), ('proj-b', 'Beta')`)

	svcRepo := NewSQLiteRepository(database.DB)
	deployer := &Deployer{dataDir: tempDir}
	ws := NewWorkloadService(svcRepo, deployer, NewLogManager(tempDir))

	svcA1 := Service{ID: "svc-a1", ProjectID: "proj-a", Name: "alpha-web", Type: "container", Image: "nginx:alpine"}
	svcA2 := Service{ID: "svc-a2", ProjectID: "proj-a", Name: "alpha-db", Type: "container", Image: "postgres:16"}
	svcB1 := Service{ID: "svc-b1", ProjectID: "proj-b", Name: "beta-api", Type: "container", Image: "nginx:alpine"}
	for _, s := range []Service{svcA1, svcA2, svcB1} {
		if err := svcRepo.Create(ctx, s); err != nil {
			t.Fatalf("create service %s: %v", s.ID, err)
		}
	}

	depA := Deployment{ID: "dep-a1", ProjectID: "proj-a", ServiceID: "svc-a1", Status: "success", StartedAt: time.Now()}
	depB := Deployment{ID: "dep-b1", ProjectID: "proj-b", ServiceID: "svc-b1", Status: "building", StartedAt: time.Now()}
	if err := svcRepo.CreateDeployment(ctx, depA); err != nil {
		t.Fatalf("create depA: %v", err)
	}
	if err := svcRepo.CreateDeployment(ctx, depB); err != nil {
		t.Fatalf("create depB: %v", err)
	}

	authRepo := auth.NewSQLiteRepository(database.DB)
	authSvc := auth.NewService(authRepo)
	_, _, _ = authSvc.SetupAdmin(ctx, "Admin", "admin@gopod.local", "adminpass123")
	ac := auth.NewAccessChecker(database.DB)

	// Member granted deploy permission ONLY on svc-a1 inside proj-a (no delete, no reveal secrets).
	member, err := ac.CreateManagedUser(ctx, auth.CreateUserRequest{
		Name:     "ScopedDev",
		Email:    "dev@gopod.local",
		Password: "devpassword123",
		Role:     "member",
		Permissions: &auth.UserPermissions{
			CanDeployServices: true,
		},
		ProjectScopes: []auth.ProjectScope{
			{ProjectID: "proj-a", ServiceIDs: []string{"svc-a1"}},
		},
	})
	if err != nil {
		t.Fatalf("CreateManagedUser: %v", err)
	}
	_, memberTok, err := authSvc.Login(ctx, member.Email, "devpassword123")
	if err != nil {
		t.Fatalf("login member: %v", err)
	}

	mw := auth.NewMiddleware(authSvc)
	mw.SetAccessChecker(ac)
	h := NewHandler(ws, mw)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := func(method, path string, body []byte) *httptest.ResponseRecorder {
		var r *http.Request
		if body != nil {
			r = httptest.NewRequest(method, path, bytes.NewReader(body))
		} else {
			r = httptest.NewRequest(method, path, nil)
		}
		r.Header.Set("Authorization", "Bearer "+memberTok)
		r.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, r)
		return rr
	}

	// 1. Foreign project & unassigned sibling service IDORs MUST return 403
	for _, tc := range []struct {
		method, path string
		body         []byte
	}{
		{http.MethodGet, "/api/services/svc-a2", nil},
		{http.MethodGet, "/api/services/svc-b1", nil},
		{http.MethodGet, "/api/deployments/dep-b1", nil},
		{http.MethodGet, "/api/deployments/dep-b1/logs", nil},
		{http.MethodPost, "/api/deployments/dep-b1/cancel", nil},
		{http.MethodPost, "/api/services/svc-b1/cancel", nil},
		{http.MethodPost, "/api/services/svc-b1/quadlet/validate", []byte(`{}`)},
		{http.MethodPost, "/api/services/svc-b1/quadlet/demote", nil},
		{http.MethodPost, "/api/services/svc-b1/group-pod", []byte(`{"podName":"p"}`)},
		{http.MethodPost, "/api/services/svc-b1/ungroup-pod", nil},
		{http.MethodPost, "/api/services/svc-b1/drop-upload", nil},
	} {
		rr := req(tc.method, tc.path, tc.body)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("%s %s got %d, want 403 (%s)", tc.method, tc.path, rr.Code, rr.Body.String())
		}
	}

	// 2. Permission checks inside allowed svc-a1: reveal secrets, quadlet preview & delete must return 403
	for _, tc := range []struct {
		method, path string
		body         []byte
	}{
		{http.MethodPost, "/api/services/svc-a1/env/reveal", []byte(`{"keys":["FOO"]}`)},
		{http.MethodGet, "/api/services/svc-a1/webhook", nil},
		{http.MethodPost, "/api/services/svc-a1/webhook/rotate", nil},
		{http.MethodPost, "/api/services/svc-a1/quadlet/preview", nil},
		{http.MethodDelete, "/api/services/svc-a1", nil},
		{http.MethodPost, "/api/services", []byte(`{"projectId":"proj-a","name":"new-svc","type":"container"}`)},
	} {
		rr := req(tc.method, tc.path, tc.body)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("%s %s without permission got %d, want 403 (%s)", tc.method, tc.path, rr.Code, rr.Body.String())
		}
	}

	// 3. Allowed service & deployment reads succeed AND PUT cannot hijack ProjectID to proj-b
	if rr := req(http.MethodGet, "/api/services/svc-a1", nil); rr.Code != http.StatusOK {
		t.Fatalf("GET allowed svc-a1 got %d, want 200", rr.Code)
	}
	if rr := req(http.MethodGet, "/api/deployments/dep-a1/logs", nil); rr.Code != http.StatusOK {
		t.Fatalf("GET allowed dep-a1 logs got %d, want 200", rr.Code)
	}

	updateBody, _ := json.Marshal(Service{
		ProjectID: "proj-b", // attempt cross-project move!
		Name:      "alpha-web",
		Type:      "container",
		Image:     "nginx:latest",
	})
	rrUpdate := req(http.MethodPut, "/api/services/svc-a1", updateBody)
	if rrUpdate.Code != http.StatusOK {
		t.Fatalf("PUT svc-a1 got %d: %s", rrUpdate.Code, rrUpdate.Body.String())
	}
	storedA1, _ := svcRepo.Get(ctx, "svc-a1")
	if storedA1.ProjectID != "proj-a" {
		t.Fatalf("ProjectID was hijacked to %q, must stay proj-a", storedA1.ProjectID)
	}
}

