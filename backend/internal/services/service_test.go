package services

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopod/internal/db"
)

func TestLogManager(t *testing.T) {
	tempDir := t.TempDir()
	lm := NewLogManager(tempDir)

	depID := "dep-test-123"

	// Subscribe before logging
	subCh, unsubscribe := lm.Subscribe(depID)
	defer unsubscribe()

	// Append log lines
	lm.LogLine(depID, "Step 1: starting deployment")
	lm.LogLine(depID, "Step 2: pulling image")

	// Read from subscriber channel
	select {
	case line := <-subCh:
		if !strings.Contains(line, "starting deployment") {
			t.Errorf("unexpected log line from subscription: %s", line)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for subscription log")
	}

	select {
	case line := <-subCh:
		if !strings.Contains(line, "pulling image") {
			t.Errorf("unexpected log line from subscription: %s", line)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for second subscription log")
	}

	// Verify GetLogs
	fullLogs, err := lm.GetLogs(depID)
	if err != nil {
		t.Fatalf("failed to read logs: %v", err)
	}
	if !strings.Contains(fullLogs, "Step 1") || !strings.Contains(fullLogs, "Step 2") {
		t.Errorf("full logs do not contain expected content: %s", fullLogs)
	}
}

func TestDeploymentCRUD(t *testing.T) {
	tempDB := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(tempDB)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	repo := NewSQLiteRepository(database.DB)
	ctx := context.Background()

	// 1. Create Service
	svc := Service{
		ID:        "svc-test-1",
		ProjectID: "aerochat",
		Name:      "web-test",
		Type:      "compose",
		Status:    "stopped",
	}
	if err := repo.Create(ctx, svc); err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	// 2. Create Initial Deployment
	now := time.Now().UTC()
	dep := Deployment{
		ID:            "dep-1",
		ServiceID:     svc.ID,
		ProjectID:     svc.ProjectID,
		Status:        "building",
		Number:        1,
		Trigger:       "compose",
		Image:         "nginx:alpine",
		CommitHash:    "",
		CommitMessage: "Docker Compose rollout",
		StartedAt:     now,
	}
	if err := repo.CreateDeployment(ctx, dep); err != nil {
		t.Fatalf("failed to create deployment: %v", err)
	}

	// 3. Get Deployment
	loaded, err := repo.GetDeployment(ctx, dep.ID)
	if err != nil {
		t.Fatalf("failed to get deployment: %v", err)
	}
	if loaded.Status != "building" || loaded.Trigger != "compose" || loaded.Number != 1 {
		t.Errorf("unexpected loaded deployment: %+v", loaded)
	}

	// 4. Update Deployment
	finished := time.Now().UTC()
	loaded.Status = "running"
	loaded.Duration = "4.2s"
	loaded.FinishedAt = &finished
	if err := repo.UpdateDeployment(ctx, *loaded); err != nil {
		t.Fatalf("failed to update deployment: %v", err)
	}

	// 5. Verify Updated Deployment
	updated, err := repo.GetDeployment(ctx, dep.ID)
	if err != nil {
		t.Fatalf("failed to get updated deployment: %v", err)
	}
	if updated.Status != "running" || updated.Duration != "4.2s" || updated.FinishedAt == nil {
		t.Errorf("deployment update failed to persist: %+v", updated)
	}

	// 6. List Deployments
	list, err := repo.ListDeployments(ctx, svc.ID)
	if err != nil {
		t.Fatalf("failed to list deployments: %v", err)
	}
	if len(list) != 1 || list[0].Status != "running" {
		t.Errorf("unexpected deployment list: %+v", list)
	}
}
