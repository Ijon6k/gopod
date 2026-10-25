package services

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"gopod/internal/db"
)

// newDeployRepoTest opens an isolated sqlite repo for deployment-sequence tests.
func newDeployRepoTest(t *testing.T) (*SQLiteRepository, context.Context) {
	t.Helper()
	tempDB := filepath.Join(t.TempDir(), "deploy_seq.db")
	database, err := db.Open(tempDB)
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return NewSQLiteRepository(database.DB), context.Background()
}

func seedWorkload(t *testing.T, repo *SQLiteRepository, ctx context.Context, serviceID string) {
	t.Helper()
	if _, err := repo.db.Exec("INSERT INTO projects (id, name) VALUES ('proj-seq', 'Seq Project')"); err != nil {
		t.Fatalf("failed to insert project: %v", err)
	}
	if err := repo.Create(ctx, Service{ID: serviceID, ProjectID: "proj-seq", Name: "seq-workload", Type: "compose", Status: "ready"}); err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
}

// NextDeploymentNumber must march past the old ListDeployments 50-row window:
// the sequence used to freeze at 51 because numbering was derived from the
// UI-facing list length.
func TestNextDeploymentNumberPassesFiftyRowWindow(t *testing.T) {
	repo, ctx := newDeployRepoTest(t)
	seedWorkload(t, repo, ctx, "svc-seq")

	now := time.Now()
	for i := 1; i <= 65; i++ {
		dep := Deployment{
			ID:        depIDForSeq(i),
			ProjectID: "proj-seq",
			ServiceID: "svc-seq",
			Number:    i,
			Trigger:   "manual",
			Status:    "success",
			StartedAt: now.Add(time.Duration(i) * time.Minute),
		}
		if err := repo.CreateDeployment(ctx, dep); err != nil {
			t.Fatalf("seed deployment %d: %v", i, err)
		}
	}

	fiftyPlusList, err := repo.ListDeployments(ctx, "svc-seq")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(fiftyPlusList) != 50 {
		t.Fatalf("ListDeployments is expected to stay a 50-row UI window, got %d", len(fiftyPlusList))
	}

	next, err := repo.NextDeploymentNumber(ctx, "svc-seq")
	if err != nil {
		t.Fatalf("next number: %v", err)
	}
	if next != 66 {
		t.Errorf("NextDeploymentNumber = %d, want 66 (must not freeze at the 50-row window)", next)
	}
}

// Legacy rows created before the number column existed may store NULL; the
// sequence must start at 1 rather than error.
func TestNextDeploymentNumberWithLegacyNullNumbers(t *testing.T) {
	repo, ctx := newDeployRepoTest(t)
	seedWorkload(t, repo, ctx, "svc-legacy")

	if _, err := repo.db.Exec(`INSERT INTO deployments (id, project_id, service_id, number, trigger, status, started_at)
		VALUES ('dep-old', 'proj-seq', 'svc-legacy', NULL, 'manual', 'success', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatalf("seed legacy row: %v", err)
	}

	next, err := repo.NextDeploymentNumber(ctx, "svc-legacy")
	if err != nil {
		t.Fatalf("next number: %v", err)
	}
	if next != 1 {
		t.Errorf("NextDeploymentNumber with NULL history = %d, want 1", next)
	}
}

// Restart sweeps rely on ListRunningDeployments finding every non-terminal
// row regardless of position, while terminal rows must be ignored.
func TestListRunningDeploymentsIsUnboundedAndExact(t *testing.T) {
	repo, ctx := newDeployRepoTest(t)
	seedWorkload(t, repo, ctx, "svc-sweep")

	now := time.Now().Add(-time.Hour)
	// 60 "building" rows — far beyond the old LIMIT 50 sweep window.
	for i := 1; i <= 60; i++ {
		dep := Deployment{
			ID:        depIDForSweep(i),
			ProjectID: "proj-seq",
			ServiceID: "svc-sweep",
			Number:    i,
			Status:    "building",
			StartedAt: now.Add(time.Duration(i) * time.Minute),
		}
		if err := repo.CreateDeployment(ctx, dep); err != nil {
			t.Fatalf("seed building row %d: %v", i, err)
		}
	}
	// Terminal rows must never be selected by the sweep.
	terminal := []string{"success", "failed", "cancelled", "interrupted"}
	for i, status := range terminal {
		dep := Deployment{
			ID:        depIDForSweep(100 + i),
			ProjectID: "proj-seq",
			ServiceID: "svc-sweep",
			Number:    100 + i,
			Status:    status,
			StartedAt: now,
		}
		if err := repo.CreateDeployment(ctx, dep); err != nil {
			t.Fatalf("seed terminal row %s: %v", status, err)
		}
	}

	running, err := repo.ListRunningDeployments(ctx, "svc-sweep")
	if err != nil {
		t.Fatalf("list running: %v", err)
	}
	if len(running) != 60 {
		t.Errorf("ListRunningDeployments = %d rows, want 60 (no LIMIT window)", len(running))
	}
	for _, d := range running {
		if d.Status != "building" && d.Status != "deploying" {
			t.Errorf("terminal status %q leaked into running sweep", d.Status)
		}
	}
}

func depIDForSeq(i int) string   { return "dep-seq-" + itoa(i) }
func depIDForSweep(i int) string { return "dep-sweep-" + itoa(i) }

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	digits := ""
	for i > 0 {
		digits = string(rune('0'+i%10)) + digits
		i /= 10
	}
	return digits
}
