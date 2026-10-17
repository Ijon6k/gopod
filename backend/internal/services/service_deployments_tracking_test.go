package services

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"gopod/internal/db"
)

// setupDeployTest creates an isolated repo+service for deploy-lock tests.
func setupDeployTest(t *testing.T) (*WorkloadService, Service) {
	t.Helper()
	tempDB := filepath.Join(t.TempDir(), "deploy_lock.db")
	database, err := db.Open(tempDB)
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	repo := NewSQLiteRepository(database.DB)
	lm := NewLogManager(t.TempDir())
	ws := NewWorkloadService(repo, nil, lm)

	ctx := context.Background()
	if _, err := database.DB.Exec("INSERT INTO projects (id, name) VALUES ('proj-lock', 'Lock Project')"); err != nil {
		t.Fatalf("failed to insert project: %v", err)
	}
	svc := Service{
		ID:        "svc-lock-sync",
		ProjectID: "proj-lock",
		Name:      "lock-sync-workload",
		Type:      "compose",
		Status:    "ready",
	}
	if err := repo.Create(ctx, svc); err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	return ws, svc
}

// TestSyncDeploy_ConflictReturnsErrDeployInProgress verifies the synchronous
// Deploy path (used by webhooks) respects the per-service deploy lock and
// surfaces the sentinel error so handlers can answer 409 Conflict.
func TestSyncDeploy_ConflictReturnsErrDeployInProgress(t *testing.T) {
	ws, svc := setupDeployTest(t)

	// A different deployment currently owns the lock.
	if holder := ws.tryAcquireDeploy(svc.ID, "dep-holder"); holder != "" {
		t.Fatalf("failed to pre-hold deploy lock: got holder %q", holder)
	}
	defer ws.releaseDeploy(svc.ID, "dep-holder")

	_, err := ws.Deploy(context.Background(), svc.ID, "manual")
	if !errors.Is(err, ErrDeployInProgress) {
		t.Fatalf("expected ErrDeployInProgress, got %v", err)
	}

	// Rejection must leave no deployment records behind.
	deps, _ := ws.ListDeployments(context.Background(), svc.ID)
	if len(deps) != 0 {
		t.Fatalf("rejected deploy must not create records, got %d", len(deps))
	}
}

// TestStartDeploy_SecondConcurrentDeployConflicts verifies two StartDeploy calls
// for the same service: only the current lock holder proceeds.
func TestStartDeploy_SecondConcurrentDeployConflicts(t *testing.T) {
	ws, svc := setupDeployTest(t)
	ctx := context.Background()

	if holder := ws.tryAcquireDeploy(svc.ID, "dep-in-flight"); holder != "" {
		t.Fatalf("failed to pre-hold deploy lock: got holder %q", holder)
	}
	defer ws.releaseDeploy(svc.ID, "dep-in-flight")

	_, err := ws.StartDeploy(ctx, svc.ID, "manual")
	if !errors.Is(err, ErrDeployInProgress) {
		t.Fatalf("expected ErrDeployInProgress, got %v", err)
	}
}

// TestStartDeploy_LockReleasedAfterCompletion verifies the async path: once the
// deployment goroutine settles, the deploy lock and tracking maps are cleared.
func TestStartDeploy_LockReleasedAfterCompletion(t *testing.T) {
	ws, svc := setupDeployTest(t)
	ctx := context.Background()

	dep, err := ws.StartDeploy(ctx, svc.ID, "manual")
	if err != nil {
		t.Fatalf("StartDeploy failed: %v", err)
	}

	// Wait for the deploy to reach a terminal state (nil deployer => fast fail).
	deadline := time.Now().Add(5 * time.Second)
	for {
		d, getErr := ws.GetDeployment(ctx, dep.ID)
		if getErr == nil && d != nil && (d.Status == "failed" || d.Status == "success" || d.Status == "cancelled") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("deployment did not reach terminal state in time (last getErr=%v)", getErr)
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Lock and tracking maps must be released after completion.
	if _, active := ws.deployLocks.Load(svc.ID); active {
		t.Fatal("deploy lock leaked after completion")
	}
	if _, active := ws.activeServiceDeps.Load(svc.ID); active {
		t.Fatal("activeServiceDeps leaked after completion")
	}
	if _, active := ws.activeCancels.Load(dep.ID); active {
		t.Fatal("activeCancels leaked after completion")
	}
}
