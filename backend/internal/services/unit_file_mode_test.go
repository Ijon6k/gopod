package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestUpdateServiceQuadletUnitsAreOwnerOnly pins the file-mode contract: unit
// files embed real env values (systemd needs them), so they must stay 0600 —
// including files written by older builds (self-healing chmod on rewrite).
// PATH is pointed at a no-op systemctl so the test never touches the real user
// manager.
func TestUpdateServiceQuadletUnitsAreOwnerOnly(t *testing.T) {
	ws, repo, ctx := newSecretsTestRepo(t)

	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "systemctl"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake systemctl: %v", err)
	}
	t.Setenv("PATH", binDir)

	dataDir := t.TempDir()
	systemdDir := t.TempDir()
	t.Setenv("QUADLET_DIR", systemdDir)
	ws.deployer = NewDeployer(nil, nil, nil, dataDir)

	created, err := ws.CreateService(ctx, Service{
		ID:            "proj-mode-1",
		ProjectID:     "proj-secrets",
		Name:          "unit-mode",
		Type:          "quadlet",
		QuadletConfig: "[Container]\nImage=alpine:latest\nEnvironment=\"API_SECRET=unit-value\"\n",
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	// Simulate a unit written by an older build with world-readable mode.
	// Unit file names follow ContainerName (= sanitized service ID).
	localPath := filepath.Join(dataDir, "quadlets", "proj-mode-1.container")
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(localPath, []byte("stale"), 0644); err != nil {
		t.Fatalf("seed old unit: %v", err)
	}

	if err := ws.UpdateService(ctx, *created); err != nil {
		t.Fatalf("update service: %v", err)
	}

	for _, path := range []string{localPath, filepath.Join(systemdDir, "proj-mode-1.container")} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("unit %s missing: %v", path, err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Fatalf("unit %s mode = %o, want 600", path, perm)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if !strings.Contains(string(content), `Environment="API_SECRET=unit-value"`) {
			t.Fatalf("unit %s must keep the real env value for systemd: %s", path, content)
		}
	}

	// The stored row keeps the secret flagged and intact.
	stored, err := repo.Get(ctx, created.ID)
	if err != nil || stored == nil {
		t.Fatalf("get service: %v", err)
	}
	if len(stored.EnvVars) != 0 {
		// CreateService does not copy QuadletConfig env vars into EnvVars; the
		// guard documents that the update path left the row untouched.
		t.Fatalf("unexpected env vars on the stored row: %+v", stored.EnvVars)
	}
}
