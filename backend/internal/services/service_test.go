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

	_, err = database.DB.Exec("INSERT INTO projects (id, name) VALUES ('test-proj', 'Test Project')")
	if err != nil {
		t.Fatalf("failed to insert test project: %v", err)
	}

	// 1. Create Service
	svc := Service{
		ID:        "svc-test-1",
		ProjectID: "test-proj",
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

func TestQuadletUnitNameAndContent(t *testing.T) {
	s := Service{
		ID:        "svc-1234",
		ProjectID: "proj-alpha",
		Name:      "My Cool Service! 99",
	}

	clean := QuadletUnitName(s)
	if clean != "My-Cool-Service-99" {
		t.Errorf("expected My-Cool-Service-99, got %s", clean)
	}

	d := &Deployer{dataDir: t.TempDir()}
	content := d.prepareQuadletContent(s, clean)

	if !strings.Contains(content, "ContainerName=svc-1234") {
		t.Errorf("content missing ContainerName directive:\n%s", content)
	}
	if !strings.Contains(content, "Network=gopod-net") {
		t.Errorf("content missing Network=gopod-net directive:\n%s", content)
	}
	if !strings.Contains(content, "Label=io.gopod.service=svc-1234") {
		t.Errorf("content missing Label=io.gopod.service directive:\n%s", content)
	}

	// Quadlet specification: Restart= MUST NOT be in [Container] section
	containerIdx := strings.Index(content, "[Container]")
	serviceIdx := strings.Index(content, "[Service]")
	if containerIdx != -1 && serviceIdx != -1 {
		containerSection := content[containerIdx:serviceIdx]
		if strings.Contains(containerSection, "Restart=") {
			t.Errorf("[Container] section must not contain Restart= directive:\n%s", containerSection)
		}
	}
}

func TestDeployByWebhookAuth(t *testing.T) {
	tempDB := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(tempDB)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	repo := NewSQLiteRepository(database.DB)
	ctx := context.Background()

	_, err = database.DB.Exec("INSERT INTO projects (id, name) VALUES ('test-proj', 'Test Project')")
	if err != nil {
		t.Fatalf("failed to insert test project: %v", err)
	}

	secretToken := "secret-webhook-token-xyz"
	svc := Service{
		ID:           "svc-secure-1",
		ProjectID:    "test-proj",
		Name:         "secure-service",
		Type:         "image",
		Status:       "stopped",
		WebhookToken: secretToken,
	}
	if err := repo.Create(ctx, svc); err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	ws := NewWorkloadService(repo, nil, nil)

	// 1. Calling with empty token must fail
	if _, err := ws.DeployByWebhook(ctx, ""); err == nil {
		t.Error("expected error for empty webhook token, got nil")
	}

	// 2. Calling with service ID (bypass attempt) must fail
	if _, err := ws.DeployByWebhook(ctx, svc.ID); err == nil {
		t.Error("expected error when passing service ID as webhook token, got nil")
	}

	// 3. Calling with valid webhook token succeeds
	matched, err := ws.DeployByWebhook(ctx, secretToken)
	if err != nil {
		t.Errorf("expected success with valid webhook token, got error: %v", err)
	}
	if matched == nil || matched.ID != svc.ID {
		t.Errorf("unexpected service returned: %+v", matched)
	}
}

func TestParseQuadlet(t *testing.T) {
	sampleQuadlet := `[Unit]
Description=Uptime Kuma Monitoring Service
After=network-online.target

[Container]
ContainerName=uptime-kuma
Image=docker.io/louislam/uptime-kuma:1
PublishPort=3001:3001
Volume=uptime-kuma-data:/app/data:Z
Environment=NODE_ENV=production
Restart=always

[Service]
Restart=always

[Install]
WantedBy=default.target`

	parsed := ParseQuadlet(sampleQuadlet)
	if parsed.Image != "docker.io/louislam/uptime-kuma:1" {
		t.Errorf("expected Image docker.io/louislam/uptime-kuma:1, got %s", parsed.Image)
	}
	if parsed.ContainerName != "uptime-kuma" {
		t.Errorf("expected ContainerName uptime-kuma, got %s", parsed.ContainerName)
	}
	if parsed.HostPort != 3001 || parsed.Port != 3001 {
		t.Errorf("expected HostPort 3001 and Port 3001, got %d, %d", parsed.HostPort, parsed.Port)
	}
	if len(parsed.Ports) != 1 || parsed.Ports[0] != "3001:3001" {
		t.Errorf("unexpected Ports: %+v", parsed.Ports)
	}
	if len(parsed.Volumes) != 1 || parsed.Volumes[0] != "uptime-kuma-data:/app/data:Z" {
		t.Errorf("unexpected Volumes: %+v", parsed.Volumes)
	}
	if parsed.Env["NODE_ENV"] != "production" {
		t.Errorf("expected NODE_ENV=production, got %s", parsed.Env["NODE_ENV"])
	}
	if parsed.RestartPolicy != "always" {
		t.Errorf("expected RestartPolicy always, got %s", parsed.RestartPolicy)
	}
}

func TestValidateContainerImage(t *testing.T) {
	validImages := []string{
		"alpine:latest",
		"docker.io/library/nginx:alpine",
		"ghcr.io/org/app:v1.0",
		"postgres:16-alpine",
		"archlinux:latest",
	}
	for _, img := range validImages {
		if err := ValidateContainerImage(img); err != nil {
			t.Errorf("expected valid image %q, got error: %v", img, err)
		}
	}

	invalidImages := []string{
		"",
		"   ",
		"docker-compose.yaml",
		"compose.yml",
		"/tmp/something.yaml",
		"./relative/path.yml",
	}
	for _, img := range invalidImages {
		if err := ValidateContainerImage(img); err == nil {
			t.Errorf("expected error for invalid image %q, got nil", img)
		}
	}
}

func TestSplitQuadletUnits(t *testing.T) {
	single := `[Unit]
Description=Single Unit
[Container]
Image=alpine:latest`
	units := SplitQuadletUnits(single, "test")
	if len(units) != 1 || units["test.container"] == "" {
		t.Fatalf("expected single test.container, got %+v", units)
	}

	multi := `# File: my-pod.pod
[Unit]
Description=My Pod
[Pod]
PodName=my-pod
---
# File: my-app.container
[Unit]
Description=My App
[Container]
Image=nginx:alpine
Pod=my-pod.pod`
	units = SplitQuadletUnits(multi, "my-app")
	if len(units) != 2 {
		t.Fatalf("expected 2 units, got %d", len(units))
	}
	if units["my-pod.pod"] == "" {
		t.Errorf("missing my-pod.pod in parsed units: %+v", units)
	}
	if units["my-app.container"] == "" {
		t.Errorf("missing my-app.container in parsed units: %+v", units)
	}
}

func TestAntiLoopGuardsInPrepareQuadlet(t *testing.T) {
	d := &Deployer{dataDir: t.TempDir()}
	s := Service{
		ID:        "svc-guard-test",
		ProjectID: "proj-1",
		Name:      "guarded-app",
		Image:     "docker.io/library/alpine:latest",
	}
	content := d.prepareQuadletContent(s, "guarded-app")
	if !strings.Contains(content, "StartLimitIntervalSec=60s") {
		t.Errorf("expected StartLimitIntervalSec=60s in quadlet unit:\n%s", content)
	}
	if !strings.Contains(content, "StartLimitBurst=5") {
		t.Errorf("expected StartLimitBurst=5 in quadlet unit:\n%s", content)
	}
}

func TestValidateQuadletDryRun(t *testing.T) {
	validUnit := `[Unit]
Description=DryRun Test Service
After=network-online.target

[Container]
ContainerName=dryrun-test
Image=docker.io/library/alpine:latest
Exec=sleep 100

[Service]
Restart=always

[Install]
WantedBy=default.target`

	valid, out, err := ValidateQuadletFiles(map[string]string{
		"dryrun-test.container": validUnit,
	})
	if !valid || err != nil {
		t.Fatalf("expected valid dryrun, valid=%v, err=%v, out=%s", valid, err, out)
	}

	invalidUnit := `[Unit]
Description=Invalid Restart Directive
[Container]
ContainerName=bad
Image=alpine:latest
Restart=always`

	valid, _, _ = ValidateQuadletFiles(map[string]string{
		"bad.container": invalidUnit,
	})
	if valid {
		t.Errorf("expected invalid unit due to Restart= in [Container]")
	}
}
