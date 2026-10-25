package storage

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"gopod/pkg/crypto"
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
		remote_provider TEXT DEFAULT '',
		remote_destination_id TEXT DEFAULT '',
		remote_key TEXT DEFAULT '',
		checksum TEXT DEFAULT '',
		remote_status TEXT DEFAULT '',
		remote_error TEXT DEFAULT '',
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
		destination_id TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE backup_destinations (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		endpoint TEXT NOT NULL,
		region TEXT DEFAULT 'us-east-1',
		access_key TEXT DEFAULT '',
		secret_enc TEXT DEFAULT '',
		bucket TEXT NOT NULL,
		prefix TEXT DEFAULT '',
		path_style INTEGER DEFAULT 1,
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

func TestDestinationCRUDRoundtrip(t *testing.T) {
	db, cleanup := setupTestStorageDB(t)
	defer cleanup()
	_ = crypto.InitMasterKey(t.TempDir())

	repo := NewSQLiteRepository(db)
	svc := NewService(repo)
	ctx := context.Background()

	created, err := svc.CreateDestination(ctx, BackupDestination{
		Name:      "minio-lab",
		Endpoint:  "https://minio.internal:9000/",
		Bucket:    "gopod-backups",
		AccessKey: "AKIA-EXAMPLE",
		SecretKey: "super-secret-value",
		Prefix:    "/prod/project-1/",
		PathStyle: true,
	})
	if err != nil {
		t.Fatalf("CreateDestination error: %v", err)
	}

	// Response contract: secret never echoed back.
	if created.SecretKey != "" {
		t.Fatalf("CreateDestination response leaked the secret key")
	}

	dests, err := svc.ListDestinations(ctx)
	if err != nil {
		t.Fatalf("ListDestinations error: %v", err)
	}
	if len(dests) != 1 {
		t.Fatalf("expected 1 destination, got %d", len(dests))
	}
	d := dests[0]
	if d.Name != "minio-lab" || d.Endpoint != "https://minio.internal:9000" {
		t.Fatalf("unexpected destination fields: %+v", d)
	}
	if d.Prefix != "prod/project-1" {
		t.Fatalf("expected trimmed prefix 'prod/project-1', got %q", d.Prefix)
	}
	if d.SecretKey != "" {
		t.Fatalf("ListDestinations leaked the secret key")
	}

	// Repository layer returns the decrypted secret for authenticated use.
	raw, err := repo.GetDestination(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetDestination error: %v", err)
	}
	if raw == nil || raw.SecretKey != "super-secret-value" {
		t.Fatalf("decrypted secret missing in repository layer: %+v", raw)
	}
	// And the stored ciphertext is not the plaintext even encrypted-at-rest.
	var storedEnc string
	if err := db.QueryRow("SELECT secret_enc FROM backup_destinations WHERE id = ?", created.ID).Scan(&storedEnc); err != nil {
		t.Fatalf("secret_enc read: %v", err)
	}
	if storedEnc == "super-secret-value" {
		t.Fatalf("secret stored as plaintext at rest")
	}

	if err := svc.DeleteDestination(ctx, created.ID); err != nil {
		t.Fatalf("DeleteDestination error: %v", err)
	}
	if dests, _ := svc.ListDestinations(ctx); len(dests) != 0 {
		t.Fatalf("destination not deleted: %d remain", len(dests))
	}
}

func TestCreateDestinationValidation(t *testing.T) {
	db, cleanup := setupTestStorageDB(t)
	defer cleanup()

	svc := NewService(NewSQLiteRepository(db))
	ctx := context.Background()

	cases := []struct {
		label string
		d     BackupDestination
	}{
		{"missing name", BackupDestination{Endpoint: "https://e", Bucket: "b", AccessKey: "a", SecretKey: "s"}},
		{"missing endpoint", BackupDestination{Name: "x", Bucket: "b", AccessKey: "a", SecretKey: "s"}},
		{"missing bucket", BackupDestination{Name: "x", Endpoint: "https://e", AccessKey: "a", SecretKey: "s"}},
		{"missing accessKey", BackupDestination{Name: "x", Endpoint: "https://e", Bucket: "b", SecretKey: "s"}},
		{"missing secretKey", BackupDestination{Name: "x", Endpoint: "https://e", Bucket: "b", AccessKey: "a"}},
		{"bad endpoint url", BackupDestination{Name: "x", Endpoint: "not-a-url", Bucket: "b", AccessKey: "a", SecretKey: "s"}},
	}
	for _, tc := range cases {
		if _, err := svc.CreateDestination(ctx, tc.d); err == nil {
			t.Fatalf("%s: expected validation error, got nil", tc.label)
		}
	}
}

func TestUpdateSnapshotRemoteRoundtrip(t *testing.T) {
	db, cleanup := setupTestStorageDB(t)
	defer cleanup()

	repo := NewSQLiteRepository(db)
	ctx := context.Background()

	_, err := db.Exec(`INSERT INTO volume_snapshots
		(id, project_id, volume_name, filename, status) VALUES ('snap-1', 'p', 'vol', 'f.tar.gz', 'completed')`)
	if err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}

	state := SnapshotRemoteState{
		Status:        "synced",
		Provider:      "s3",
		DestinationID: "dest-1",
		RemoteKey:     "prefix/vol/f.tar.gz",
		Checksum:      "abc123",
	}
	if err := repo.UpdateSnapshotRemote(ctx, "snap-1", state); err != nil {
		t.Fatalf("UpdateSnapshotRemote: %v", err)
	}

	snap, err := repo.GetSnapshot(ctx, "snap-1")
	if err != nil || snap == nil {
		t.Fatalf("GetSnapshot: %v (%v)", snap, err)
	}
	if snap.RemoteStatus != "synced" || snap.RemoteKey != state.RemoteKey || snap.Checksum != state.Checksum || snap.RemoteDestinationID != state.DestinationID {
		t.Fatalf("remote state roundtrip mismatch: %+v", snap)
	}

	// Failed push keeps the local row, marks the snapshot local-only.
	fail := SnapshotRemoteState{Status: "local-only", Provider: "s3", DestinationID: "dest-1", ErrText: "boom"}
	if err := repo.UpdateSnapshotRemote(ctx, "snap-1", fail); err != nil {
		t.Fatalf("UpdateSnapshotRemote failure path: %v", err)
	}
	if snap, _ = repo.GetSnapshot(ctx, "snap-1"); snap == nil || snap.RemoteStatus != "local-only" || snap.RemoteError != "boom" || snap.Filename != "f.tar.gz" {
		t.Fatalf("local-only state not recorded cleanly: %+v", snap)
	}
}
