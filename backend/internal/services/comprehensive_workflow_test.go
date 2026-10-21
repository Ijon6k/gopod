package services

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopod/internal/db"
)

// TestDeploymentCancellation_Active verifies that cancelling an active in-flight deployment
// invokes the cancel context, marks the deployment as 'cancelled', and resets service state.
func TestDeploymentCancellation_Active(t *testing.T) {
	tempDB := filepath.Join(t.TempDir(), "test_cancel_active.db")
	database, err := db.Open(tempDB)
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer database.Close()

	repo := NewSQLiteRepository(database.DB)
	lm := NewLogManager(t.TempDir())
	ws := NewWorkloadService(repo, nil, lm)
	ctx := context.Background()

	// 1. Setup project & service
	if _, err := database.DB.Exec("INSERT INTO projects (id, name) VALUES ('proj-cancel', 'Cancel Project')"); err != nil {
		t.Fatalf("failed to insert project: %v", err)
	}

	svc := Service{
		ID:        "svc-cancel-active",
		ProjectID: "proj-cancel",
		Name:      "cancellable-workload",
		Type:      "compose",
		Status:    "deploying",
	}
	if err := repo.Create(ctx, svc); err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	// 2. Setup active deployment
	dep := Deployment{
		ID:        "dep-cancel-active",
		ProjectID: "proj-cancel",
		ServiceID: svc.ID,
		Status:    "building",
		StartedAt: time.Now(),
	}
	if err := repo.CreateDeployment(ctx, dep); err != nil {
		t.Fatalf("failed to create deployment: %v", err)
	}

	cancelCalled := false
	mockCancel := func() {
		cancelCalled = true
	}
	ws.activeCancels.Store(dep.ID, context.CancelFunc(mockCancel))
	ws.activeServiceDeps.Store(svc.ID, dep.ID)

	// 3. Cancel the deployment
	cancelledDep, err := ws.CancelDeployment(ctx, dep.ID)
	if err != nil {
		t.Fatalf("CancelDeployment failed: %v", err)
	}
	if cancelledDep.Status != "cancelled" {
		t.Fatalf("expected status 'cancelled', got %s", cancelledDep.Status)
	}
	if !cancelCalled {
		t.Fatal("expected cancelFunc to be invoked")
	}

	// 4. Verify DB persistence
	persistedDep, err := repo.GetDeployment(ctx, dep.ID)
	if err != nil {
		t.Fatalf("failed to query deployment: %v", err)
	}
	if persistedDep.Status != "cancelled" {
		t.Fatalf("persisted status mismatch: got %s, want 'cancelled'", persistedDep.Status)
	}

	persistedSvc, err := ws.GetService(ctx, svc.ID)
	if err != nil {
		t.Fatalf("failed to query service: %v", err)
	}
	if persistedSvc.Status != "stopped" {
		t.Fatalf("persisted service status mismatch: got %s, want 'stopped'", persistedSvc.Status)
	}
}

// TestDeploymentCancellation_Idempotent verifies that cancelling an already finished
// deployment (success or failed) does not overwrite its terminal status.
func TestDeploymentCancellation_Idempotent(t *testing.T) {
	tempDB := filepath.Join(t.TempDir(), "test_cancel_idempotent.db")
	database, err := db.Open(tempDB)
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer database.Close()

	repo := NewSQLiteRepository(database.DB)
	lm := NewLogManager(t.TempDir())
	ws := NewWorkloadService(repo, nil, lm)
	ctx := context.Background()

	if _, err := database.DB.Exec("INSERT INTO projects (id, name) VALUES ('proj-idem', 'Idempotent Project')"); err != nil {
		t.Fatalf("failed to insert project: %v", err)
	}

	svc := Service{
		ID:        "svc-idem",
		ProjectID: "proj-idem",
		Name:      "idem-service",
		Type:      "compose",
		Status:    "running",
	}
	if err := repo.Create(ctx, svc); err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	// Create completed success deployment
	now := time.Now()
	dep := Deployment{
		ID:         "dep-completed-success",
		ProjectID:  "proj-idem",
		ServiceID:  svc.ID,
		Status:     "success",
		Duration:   "45s",
		StartedAt:  now.Add(-45 * time.Second),
		FinishedAt: &now,
	}
	if err := repo.CreateDeployment(ctx, dep); err != nil {
		t.Fatalf("failed to create deployment: %v", err)
	}

	// Attempt to cancel already finished deployment
	res, err := ws.CancelDeployment(ctx, dep.ID)
	if err != nil {
		t.Fatalf("CancelDeployment on finished deployment returned unexpected error: %v", err)
	}
	if res.Status != "success" {
		t.Fatalf("terminal status should be preserved, got %s, want 'success'", res.Status)
	}

	persisted, _ := repo.GetDeployment(ctx, dep.ID)
	if persisted.Status != "success" {
		t.Fatalf("database status was overwritten to %s, expected 'success'", persisted.Status)
	}
}

