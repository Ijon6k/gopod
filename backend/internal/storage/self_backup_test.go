package storage

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"gopod/pkg/crypto"
)

func TestCreateSystemBackup(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	_ = os.MkdirAll(dataDir, 0755)

	// Init master key
	if err := crypto.InitMasterKey(dataDir); err != nil {
		t.Fatalf("InitMasterKey: %v", err)
	}

	// Create test DB
	dbPath := filepath.Join(dataDir, "gopod.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	_, err = db.Exec("CREATE TABLE test_table (id INT, note TEXT); INSERT INTO test_table VALUES (1, 'system-test');")
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	// Create dummy caddy & quadlet config
	caddyDir := filepath.Join(dataDir, "caddy")
	_ = os.MkdirAll(caddyDir, 0755)
	_ = os.WriteFile(filepath.Join(caddyDir, "Caddyfile"), []byte(":80 { respond ok }"), 0644)

	// In-memory / test SQLite repository for snapshots
	snapDB, err := sql.Open("sqlite", filepath.Join(tmpDir, "meta.db"))
	if err != nil {
		t.Fatalf("open meta db: %v", err)
	}
	defer snapDB.Close()

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
		service_id TEXT DEFAULT '',
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
	);`
	if _, err := snapDB.Exec(schema); err != nil {
		t.Fatalf("meta schema: %v", err)
	}

	repo := NewSQLiteRepository(snapDB)
	svc := NewService(repo)
	svc.SetSystemSource(db, dataDir)

	// 1. Key Status Check
	keyStatus := svc.GetKeyStatus()
	if !keyStatus.KeyExists {
		t.Errorf("expected KeyExists=true, got false")
	}
	if keyStatus.Fingerprint == "" {
		t.Errorf("expected non-empty key fingerprint")
	}

	// 2. Execute System Backup
	ctx := context.Background()
	snap, err := svc.CreateSystemBackup(ctx, "")
	if err != nil {
		t.Fatalf("CreateSystemBackup: %v", err)
	}
	if snap.VolumeName != "system:gopod" {
		t.Errorf("expected VolumeName=system:gopod, got %s", snap.VolumeName)
	}
	if snap.Status != "completed" {
		t.Errorf("expected Status=completed, got %s", snap.Status)
	}

	// 2b. The archive bundles master.key and the whole database; verify it is
	// written owner-only (mode 0600).
	if fi, statErr := os.Stat(filepath.Join(dataDir, "backups", snap.Filename)); statErr == nil {
		if got := fi.Mode().Perm(); got != 0600 {
			t.Errorf("expected archive mode 0600, got %v", got)
		}
	} else {
		t.Fatalf("archive missing on disk: %v", statErr)
	}

	// 3. Verify archive file structure
	archivePath, _, err := svc.GetSnapshotFilePath(ctx, snap.ID)
	if err != nil {
		t.Fatalf("GetSnapshotFilePath: %v", err)
	}
	defer os.Remove(archivePath)

	f, err := os.Open(archivePath)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	foundDB := false
	foundKey := false
	foundCaddy := false
	foundManifest := false

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar next: %v", err)
		}
		if hdr.Name == "gopod.db" {
			foundDB = true
		}
		if strings.Contains(hdr.Name, "master.key") {
			foundKey = true
		}
		if strings.Contains(hdr.Name, "Caddyfile") {
			foundCaddy = true
		}
		if hdr.Name == "gopod-manifest.json" {
			foundManifest = true
		}
	}

	if !foundDB {
		t.Errorf("archive missing gopod.db")
	}
	if !foundKey {
		t.Errorf("archive missing keys/master.key")
	}
	if !foundCaddy {
		t.Errorf("archive missing Caddyfile")
	}
	if !foundManifest {
		t.Errorf("archive missing gopod-manifest.json")
	}

	// 4. Test Schedule Management
	if _, err := svc.SaveSystemBackupSchedule(ctx, "invalid-cron-syntax", 7, "", true); err == nil {
		t.Errorf("expected error for invalid cron syntax, got nil")
	}

	sched, err := svc.SaveSystemBackupSchedule(ctx, "0 4 * * *", 14, "", true)
	if err != nil {
		t.Fatalf("SaveSystemBackupSchedule: %v", err)
	}
	if sched.Cron != "0 4 * * *" || sched.RetentionCount != 14 {
		t.Errorf("unexpected schedule: %+v", sched)
	}

	gotSched, err := svc.GetSystemBackupSchedule(ctx)
	if err != nil || gotSched == nil || gotSched.RetentionCount != 14 {
		t.Errorf("failed to retrieve saved schedule: %+v, err: %v", gotSched, err)
	}
}

func TestManualSystemBackupAppliesRetention(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	_ = os.MkdirAll(dataDir, 0755)

	if err := crypto.InitMasterKey(dataDir); err != nil {
		t.Fatalf("InitMasterKey: %v", err)
	}

	dbPath := filepath.Join(dataDir, "gopod.db")
	sourceDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer sourceDB.Close()
	_, _ = sourceDB.Exec("CREATE TABLE t (id INT);")

	snapDB, err := sql.Open("sqlite", filepath.Join(tmpDir, "meta.db"))
	if err != nil {
		t.Fatalf("open meta db: %v", err)
	}
	defer snapDB.Close()
	if _, err := snapDB.Exec(`
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
		service_id TEXT DEFAULT '',
		volume_name TEXT NOT NULL,
		cron TEXT NOT NULL,
		label TEXT DEFAULT '',
		retention_count INTEGER DEFAULT 7,
		enabled BOOLEAN DEFAULT 1,
		last_run DATETIME,
		destination_id TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`); err != nil {
		t.Fatalf("meta schema: %v", err)
	}

	repo := NewSQLiteRepository(snapDB)
	svc := NewService(repo)
	svc.SetSystemSource(sourceDB, dataDir)
	ctx := context.Background()

	// Manual backups are pruned against the enabled schedule's policy.
	if _, err := svc.SaveSystemBackupSchedule(ctx, "0 3 * * *", 2, "", true); err != nil {
		t.Fatalf("SaveSystemBackupSchedule: %v", err)
	}
	for i := 0; i < 3; i++ {
		if _, err := svc.CreateSystemBackup(ctx, ""); err != nil {
			t.Fatalf("CreateSystemBackup #%d: %v", i, err)
		}
	}
	kept, err := svc.ListSystemBackups(ctx)
	if err != nil {
		t.Fatalf("ListSystemBackups: %v", err)
	}
	if len(kept) != 2 {
		t.Fatalf("retention on manual backups: expected 2 kept snapshots, got %d", len(kept))
	}
}

func TestDeleteMissingSnapshotReportsNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	dbPath := filepath.Join(dataDir, "gopod.db")
	sourceDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer sourceDB.Close()

	snapDB, err := sql.Open("sqlite", filepath.Join(tmpDir, "meta.db"))
	if err != nil {
		t.Fatalf("open meta db: %v", err)
	}
	defer snapDB.Close()
	if _, err := snapDB.Exec(`CREATE TABLE volume_snapshots (
		id TEXT PRIMARY KEY, project_id TEXT NOT NULL, service_id TEXT DEFAULT '',
		volume_name TEXT NOT NULL, filename TEXT NOT NULL, size TEXT DEFAULT '',
		size_bytes INTEGER DEFAULT 0, status TEXT DEFAULT 'completed',
		compression TEXT DEFAULT 'gzip', remote_provider TEXT DEFAULT '',
		remote_destination_id TEXT DEFAULT '', remote_key TEXT DEFAULT '',
		checksum TEXT DEFAULT '', remote_status TEXT DEFAULT '',
		remote_error TEXT DEFAULT '', created_at DATETIME DEFAULT CURRENT_TIMESTAMP);`); err != nil {
		t.Fatalf("meta schema: %v", err)
	}

	svc := NewService(NewSQLiteRepository(snapDB))
	svc.SetSystemSource(sourceDB, dataDir)

	err = svc.DeleteSnapshot(context.Background(), "does-not-exist")
	if err == nil || !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("expected ErrSnapshotNotFound, got %v", err)
	}
}

// newVerifyFixture wires a storage service against a throwaway data dir with
// the full snapshot metadata schema, ready for CreateSystemBackup/Verify
// round-trips. Returns the service and the data directory.
func newVerifyFixture(t *testing.T) (*Service, string) {
	t.Helper()
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("mkdir data dir: %v", err)
	}
	if err := crypto.InitMasterKey(dataDir); err != nil {
		t.Fatalf("InitMasterKey: %v", err)
	}

	sourceDB, err := sql.Open("sqlite", filepath.Join(dataDir, "gopod.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { sourceDB.Close() })
	if _, err := sourceDB.Exec("CREATE TABLE probe (id INT, note TEXT); INSERT INTO probe VALUES (1, 'verify-me');"); err != nil {
		t.Fatalf("seed db: %v", err)
	}

	snapDB, err := sql.Open("sqlite", filepath.Join(tmpDir, "meta.db"))
	if err != nil {
		t.Fatalf("open meta db: %v", err)
	}
	t.Cleanup(func() { snapDB.Close() })
	if _, err := snapDB.Exec(`CREATE TABLE volume_snapshots (
		id TEXT PRIMARY KEY, project_id TEXT NOT NULL, service_id TEXT DEFAULT '',
		volume_name TEXT NOT NULL, filename TEXT NOT NULL, size TEXT DEFAULT '',
		size_bytes INTEGER DEFAULT 0, status TEXT DEFAULT 'completed',
		compression TEXT DEFAULT 'gzip', remote_provider TEXT DEFAULT '',
		remote_destination_id TEXT DEFAULT '', remote_key TEXT DEFAULT '',
		checksum TEXT DEFAULT '', remote_status TEXT DEFAULT '',
		remote_error TEXT DEFAULT '', created_at DATETIME DEFAULT CURRENT_TIMESTAMP);`); err != nil {
		t.Fatalf("meta schema: %v", err)
	}

	svc := NewService(NewSQLiteRepository(snapDB))
	svc.SetSystemSource(sourceDB, dataDir)
	return svc, dataDir
}

func archiveChecksum(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}

func setStoredChecksum(t *testing.T, svc *Service, id, checksum string) {
	t.Helper()
	err := svc.repo.UpdateSnapshotRemote(context.Background(), id, SnapshotRemoteState{
		Status: "synced", Provider: "s3", RemoteKey: "mirror/key", Checksum: checksum,
	})
	if err != nil {
		t.Fatalf("set stored checksum: %v", err)
	}
}

func TestVerifySystemBackupValidArchive(t *testing.T) {
	svc, dataDir := newVerifyFixture(t)
	ctx := context.Background()

	snap, err := svc.CreateSystemBackup(ctx, "")
	if err != nil {
		t.Fatalf("CreateSystemBackup: %v", err)
	}

	report, err := svc.VerifySystemBackup(ctx, snap.ID)
	if err != nil {
		t.Fatalf("VerifySystemBackup: %v", err)
	}
	if !report.Valid {
		t.Fatalf("expected valid report, got errors=%v warnings=%v", report.Errors, report.Warnings)
	}
	if !report.ArchiveReadable || !report.DatabaseFound || !report.ManifestFound {
		t.Fatalf("expected readable+db+manifest, got %+v", report)
	}
	if !report.KeyBundleFound {
		t.Fatalf("expected bundled key detection for file-based key setups")
	}
	if report.ManifestKeyFromEnv {
		t.Fatalf("manifest must report file-based key (keyFromEnv=false)")
	}
	if report.DatabaseIntegrity != "ok" {
		t.Fatalf("expected integrity ok, got %q", report.DatabaseIntegrity)
	}
	if report.ChecksumMatch != "unavailable" {
		t.Fatalf("expected checksum unavailable without mirror, got %q", report.ChecksumMatch)
	}
	if len(report.Warnings) == 0 {
		t.Fatalf("expected the missing-checksum warning")
	}
	entries := strings.Join(report.Entries, ",")
	for _, want := range []string{"gopod.db", "gopod-manifest.json", "keys/master.key"} {
		if !strings.Contains(entries, want) {
			t.Fatalf("expected %q in entries, got %s", want, entries)
		}
	}
	if _, err := os.Stat(filepath.Join(dataDir, "backups", snap.Filename)); err != nil {
		t.Fatalf("archive must survive verification: %v", err)
	}
}

func TestVerifySystemBackupChecksumMatchAndMismatch(t *testing.T) {
	svc, dataDir := newVerifyFixture(t)
	ctx := context.Background()

	snap, err := svc.CreateSystemBackup(ctx, "")
	if err != nil {
		t.Fatalf("CreateSystemBackup: %v", err)
	}
	good := archiveChecksum(t, filepath.Join(dataDir, "backups", snap.Filename))

	setStoredChecksum(t, svc, snap.ID, good)
	report, err := svc.VerifySystemBackup(ctx, snap.ID)
	if err != nil {
		t.Fatalf("VerifySystemBackup: %v", err)
	}
	if !report.Valid || report.ChecksumMatch != "match" {
		t.Fatalf("expected valid report with matching checksum, got valid=%v match=%q errors=%v",
			report.Valid, report.ChecksumMatch, report.Errors)
	}

	setStoredChecksum(t, svc, snap.ID, "deadbeefdeadbeef")
	report, err = svc.VerifySystemBackup(ctx, snap.ID)
	if err != nil {
		t.Fatalf("VerifySystemBackup: %v", err)
	}
	if report.Valid || report.ChecksumMatch != "mismatch" {
		t.Fatalf("expected invalid report with mismatching checksum, got valid=%v match=%q",
			report.Valid, report.ChecksumMatch)
	}
	if len(report.Errors) == 0 || !strings.Contains(strings.Join(report.Errors, " "), "Checksum mismatch") {
		t.Fatalf("expected mismatch error, got %v", report.Errors)
	}
}

func TestVerifySystemBackupCorruptArchive(t *testing.T) {
	svc, dataDir := newVerifyFixture(t)
	ctx := context.Background()

	snap, err := svc.CreateSystemBackup(ctx, "")
	if err != nil {
		t.Fatalf("CreateSystemBackup: %v", err)
	}
	archivePath := filepath.Join(dataDir, "backups", snap.Filename)
	if err := os.WriteFile(archivePath, []byte("definitely-not-a-gzip-stream"), 0600); err != nil {
		t.Fatalf("corrupt archive: %v", err)
	}

	report, err := svc.VerifySystemBackup(ctx, snap.ID)
	if err != nil {
		t.Fatalf("VerifySystemBackup: %v", err)
	}
	if report.Valid || report.ArchiveReadable {
		t.Fatalf("expected invalid report for garbage archive, got %+v", report)
	}
	if len(report.Errors) == 0 {
		t.Fatalf("expected a structural error, got none")
	}
}

func TestVerifySystemBackupMissingKeyRule(t *testing.T) {
	svc, dataDir := newVerifyFixture(t)
	ctx := context.Background()

	// Craft an archive that carries a healthy database and a manifest
	// declaring a file-based key, but no bundled key.
	staging := filepath.Join(t.TempDir(), "stage")
	if err := os.MkdirAll(staging, 0700); err != nil {
		t.Fatalf("mkdir staging: %v", err)
	}
	srcDB, err := sql.Open("sqlite", filepath.Join(staging, "gopod.db"))
	if err != nil {
		t.Fatalf("open packaged db: %v", err)
	}
	if _, err := srcDB.Exec("CREATE TABLE t (id INT)"); err != nil {
		t.Fatalf("seed packaged db: %v", err)
	}
	srcDB.Close()

	manifest := `{"version": 1, "createdAt": "2026-01-01T00:00:00Z", "keyFromEnv": false, "keyFingerprint": "sha256:0000000000000000"}`
	if err := os.WriteFile(filepath.Join(staging, "gopod-manifest.json"), []byte(manifest), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	archivePath := filepath.Join(dataDir, "backups", "crafted-no-key.tar.gz")
	if err := os.MkdirAll(filepath.Dir(archivePath), 0700); err != nil {
		t.Fatalf("mkdir backups dir: %v", err)
	}
	if err := createTarGz(staging, archivePath); err != nil {
		t.Fatalf("craft archive: %v", err)
	}
	if err := svc.repo.CreateSnapshot(ctx, VolumeSnapshot{
		ID: "sys-craftednokey", ProjectID: "system", VolumeName: "system:gopod",
		Filename: "crafted-no-key.tar.gz", Status: "completed", Compression: "gzip",
		CreatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("record crafted snapshot: %v", err)
	}

	report, err := svc.VerifySystemBackup(ctx, "sys-craftednokey")
	if err != nil {
		t.Fatalf("VerifySystemBackup: %v", err)
	}
	if report.Valid {
		t.Fatalf("expected invalid report for a keyless archive, got %+v", report)
	}
	if report.DatabaseIntegrity != "ok" || !report.ManifestFound {
		t.Fatalf("db and manifest should be healthy, got integrity=%q manifest=%v", report.DatabaseIntegrity, report.ManifestFound)
	}
	if !strings.Contains(strings.Join(report.Errors, " "), "keys/master.key is missing") {
		t.Fatalf("expected the key-presence error, got %v", report.Errors)
	}
}

func TestVerifySystemBackupRejectsForeignSnapshot(t *testing.T) {
	svc, _ := newVerifyFixture(t)

	if _, err := svc.VerifySystemBackup(context.Background(), "no-such-id"); !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("expected ErrSnapshotNotFound for unknown id, got %v", err)
	}
}
