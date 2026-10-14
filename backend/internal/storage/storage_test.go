package storage

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func setupTestStorageDB(t *testing.T) (*sql.DB, func()) {
	tmpDir, err := os.MkdirTemp("", "gopod-storage-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}

	schema := `
	CREATE TABLE volume_snapshots (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		service_id TEXT DEFAULT '',
		volume_name TEXT NOT NULL,
		filename TEXT NOT NULL,
		size TEXT DEFAULT '',
		size_bytes INTEGER DEFAULT 0,
		status TEXT DEFAULT 'completed',
		compression TEXT DEFAULT 'gzip',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE volume_schedules (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		volume_name TEXT NOT NULL,
		cron TEXT NOT NULL,
		label TEXT DEFAULT '',
		retention_count INTEGER DEFAULT 7,
		enabled BOOLEAN DEFAULT 1,
		last_run DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	cleanup := func() {
		_ = db.Close()
		_ = os.RemoveAll(tmpDir)
	}

	return db, cleanup
}

func TestSnapshotFilenamePathTraversalSanitization(t *testing.T) {
	db, cleanup := setupTestStorageDB(t)
	defer cleanup()

	repo := NewSQLiteRepository(db)
	svc := NewService(repo)
	ctx := context.Background()

	// Attempt path traversal via filename
	snap, err := svc.CreateSnapshot(ctx, VolumeSnapshot{
		ProjectID:  "proj-1",
		VolumeName: "app_data",
		Filename:   "../../../../etc/passwd",
	})
	if err != nil {
		t.Fatalf("CreateSnapshot error: %v", err)
	}

	// Filename must NOT contain directory separators or traversal sequences
	if strings.Contains(snap.Filename, "..") || strings.Contains(snap.Filename, "/") || strings.Contains(snap.Filename, "\\") {
		t.Fatalf("Path traversal sequence was not sanitized: %s", snap.Filename)
	}

	if snap.Filename != "passwd" {
		t.Fatalf("Expected sanitized filename 'passwd', got: %s", snap.Filename)
	}
}