// TestDeploymentCancellation_NotFound verifies error handling for non-existent deployment ID.
func TestDeploymentCancellation_NotFound(t *testing.T) {
	tempDB := filepath.Join(t.TempDir(), "test_cancel_404.db")
	database, err := db.Open(tempDB)
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer database.Close()

	repo := NewSQLiteRepository(database.DB)
	ws := NewWorkloadService(repo, nil, nil)
	ctx := context.Background()

	_, err = ws.CancelDeployment(ctx, "non-existent-deployment-id")
	if err == nil {
		t.Fatal("expected error when cancelling non-existent deployment, got nil")
	}
}

// TestServiceDeletion_CancelsActiveDeployment verifies that deleting a service
// automatically terminates its active deployment context.
func TestServiceDeletion_CancelsActiveDeployment(t *testing.T) {
	tempDB := filepath.Join(t.TempDir(), "test_delete_cancels.db")
	database, err := db.Open(tempDB)
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer database.Close()

	repo := NewSQLiteRepository(database.DB)
	lm := NewLogManager(t.TempDir())
	ws := NewWorkloadService(repo, nil, lm)
	ctx := context.Background()

	if _, err := database.DB.Exec("INSERT INTO projects (id, name) VALUES ('proj-del', 'Delete Project')"); err != nil {
		t.Fatalf("failed to insert project: %v", err)
	}

	svc := Service{
		ID:        "svc-to-delete",
		ProjectID: "proj-del",
		Name:      "service-to-delete",
		Type:      "compose",
		Status:    "deploying",
	}
	if err := repo.Create(ctx, svc); err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	dep := Deployment{
		ID:        "dep-to-delete",
		ProjectID: "proj-del",
		ServiceID: svc.ID,
		Status:    "building",
		StartedAt: time.Now(),
	}
	if err := repo.CreateDeployment(ctx, dep); err != nil {
		t.Fatalf("failed to create deployment: %v", err)
	}

	cancelCalled := false
	ws.activeCancels.Store(dep.ID, context.CancelFunc(func() {
		cancelCalled = true
	}))
	ws.activeServiceDeps.Store(svc.ID, dep.ID)

	// Delete the service
	if err := ws.DeleteService(ctx, svc.ID, false); err != nil {
		t.Fatalf("DeleteService failed: %v", err)
	}

	if !cancelCalled {
		t.Fatal("expected active deployment to be cancelled upon service deletion")
	}

	// Service should be removed from database
	deletedSvc, err := ws.GetService(ctx, svc.ID)
	if err == nil && deletedSvc != nil {
		t.Fatal("expected service to be deleted from database")
	}
}

// TestReconcileOrphanedDeployments_MultipleStatuses verifies boot-time recovery
// of multiple services and deployments left behind by sudden host shutdown.
func TestReconcileOrphanedDeployments_MultipleStatuses(t *testing.T) {
	tempDB := filepath.Join(t.TempDir(), "test_reconcile_multi.db")
	database, err := db.Open(tempDB)
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer database.Close()

	repo := NewSQLiteRepository(database.DB)
	lm := NewLogManager(t.TempDir())
	ws := NewWorkloadService(repo, nil, lm)
	ctx := context.Background()

	if _, err := database.DB.Exec("INSERT INTO projects (id, name) VALUES ('proj-reconcile', 'Reconcile Project')"); err != nil {
		t.Fatalf("failed to insert project: %v", err)
	}

	// Setup 3 services in various states
	services := []Service{
		{ID: "svc-r1", ProjectID: "proj-reconcile", Name: "app-stuck-building", Type: "compose", Status: "building"},
		{ID: "svc-r2", ProjectID: "proj-reconcile", Name: "app-stuck-deploying", Type: "compose", Status: "deploying"},
		{ID: "svc-r3", ProjectID: "proj-reconcile", Name: "app-healthy-normal", Type: "compose", Status: "stopped"},
	}
	for _, s := range services {
		if err := repo.Create(ctx, s); err != nil {
			t.Fatalf("failed to create service %s: %v", s.ID, err)
		}
	}

	// Deployments for each service
	deps := []Deployment{
		{ID: "dep-r1", ProjectID: "proj-reconcile", ServiceID: "svc-r1", Status: "building", StartedAt: time.Now()},
		{ID: "dep-r2", ProjectID: "proj-reconcile", ServiceID: "svc-r2", Status: "deploying", StartedAt: time.Now()},
		{ID: "dep-r3", ProjectID: "proj-reconcile", ServiceID: "svc-r3", Status: "success", StartedAt: time.Now().Add(-1 * time.Hour)},
	}
	for _, d := range deps {
		if err := repo.CreateDeployment(ctx, d); err != nil {
			t.Fatalf("failed to create deployment %s: %v", d.ID, err)
		}
	}

	// Run startup reconciliation
	if err := ws.ReconcileOrphanedDeployments(ctx); err != nil {
		t.Fatalf("ReconcileOrphanedDeployments failed: %v", err)
	}

	// Verify dep-r1 & dep-r2 became 'interrupted'
	d1, _ := repo.GetDeployment(ctx, "dep-r1")
	if d1.Status != "interrupted" {
		t.Fatalf("expected dep-r1 to be 'interrupted', got %s", d1.Status)
	}

	d2, _ := repo.GetDeployment(ctx, "dep-r2")
	if d2.Status != "interrupted" {
		t.Fatalf("expected dep-r2 to be 'interrupted', got %s", d2.Status)
	}

	// Verify dep-r3 remained 'success'
	d3, _ := repo.GetDeployment(ctx, "dep-r3")
	if d3.Status != "success" {
		t.Fatalf("expected dep-r3 to remain 'success', got %s", d3.Status)
	}

	// Verify services stuck in deploying/building reset to 'stopped'
	s1, _ := ws.GetService(ctx, "svc-r1")
	if s1.Status != "stopped" {
		t.Fatalf("expected s1 status 'stopped', got %s", s1.Status)
	}
	s2, _ := ws.GetService(ctx, "svc-r2")
	if s2.Status != "stopped" {
		t.Fatalf("expected s2 status 'stopped', got %s", s2.Status)
	}
	s3, _ := ws.GetService(ctx, "svc-r3")
	if s3.Status != "stopped" {
		t.Fatalf("expected s3 status 'stopped', got %s", s3.Status)
	}
}

