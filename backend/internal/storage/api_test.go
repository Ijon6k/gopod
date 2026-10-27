package storage

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	_ "modernc.org/sqlite"

	"gopod/internal/auth"
	"gopod/pkg/crypto"
)

type mockAuditRecorder struct {
	mu      sync.Mutex
	records []string
}

func (m *mockAuditRecorder) Record(ctx context.Context, action, actor, target, category, ip, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records = append(m.records, action+":"+status)
	return nil
}

func (m *mockAuditRecorder) Has(action, status string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	target := action + ":" + status
	for _, r := range m.records {
		if r == target {
			return true
		}
	}
	return false
}

func setupStorageHTTPTest(t *testing.T) (*Handler, *Service, *mockAuditRecorder, *http.Cookie) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	_ = os.MkdirAll(dataDir, 0755)

	// Master key
	if err := crypto.InitMasterKey(dataDir); err != nil {
		t.Fatalf("InitMasterKey: %v", err)
	}

	// SQLite DB for app state
	dbPath := filepath.Join(dataDir, "gopod.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	_, _ = db.Exec("CREATE TABLE test_data (id INT, name TEXT); INSERT INTO test_data VALUES (1, 'live-gopod');")

	// Dummy configs
	caddyDir := filepath.Join(dataDir, "caddy")
	_ = os.MkdirAll(caddyDir, 0755)
	_ = os.WriteFile(filepath.Join(caddyDir, "Caddyfile"), []byte(":80 { respond ok }"), 0644)

	// SQLite Repository for storage
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
	);
	CREATE TABLE users (
		id TEXT PRIMARY KEY,
		email TEXT UNIQUE,
		password_hash TEXT,
		name TEXT,
		role TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE sessions (
		token TEXT PRIMARY KEY,
		user_id TEXT,
		expires_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("schema: %v", err)
	}

	repo := NewSQLiteRepository(db)
	svc := NewService(repo)
	svc.SetSystemSource(db, dataDir)

	mockAudit := &mockAuditRecorder{}
	svc.SetAuditRecorder(mockAudit)

	// Setup auth middleware with mock admin session
	authRepo := auth.NewSQLiteRepository(db)
	authSvc := auth.NewService(authRepo)
	adminUser, token, err := authSvc.SetupAdmin(context.Background(), "Admin", "admin@gopod.test", "adminpass123")
	if err != nil {
		t.Fatalf("SetupAdmin: %v", err)
	}
	_ = adminUser

	authMW := auth.NewMiddleware(authSvc)
	handler := NewHandler(svc, authMW)

	cookie := &http.Cookie{
		Name:  "gopod_session",
		Value: token,
		Path:  "/",
	}

	return handler, svc, mockAudit, cookie
}

