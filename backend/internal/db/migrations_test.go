package db

import (
	"path/filepath"
	"testing"
)

func TestMigrateBackfillsAutoDeployOnce(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "migration.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer database.Close()

	if _, err := database.Exec(`INSERT INTO projects (id, name) VALUES ('p1', 'project')`); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if _, err := database.Exec(`INSERT INTO services (id, project_id, name, type, auto_deploy) VALUES ('s1', 'p1', 'svc', 'application', 0)`); err != nil {
		t.Fatalf("insert service: %v", err)
	}

	// Simulate a database created before the one-time backfill ran.
	if _, err := database.Exec("PRAGMA user_version = 0"); err != nil {
		t.Fatalf("reset user_version: %v", err)
	}
	if err := database.migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	var autoDeploy int
	if err := database.QueryRow(`SELECT auto_deploy FROM services WHERE id = 's1'`).Scan(&autoDeploy); err != nil {
		t.Fatalf("read auto_deploy: %v", err)
	}
	if autoDeploy != 1 {
		t.Fatalf("expected the first migration to enable auto-deploy, got %d", autoDeploy)
	}

	// A later explicit opt-out must survive subsequent startups.
	if _, err := database.Exec(`UPDATE services SET auto_deploy = 0 WHERE id = 's1'`); err != nil {
		t.Fatalf("disable auto-deploy: %v", err)
	}
	if err := database.migrate(); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if err := database.QueryRow(`SELECT auto_deploy FROM services WHERE id = 's1'`).Scan(&autoDeploy); err != nil {
		t.Fatalf("read auto_deploy: %v", err)
	}
	if autoDeploy != 0 {
		t.Fatalf("data migration ran twice and re-enabled auto-deploy")
	}
}

func TestMigrateSystemProjectAndVolumeSnapshots(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "system_snap.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer database.Close()

	// Verify system project exists
	var sysName string
	err = database.QueryRow(`SELECT name FROM projects WHERE id = 'system'`).Scan(&sysName)
	if err != nil {
		t.Fatalf("system project missing after migrate: %v", err)
	}

	// Verify inserting a system snapshot succeeds with foreign keys enabled
	_, err = database.Exec(`
		INSERT INTO volume_snapshots (id, project_id, service_id, volume_name, filename, size, size_bytes, status, compression)
		VALUES ('snap-sys-1', 'system', 'gopod', 'system:gopod', 'backup.tar.gz', '1.2 MB', 123456, 'completed', 'gzip')
	`)
	if err != nil {
		t.Fatalf("inserting system snapshot failed: %v", err)
	}

	// Verify inserting a system schedule succeeds with foreign keys enabled
	_, err = database.Exec(`
		INSERT INTO volume_schedules (id, project_id, service_id, volume_name, cron, label, retention_count, enabled)
		VALUES ('sched-sys-1', 'system', 'gopod', 'system:gopod', '0 3 * * *', 'System Backup', 7, 1)
	`)
	if err != nil {
		t.Fatalf("inserting system schedule failed: %v", err)
	}
}