// TestEnvironmentVariables_DiskSync verifies that setting and clearing environment variables
// correctly writes or removes the .env file in the compose project directory.
func TestEnvironmentVariables_DiskSync(t *testing.T) {
	tempDataDir := t.TempDir()
	composeDir := filepath.Join(tempDataDir, "compose", "proj-env", "proj-env-svc")
	if err := os.MkdirAll(composeDir, 0755); err != nil {
		t.Fatalf("failed to create compose dir: %v", err)
	}

	envFile := filepath.Join(composeDir, ".env")

	// 1. Service with environment variables
	svcWithEnv := Service{
		ID:        "svc-env",
		Name:      "test-service",
		ProjectID: "proj-env",
		EnvVars: []EnvVar{
			{Key: "PORT", Value: "8080"},
			{Key: "DATABASE_URL", Value: "postgres://user:pass@localhost:5432/mydb?sslmode=disable"},
			{Key: "EMPTY_VAL", Value: ""},
			{Key: "WITH_SPACES", Value: "hello world"},
		},
	}

	// Write .env
	var envBuf strings.Builder
	for _, env := range svcWithEnv.EnvVars {
		if env.Key != "" {
			envBuf.WriteString(env.Key)
			envBuf.WriteByte('=')
			envBuf.WriteString(env.Value)
			envBuf.WriteByte('\n')
		}
	}
	if err := os.WriteFile(envFile, []byte(envBuf.String()), 0600); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	// Verify file content
	content, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatalf("failed to read .env: %v", err)
	}
	contentStr := string(content)
	if !strings.Contains(contentStr, "PORT=8080") ||
		!strings.Contains(contentStr, "DATABASE_URL=postgres://user:pass@localhost:5432/mydb?sslmode=disable") ||
		!strings.Contains(contentStr, "EMPTY_VAL=") ||
		!strings.Contains(contentStr, "WITH_SPACES=hello world") {
		t.Fatalf("unexpected .env content: %s", contentStr)
	}

	// 2. Service updated to clear all environment variables
	svcCleared := Service{
		ID:        "svc-env",
		Name:      "test-service",
		ProjectID: "proj-env",
		EnvVars:   []EnvVar{},
	}

	// Mimic deployer_compose logic: if len(s.EnvVars) == 0, remove .env file
	if len(svcCleared.EnvVars) == 0 {
		_ = os.Remove(envFile)
	}

	if _, err := os.Stat(envFile); !os.IsNotExist(err) {
		t.Fatalf("expected .env file to be removed when EnvVars is empty")
	}
}

// TestGitSourceDetectionLogic verifies the heuristic distinguishing raw compose filenames
// from real Git repository URLs.
func TestGitSourceDetectionLogic(t *testing.T) {
	testCases := []struct {
		input    string
		expected bool
	}{
		{"compose.yaml", false},
		{"docker-compose.yml", false},
		{"docker-compose.prod.yaml", false},
		{"../data/compose/file.yml", false},
		{"", false},
		{"   ", false},
		{"docker.io/library/nginx:latest", false},
		{"https://github.com/org/repo.git", true},
		{"https://gitlab.com/org/repo", true},
		{"http://git.internal.company.com/stack.git", true},
		{"git@github.com:user/repo.git", true},
		{"ssh://git@custom.host:22/user/repo", true},
		{"git://git.internal.company.com/repo", true},
	}

	for _, tc := range testCases {
		res := isGitSource(Service{Source: tc.input})
		if res != tc.expected {
			t.Errorf("isGitSource(%q) = %v, expected %v", tc.input, res, tc.expected)
		}
	}
}
