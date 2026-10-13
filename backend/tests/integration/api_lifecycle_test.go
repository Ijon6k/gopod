package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"testing/fstest"

	"gopod/internal/server"
	"gopod/internal/testutil"
)

func setupTestServer(t *testing.T) (http.Handler, string) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "gopod_integration.db")

	cfg := server.Config{
		Port:         "0",
		PodmanSocket: "",
		DBPath:       dbPath,
		CaddyAdmin:   "",
		DistFS:       fstest.MapFS{},
	}

	handler := server.NewServer(cfg)
	return handler, dbPath
}

func TestAPILifecycle_UnauthenticatedGuard(t *testing.T) {
	handler, _ := setupTestServer(t)

	// Attempting to access protected endpoints without authentication
	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/projects"},
		{"POST", "/api/projects"},
		{"GET", "/api/services"},
		{"POST", "/api/services"},
		{"GET", "/api/credentials/ssh-keys"},
		{"GET", "/api/domains"},
	}

	for _, ep := range endpoints {
		req := httptest.NewRequest(ep.method, ep.path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		// Before setup, GOPOD returns 403 Forbidden ("setup required"); after setup it returns 401 Unauthorized
		if rec.Code != http.StatusForbidden && rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s expected 401 or 403, got %d", ep.method, ep.path, rec.Code)
		}
	}
}

func TestAPILifecycle_FullWorkflow(t *testing.T) {
	handler, _ := setupTestServer(t)

	// 1. Initial Status Check
	req := httptest.NewRequest("GET", "/api/auth/status", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on auth status, got %d", rec.Code)
	}

	var statusResp struct {
		Initialized   bool `json:"initialized"`
		Authenticated bool `json:"authenticated"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&statusResp); err != nil {
		t.Fatalf("failed to decode auth status: %v", err)
	}
	if statusResp.Initialized {
		t.Fatal("expected uninitialized cluster")
	}

	// 2. Perform Initial Setup
	setupPayload := map[string]string{
		"name":     "Cluster Administrator",
		"email":    "admin@gopod.internal",
		"password": "MasterAdminPass123!",
	}
	setupBody, _ := json.Marshal(setupPayload)
	req = httptest.NewRequest("POST", "/api/auth/setup", bytes.NewReader(setupBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on setup, got %d: %s", rec.Code, rec.Body.String())
	}

	var setupResp struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&setupResp); err != nil {
		t.Fatalf("failed to decode setup response: %v", err)
	}
	token := setupResp.Token
	if token == "" {
		t.Fatal("expected non-empty token from setup")
	}

	// 3. Create a Project
	projectPayload := map[string]string{
		"name":        "Production Cloud",
		"description": "Primary application workload cluster",
	}
	projBody, _ := json.Marshal(projectPayload)
	req = testutil.NewAuthRequest("POST", "/api/projects", bytes.NewReader(projBody), token)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
		t.Fatalf("expected 200/201 on create project, got %d: %s", rec.Code, rec.Body.String())
	}

	var createdProj struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&createdProj); err != nil {
		t.Fatalf("failed to decode created project: %v", err)
	}
	if createdProj.ID == "" {
		t.Fatal("expected non-empty project ID")
	}

	// 4. Create a Service under the Project
	servicePayload := map[string]interface{}{
		"projectId":   createdProj.ID,
		"name":        "nginx-gateway",
		"type":        "compose",
		"composeYaml": testutil.SampleComposeYAML(),
		"envVars": []map[string]interface{}{
			{"key": "PORT", "value": "8080", "secret": false},
			{"key": "APP_ENV", "value": "production", "secret": false},
		},
	}
	svcBody, _ := json.Marshal(servicePayload)
	req = testutil.NewAuthRequest("POST", "/api/services", bytes.NewReader(svcBody), token)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
		t.Fatalf("expected 200/201 on create service, got %d: %s", rec.Code, rec.Body.String())
	}

	var createdSvc struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&createdSvc); err != nil {
		t.Fatalf("failed to decode created service: %v", err)
	}
	if createdSvc.ID == "" {
		t.Fatal("expected non-empty service ID")
	}

	// 5. Trigger Deployment via HTTP API
	deployPayload := map[string]string{
		"trigger": "manual",
	}
	depBody, _ := json.Marshal(deployPayload)
	req = testutil.NewAuthRequest("POST", "/api/services/"+createdSvc.ID+"/deploy", bytes.NewReader(depBody), token)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK && rec.Code != http.StatusAccepted {
		t.Fatalf("expected 200/202 on trigger deploy, got %d: %s", rec.Code, rec.Body.String())
	}

	var depResp struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&depResp); err != nil {
		t.Fatalf("failed to decode deploy response: %v", err)
	}
	if depResp.ID == "" {
		t.Fatal("expected deployment ID")
	}

	// 6. Cancel Deployment via HTTP API
	req = testutil.NewAuthRequest("POST", "/api/services/"+createdSvc.ID+"/cancel", nil, token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on cancel service deployment, got %d: %s", rec.Code, rec.Body.String())
	}

	// 7. Verify Cancelled Status
	req = testutil.NewAuthRequest("GET", "/api/deployments/"+depResp.ID, nil, token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on get deployment, got %d: %s", rec.Code, rec.Body.String())
	}

	var verifiedDep struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&verifiedDep); err != nil {
		t.Fatalf("failed to decode verified deployment: %v", err)
	}
	if verifiedDep.Status != "cancelled" {
		t.Fatalf("expected status 'cancelled', got %s", verifiedDep.Status)
	}

	// 8. Logout
	req = testutil.NewAuthRequest("POST", "/api/auth/logout", nil, token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on logout, got %d", rec.Code)
	}

	// 9. Subsequent Call with Revoked Token should be rejected
	req = testutil.NewAuthRequest("GET", "/api/projects", nil, token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized after logout, got %d", rec.Code)
	}
}