func TestStorageHTTPAPIEndpoints(t *testing.T) {
	handler, svc, mockAudit, cookie := setupStorageHTTPTest(t)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// 1. GET /api/system/security/key-status without auth -> 401
	req := httptest.NewRequest("GET", "/api/system/security/key-status", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", w.Code)
	}

	// 2. GET /api/system/security/key-status with auth -> 200
	req = httptest.NewRequest("GET", "/api/system/security/key-status", nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for key-status, got %d: %s", w.Code, w.Body.String())
	}
	var keyResp KeyStatusResponse
	if err := json.Unmarshal(w.Body.Bytes(), &keyResp); err != nil {
		t.Fatalf("unmarshal key status: %v", err)
	}
	if !keyResp.KeyExists || keyResp.Fingerprint == "" || keyResp.Algorithm != "AES-256-GCM" {
		t.Fatalf("unexpected key status payload: %+v", keyResp)
	}

	// 3. POST /api/system/backup (Trigger Self-Backup via HTTP) -> 201 Created
	backupReqBody, _ := json.Marshal(map[string]string{"destinationId": ""})
	req = httptest.NewRequest("POST", "/api/system/backup", bytes.NewReader(backupReqBody))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 for system backup, got %d: %s", w.Code, w.Body.String())
	}
	var createdSnap VolumeSnapshot
	if err := json.Unmarshal(w.Body.Bytes(), &createdSnap); err != nil {
		t.Fatalf("unmarshal created snapshot: %v", err)
	}
	if createdSnap.VolumeName != "system:gopod" || createdSnap.Status != "completed" {
		t.Fatalf("unexpected created snapshot: %+v", createdSnap)
	}

	// Check audit log recorded backup action
	if !mockAudit.Has("system.backup", "success") {
		t.Errorf("expected audit record 'system.backup:success' to be recorded")
	}

	// 4. GET /api/system/backups (List backups via HTTP) -> 200 OK
	req = httptest.NewRequest("GET", "/api/system/backups", nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for list backups, got %d", w.Code)
	}
	var backupList []VolumeSnapshot
	if err := json.Unmarshal(w.Body.Bytes(), &backupList); err != nil {
		t.Fatalf("unmarshal backup list: %v", err)
	}
	if len(backupList) != 1 || backupList[0].ID != createdSnap.ID {
		t.Fatalf("expected 1 backup with ID %s, got %+v", createdSnap.ID, backupList)
	}

	// 5. GET /api/storage/snapshots/{id}/download (Download Tarball via HTTP) -> 200 OK
	req = httptest.NewRequest("GET", "/api/storage/snapshots/"+createdSnap.ID+"/download", nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for download, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the downloaded stream is a valid tar.gz containing db, key, manifest
	gr, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatalf("invalid gzip download stream: %v", err)
	}
	tr := tar.NewReader(gr)
	hasDB, hasKey, hasManifest := false, false, false
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar next: %v", err)
		}
		if hdr.Name == "gopod.db" {
			hasDB = true
		}
		if strings.Contains(hdr.Name, "master.key") {
			hasKey = true
		}
		if hdr.Name == "gopod-manifest.json" {
			hasManifest = true
		}
	}
	if !hasDB || !hasKey || !hasManifest {
		t.Fatalf("downloaded archive missing critical files (db=%v, key=%v, manifest=%v)", hasDB, hasKey, hasManifest)
	}

	// 6. POST /api/system/backup/schedule (Invalid cron syntax) -> 400 Bad Request
	badSchedBody, _ := json.Marshal(map[string]interface{}{
		"cron":           "invalid-cron-format",
		"retentionCount": 5,
		"enabled":        true,
	})
	req = httptest.NewRequest("POST", "/api/system/backup/schedule", bytes.NewReader(badSchedBody))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid cron, got %d", w.Code)
	}

	// 7. POST /api/system/backup/schedule (Valid cron) -> 200 OK
	goodSchedBody, _ := json.Marshal(map[string]interface{}{
		"cron":           "0 3 * * *",
		"retentionCount": 10,
		"enabled":        true,
	})
	req = httptest.NewRequest("POST", "/api/system/backup/schedule", bytes.NewReader(goodSchedBody))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for valid schedule, got %d: %s", w.Code, w.Body.String())
	}
	var savedSched VolumeSchedule
	if err := json.Unmarshal(w.Body.Bytes(), &savedSched); err != nil {
		t.Fatalf("unmarshal schedule: %v", err)
	}
	if savedSched.Cron != "0 3 * * *" || savedSched.RetentionCount != 10 || !savedSched.Enabled {
		t.Fatalf("unexpected saved schedule: %+v", savedSched)
	}
	if !mockAudit.Has("system.backup_schedule", "success") {
		t.Errorf("expected audit record 'system.backup_schedule:success' to be recorded")
	}

	// 8. GET /api/system/backup/schedule -> 200 OK
	req = httptest.NewRequest("GET", "/api/system/backup/schedule", nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for get schedule, got %d", w.Code)
	}

	// 9. POST /api/storage/snapshots/{id}/restore on self-backup -> 400 error rejection
	req = httptest.NewRequest("POST", "/api/storage/snapshots/"+createdSnap.ID+"/restore", nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "instance disaster recovery archives cannot be restored") {
		t.Fatalf("expected restore rejection for instance archive, got code %d: %s", w.Code, w.Body.String())
	}

	// 10. DELETE /api/storage/snapshots/{id} -> 200 OK
	req = httptest.NewRequest("DELETE", "/api/storage/snapshots/"+createdSnap.ID, nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for delete snapshot, got %d: %s", w.Code, w.Body.String())
	}
	if !mockAudit.Has("system.backup_delete", "success") {
		t.Errorf("expected audit record 'system.backup_delete:success' to be recorded")
	}

	// Verify file is actually deleted from disk
	_, _, err = svc.GetSnapshotFilePath(context.Background(), createdSnap.ID)
	if err == nil {
		t.Fatalf("expected error getting deleted snapshot file, got nil")
	}
}

func TestStorageHTTPVerifyEndpoint(t *testing.T) {
	handler, svc, _, cookie := setupStorageHTTPTest(t)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	ctx := context.Background()

	// 1. Unauthenticated verify -> 401
	req := httptest.NewRequest("GET", "/api/system/backups/no-such/verify", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated verify, got %d", w.Code)
	}

	// 2. Unknown snapshot id -> 404
	req = httptest.NewRequest("GET", "/api/system/backups/no-such/verify", nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown verify id, got %d: %s", w.Code, w.Body.String())
	}

	// 3. Verify a real archive -> 200 with a valid report
	created, err := svc.CreateSystemBackup(ctx, "")
	if err != nil {
		t.Fatalf("CreateSystemBackup: %v", err)
	}
	req = httptest.NewRequest("GET", "/api/system/backups/"+created.ID+"/verify", nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for verify, got %d: %s", w.Code, w.Body.String())
	}
	var report struct {
		Valid             bool     `json:"valid"`
		ManifestFound     bool     `json:"manifestFound"`
		KeyBundleFound    bool     `json:"keyBundleFound"`
		DatabaseFound     bool     `json:"databaseFound"`
		DatabaseIntegrity string   `json:"databaseIntegrity"`
		ChecksumMatch     string   `json:"checksumMatch"`
		Entries           []string `json:"entries"`
		Errors            []string `json:"errors"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &report); err != nil {
		t.Fatalf("unmarshal verification report: %v", err)
	}
	if !report.Valid {
		t.Fatalf("expected valid report over HTTP, got errors=%v", report.Errors)
	}
	if !report.ManifestFound || !report.KeyBundleFound || !report.DatabaseFound || report.DatabaseIntegrity != "ok" {
		t.Fatalf("unexpected report payload: %+v", report)
	}
	if report.ChecksumMatch != "unavailable" {
		t.Fatalf("expected checksumMatch unavailable without mirror, got %q", report.ChecksumMatch)
	}
}
