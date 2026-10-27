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
	content := d.prepareQuadletContent(context.Background(), s, clean)

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
	content := d.prepareQuadletContent(context.Background(), s, "guarded-app")
	if !strings.Contains(content, "StartLimitIntervalSec=60s") {
		t.Errorf("expected StartLimitIntervalSec=60s in quadlet unit:\n%s", content)
	}
	if !strings.Contains(content, "StartLimitBurst=5") {
		t.Errorf("expected StartLimitBurst=5 in quadlet unit:\n%s", content)
	}
}

func TestValidateQuadletDryRun(t *testing.T) {
	if quadletBinary() == "" {
		t.Skip("quadlet dryrun binary not present; the generator-backed invalid-unit assertion needs the real quadlet")
	}
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

func TestGetQuadletCandidateUnitNames(t *testing.T) {
	s := Service{
		ID:   "svc-arch-99",
		Name: "Arch App",
		Type: "quadlet",
		QuadletConfig: `[Unit]
Description=Arch Linux Web
[Container]
Image=docker.io/library/archlinux:latest
ContainerName=archlinux
PublishPort=8080:80`,
	}

	cands := getQuadletCandidateUnitNames(s, "arch-app")
	foundArchlinux := false
	for _, c := range cands {
		if c == "archlinux.service" {
			foundArchlinux = true
			break
		}
	}
	if !foundArchlinux {
		t.Fatalf("expected archlinux.service in candidate units, got %+v", cands)
	}
}

func TestFindServiceUnitFilesAndCandidates(t *testing.T) {
	tempDir := t.TempDir()
	quadDir := filepath.Join(tempDir, "quadlets")
	_ = os.MkdirAll(quadDir, 0755)

	// Create an orphaned quadlet file on disk from a previous iteration of the service
	orphanFile := filepath.Join(quadDir, "arch-sandbox.container")
	orphanContent := `[Unit]
Description=GoPod archlinux Service
[Container]
ContainerName=arch-sandbox
Image=docker.io/library/archlinux:latest
Label=io.gopod.service=self-hosted-archlinux-dbd3a5
Label=io.gopod.project=self-hosted`
	if err := os.WriteFile(orphanFile, []byte(orphanContent), 0644); err != nil {
		t.Fatalf("failed to write test quadlet file: %v", err)
	}

	d := NewDeployer(nil, nil, nil, tempDir)

	// Service has been recreated with a new ID (self-hosted-archlinux-709f64) and is in Compose format
	s := Service{
		ID:        "self-hosted-archlinux-709f64",
		ProjectID: "self-hosted",
		Name:      "archlinux",
		Type:      "compose",
		ComposeYaml: `services:
  archlinux:
    image: docker.io/library/archlinux:latest
    container_name: arch-sandbox`,
	}

	units, containers, files := d.findServiceUnitFilesAndCandidates(s)

	hasUnit := false
	for _, u := range units {
		if u == "arch-sandbox.service" {
			hasUnit = true
			break
		}
	}
	if !hasUnit {
		t.Errorf("expected arch-sandbox.service in units, got %+v", units)
	}

	hasCont := false
	for _, c := range containers {
		if c == "arch-sandbox" {
			hasCont = true
			break
		}
	}
	if !hasCont {
		t.Errorf("expected arch-sandbox in containers, got %+v", containers)
	}

	hasFile := false
	for _, f := range files {
		if f == orphanFile {
			hasFile = true
			break
		}
	}
	if !hasFile {
		t.Errorf("expected orphanFile %s in files to delete, got %+v", orphanFile, files)
	}

	// Now execute TeardownService and verify the file is completely removed
	if err := d.TeardownService(context.Background(), s, false); err != nil {
		t.Fatalf("TeardownService failed: %v", err)
	}

	if _, err := os.Stat(orphanFile); !os.IsNotExist(err) {
		t.Errorf("expected orphanFile to be removed by TeardownService, but it still exists")
	}
}

func TestDatabaseServiceLifecycle(t *testing.T) {
	tempDB := filepath.Join(t.TempDir(), "test_db_svc.db")
	database, err := db.Open(tempDB)
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer database.Close()

	repo := NewSQLiteRepository(database.DB)
	ws := NewWorkloadService(repo, nil, nil)
	ctx := context.Background()

	// 1. Setup project
	_, err = database.DB.Exec("INSERT INTO projects (id, name) VALUES ('proj-db-test', 'DB Test Project')")
	if err != nil {
		t.Fatalf("failed to insert project: %v", err)
	}

	// 2. Create PostgreSQL Database Service with customizable attributes
	initialSvc := Service{
		ProjectID: "proj-db-test",
		Name:      "postgres-primary",
		Type:      "database",
		Image:     "docker.io/library/postgres:16-alpine",
		Port:      5432,
		HostPort:  15432,
		Command:   "",
		EnvVars: []EnvVar{
			{Key: "POSTGRES_DB", Value: "custom_app_db", Secret: false},
			{Key: "POSTGRES_USER", Value: "db_admin_user", Secret: false},
			{Key: "POSTGRES_PASSWORD", Value: "InitialSecretPass123!", Secret: true},
			{Key: "PGDATA", Value: "/var/lib/postgresql/data/pgdata", Secret: false},
		},
		Description: "Primary PostgreSQL cluster database",
	}

	created, err := ws.CreateService(ctx, initialSvc)
	if err != nil {
		t.Fatalf("failed to create database service: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected generated service ID, got empty")
	}
	if created.Type != "database" {
		t.Fatalf("expected type database, got %s", created.Type)
	}

	// 3. Retrieve service and verify credentials
	fetched, err := ws.GetService(ctx, created.ID)
	if err != nil {
		t.Fatalf("failed to get database service: %v", err)
	}
	if fetched.Port != 5432 || fetched.HostPort != 15432 {
		t.Fatalf("unexpected ports: port=%d, hostPort=%d", fetched.Port, fetched.HostPort)
	}
	if fetched.Image != "docker.io/library/postgres:16-alpine" {
		t.Fatalf("unexpected image: %s", fetched.Image)
	}

	foundDB, foundUser, foundPass := false, false, false
	for _, env := range fetched.EnvVars {
		if env.Key == "POSTGRES_DB" && env.Value == "custom_app_db" {
			foundDB = true
		}
		if env.Key == "POSTGRES_USER" && env.Value == "db_admin_user" {
			foundUser = true
		}
		if env.Key == "POSTGRES_PASSWORD" && env.Value == "InitialSecretPass123!" {
			foundPass = true
		}
	}
	if !foundDB || !foundUser || !foundPass {
		t.Fatalf("expected env vars to be preserved: db=%v, user=%v, pass=%v", foundDB, foundUser, foundPass)
	}

	// 4. Update Database Service (Upgrade image to 17-alpine, edit user, db name, password, and port)
	fetched.Image = "docker.io/library/postgres:17-alpine"
	fetched.Port = 5433
	fetched.HostPort = 25432
	fetched.Description = "Upgraded to PostgreSQL 17-alpine with updated credentials"
	fetched.EnvVars = []EnvVar{
		{Key: "POSTGRES_DB", Value: "new_production_db", Secret: false},
		{Key: "POSTGRES_USER", Value: "new_app_user", Secret: false},
		{Key: "POSTGRES_PASSWORD", Value: "RotatedSecurePass456!", Secret: true},
		{Key: "PGDATA", Value: "/var/lib/postgresql/data/pgdata", Secret: false},
		{Key: "CUSTOM_EXTRA_VAR", Value: "preserved_val", Secret: false},
	}

	err = repo.Update(ctx, *fetched)
	if err != nil {
		t.Fatalf("failed to update database service: %v", err)
	}

	// 5. Re-fetch and verify all updated fields are persisted without state loss
	updated, err := ws.GetService(ctx, created.ID)
	if err != nil {
		t.Fatalf("failed to re-fetch updated service: %v", err)
	}
	if updated.Image != "docker.io/library/postgres:17-alpine" {
		t.Fatalf("expected upgraded image postgres:17-alpine, got %s", updated.Image)
	}
	if updated.Port != 5433 || updated.HostPort != 25432 {
		t.Fatalf("expected updated ports 5433:25432, got %d:%d", updated.Port, updated.HostPort)
	}

	foundNewDB, foundNewUser, foundNewPass, foundCustom := false, false, false, false
	for _, env := range updated.EnvVars {
		if env.Key == "POSTGRES_DB" && env.Value == "new_production_db" {
			foundNewDB = true
		}
		if env.Key == "POSTGRES_USER" && env.Value == "new_app_user" {
			foundNewUser = true
		}
		if env.Key == "POSTGRES_PASSWORD" && env.Value == "RotatedSecurePass456!" {
			foundNewPass = true
		}
		if env.Key == "CUSTOM_EXTRA_VAR" && env.Value == "preserved_val" {
			foundCustom = true
		}
	}
	if !foundNewDB || !foundNewUser || !foundNewPass || !foundCustom {
		t.Fatalf("updated env vars verification failed: db=%v, user=%v, pass=%v, custom=%v",
			foundNewDB, foundNewUser, foundNewPass, foundCustom)
	}
}

func TestCancelDeploymentAndReconcile(t *testing.T) {
	tempDB := filepath.Join(t.TempDir(), "test_cancel.db")
	database, err := db.Open(tempDB)
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer database.Close()

	repo := NewSQLiteRepository(database.DB)
	lm := NewLogManager(t.TempDir())
	ws := NewWorkloadService(repo, nil, lm)
	ctx := context.Background()

	// 1. Create parent project to satisfy foreign keys
	if _, err := database.DB.Exec("INSERT INTO projects (id, name) VALUES ('proj-1', 'Test Project')"); err != nil {
		t.Fatalf("failed to insert project: %v", err)
	}

	// 2. Create test service
	svc := Service{
		ID:        "svc-cancel-1",
		ProjectID: "proj-1",
		Name:      "cancellable-app",
		Type:      "compose",
		Status:    "deploying",
	}
	if err := repo.Create(ctx, svc); err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	// 2. Create test deployment in "building" status
	dep := Deployment{
		ID:        "dep-cancel-1",
		ProjectID: "proj-1",
		ServiceID: svc.ID,
		Status:    "building",
		StartedAt: time.Now(),
	}
	if err := repo.CreateDeployment(ctx, dep); err != nil {
		t.Fatalf("failed to create deployment: %v", err)
	}

	// 3. Register mock cancel func
	cancelledTriggered := false
	mockCancel := func() {
		cancelledTriggered = true
	}
	ws.activeCancels.Store(dep.ID, context.CancelFunc(mockCancel))
	ws.activeServiceDeps.Store(svc.ID, dep.ID)

	// 4. Execute CancelServiceDeployment
	if _, err := ws.CancelServiceDeployment(ctx, svc.ID); err != nil {
		t.Fatalf("CancelServiceDeployment failed: %v", err)
	}

	if !cancelledTriggered {
		t.Fatal("expected active deployment context cancel func to be triggered")
	}

	// Verify deployment status is cancelled
	persistedDep, err := repo.GetDeployment(ctx, dep.ID)
	if err != nil {
		t.Fatalf("failed to get deployment: %v", err)
	}
	if persistedDep.Status != "cancelled" {
		t.Fatalf("expected deployment status to be cancelled, got %s", persistedDep.Status)
	}

	// Verify service status reset to stopped
	persistedSvc, err := ws.GetService(ctx, svc.ID)
	if err != nil {
		t.Fatalf("failed to get service: %v", err)
	}
	if persistedSvc.Status != "stopped" {
		t.Fatalf("expected service status to be stopped, got %s", persistedSvc.Status)
	}

	// 5. Test startup reconciliation of orphaned stuck deployments
	stuckSvc := Service{
		ID:        "svc-stuck-orphan",
		ProjectID: "proj-1",
		Name:      "stuck-orphan-svc",
		Type:      "compose",
		Status:    "deploying",
	}
	if err := repo.Create(ctx, stuckSvc); err != nil {
		t.Fatalf("failed to create stuck service: %v", err)
	}

	stuckDep := Deployment{
		ID:        "dep-stuck-orphan",
		ProjectID: "proj-1",
		ServiceID: stuckSvc.ID,
		Status:    "building",
		StartedAt: time.Now(),
	}
	if err := repo.CreateDeployment(ctx, stuckDep); err != nil {
		t.Fatalf("failed to create stuck deployment: %v", err)
	}

	// Run startup reconciliation
	if err := ws.ReconcileOrphanedDeployments(ctx); err != nil {
		t.Fatalf("ReconcileOrphanedDeployments failed: %v", err)
	}

	// Check reconciled status
	reconciledDep, err := repo.GetDeployment(ctx, stuckDep.ID)
	if err != nil {
		t.Fatalf("failed to get reconciled deployment: %v", err)
	}
	if reconciledDep.Status != "interrupted" {
		t.Fatalf("expected reconciled deployment status to be interrupted, got %s", reconciledDep.Status)
	}

	reconciledSvc, err := ws.GetService(ctx, stuckSvc.ID)
	if err != nil {
		t.Fatalf("failed to get reconciled service: %v", err)
	}
	if reconciledSvc.Status != "stopped" {
		t.Fatalf("expected reconciled service status to be stopped, got %s", reconciledSvc.Status)
	}
}
