package services

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopod/internal/db"
)

func createTestZip(t *testing.T, files map[string][]byte) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("failed to create zip entry %s: %v", name, err)
		}
		if _, err := w.Write(content); err != nil {
			t.Fatalf("failed to write zip entry %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}
	return buf.Bytes()
}

func makeUploadRequest(t *testing.T, urlPath, fieldName, filename string, content []byte) *http.Request {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	if _, err := io.Copy(fw, bytes.NewReader(content)); err != nil {
		t.Fatalf("failed to copy content: %v", err)
	}
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, urlPath, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func setupDropTest(t *testing.T) (*Handler, string, Service) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	_, err = database.DB.Exec("INSERT INTO projects (id, name) VALUES ('proj-1', 'Test Project')")
	if err != nil {
		t.Fatalf("failed to insert test project: %v", err)
	}

	repo := NewSQLiteRepository(database.DB)
	deployer := &Deployer{dataDir: tempDir}
	ws := NewWorkloadService(repo, deployer, nil)
	handler := NewHandler(ws, nil)

	svc := Service{
		ID:        "svc-test-drop",
		ProjectID: "proj-1",
		Name:      "test-drop",
		Status:    "stopped",
	}
	if err := repo.Create(context.Background(), svc); err != nil {
		t.Fatalf("failed to create test service: %v", err)
	}

	return handler, tempDir, svc
}

func TestDropUploadValid(t *testing.T) {
	handler, tempDir, svc := setupDropTest(t)

	zipBytes := createTestZip(t, map[string][]byte{
		"index.html": []byte("<h1>Hello GoPod</h1>"),
		"style.css":  []byte("body { color: red; }"),
	})

	req := makeUploadRequest(t, "/api/services/"+svc.ID+"/drop-upload", "file", "site.zip", zipBytes)
	req.SetPathValue("id", svc.ID)

	w := httptest.NewRecorder()
	handler.handleDropUpload(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", w.Code, w.Body.String())
	}

	distDir := filepath.Join(tempDir, "static", svc.ProjectID, svc.ID, "dist")
	indexFile := filepath.Join(distDir, "index.html")
	content, err := os.ReadFile(indexFile)
	if err != nil {
		t.Fatalf("index.html not found: %v", err)
	}
	if string(content) != "<h1>Hello GoPod</h1>" {
		t.Fatalf("unexpected content: %s", string(content))
	}
}

func TestDropUploadZipSlipRejected(t *testing.T) {
	handler, tempDir, svc := setupDropTest(t)

	zipBytes := createTestZip(t, map[string][]byte{
		"../escape.txt": []byte("evil payload"),
	})

	req := makeUploadRequest(t, "/api/services/"+svc.ID+"/drop-upload", "file", "slip.zip", zipBytes)
	req.SetPathValue("id", svc.ID)

	w := httptest.NewRecorder()
	handler.handleDropUpload(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400 for Zip Slip, got %d: %s", w.Code, w.Body.String())
	}

	escapedFile := filepath.Join(tempDir, "static", svc.ProjectID, svc.ID, "escape.txt")
	if _, err := os.Stat(escapedFile); !os.IsNotExist(err) {
		t.Fatalf("zip slip file must not exist on disk: %s", escapedFile)
	}
}

func TestDropUploadZipBombRejected(t *testing.T) {
	t.Setenv("GOPOD_MAX_UPLOAD_MB", "1") // 1 MB upload cap -> max total uncompressed = 10 MB

	handler, tempDir, svc := setupDropTest(t)

	// Create a zip with 12 MB of uncompressed zeroes. It compresses to only ~12 KB.
	hugeData := make([]byte, 12<<20)
	zipBytes := createTestZip(t, map[string][]byte{
		"bigfile.bin": hugeData,
	})

	req := makeUploadRequest(t, "/api/services/"+svc.ID+"/drop-upload", "file", "bomb.zip", zipBytes)
	req.SetPathValue("id", svc.ID)

	w := httptest.NewRecorder()
	handler.handleDropUpload(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected HTTP 413 Request Entity Too Large, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "zip bomb protection") {
		t.Fatalf("expected response to mention zip bomb protection, got: %s", w.Body.String())
	}

	distDir := filepath.Join(tempDir, "static", svc.ProjectID, svc.ID, "dist")
	bigFile := filepath.Join(distDir, "bigfile.bin")
	if _, err := os.Stat(bigFile); !os.IsNotExist(err) {
		t.Fatalf("half-extracted file must be purged on zip bomb detection: %s", bigFile)
	}
}

func TestDropUploadSingleFile(t *testing.T) {
	handler, tempDir, svc := setupDropTest(t)

	zipBytes := createTestZip(t, map[string][]byte{
		"index.html": []byte("<h1>Single File</h1>"),
	})

	req := makeUploadRequest(t, "/api/services/"+svc.ID+"/drop-upload", "file", "single.zip", zipBytes)
	req.SetPathValue("id", svc.ID)

	w := httptest.NewRecorder()
	handler.handleDropUpload(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", w.Code, w.Body.String())
	}

	distDir := filepath.Join(tempDir, "static", svc.ProjectID, svc.ID, "dist")
	indexFile := filepath.Join(distDir, "index.html")
	content, err := os.ReadFile(indexFile)
	if err != nil {
		t.Fatalf("index.html not found: %v", err)
	}
	if string(content) != "<h1>Single File</h1>" {
		t.Fatalf("unexpected content: %s", string(content))
	}
}

func TestDropUploadWrappedSingleFolder(t *testing.T) {
	handler, tempDir, svc := setupDropTest(t)

	zipBytes := createTestZip(t, map[string][]byte{
		"build/index.html":  []byte("<h1>Nested in build</h1>"),
		"build/css/app.css": []byte("body { background: black; }"),
	})

	req := makeUploadRequest(t, "/api/services/"+svc.ID+"/drop-upload", "file", "build.zip", zipBytes)
	req.SetPathValue("id", svc.ID)

	w := httptest.NewRecorder()
	handler.handleDropUpload(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", w.Code, w.Body.String())
	}

	distDir := filepath.Join(tempDir, "static", svc.ProjectID, svc.ID, "dist")
	// Single root folder "build" must be unwrapped
	indexFile := filepath.Join(distDir, "index.html")
	content, err := os.ReadFile(indexFile)
	if err != nil {
		t.Fatalf("unwrapped index.html not found: %v", err)
	}
	if string(content) != "<h1>Nested in build</h1>" {
		t.Fatalf("unexpected content: %s", string(content))
	}

	cssFile := filepath.Join(distDir, "css", "app.css")
	if _, err := os.Stat(cssFile); err != nil {
		t.Fatalf("unwrapped css/app.css not found: %v", err)
	}
}

func TestDropUploadEmptyZipRejected(t *testing.T) {
	handler, _, svc := setupDropTest(t)

	zipBytes := createTestZip(t, map[string][]byte{})

	req := makeUploadRequest(t, "/api/services/"+svc.ID+"/drop-upload", "file", "empty.zip", zipBytes)
	req.SetPathValue("id", svc.ID)

	w := httptest.NewRecorder()
	handler.handleDropUpload(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400 for empty zip, got %d: %s", w.Code, w.Body.String())
	}
}
