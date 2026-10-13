package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDeleteQuadletFileCleanup(t *testing.T) {
	tempDir := t.TempDir()
	quadDir := filepath.Join(tempDir, "quadlets")
	_ = os.MkdirAll(quadDir, 0755)

	testFile := filepath.Join(quadDir, "test-workload.container")
	content := "[Unit]\nDescription=Test\n[Container]\nImage=alpine:latest\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Override search dir via env
	t.Setenv("QUADLET_DIR", quadDir)

	svc := NewService(nil)
	if err := svc.DeleteQuadlet(context.Background(), testFile); err != nil {
		t.Fatalf("DeleteQuadlet failed: %v", err)
	}

	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Errorf("expected testFile to be deleted, but still exists: %v", err)
	}
}
