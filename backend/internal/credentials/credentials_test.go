package credentials

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"gopod/pkg/crypto"
	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) (*sql.DB, func()) {
	tmpDir, err := os.MkdirTemp("", "gopod-cred-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}

	schema := `
	CREATE TABLE ssh_keys (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		public_key TEXT NOT NULL,
		private_key TEXT DEFAULT '',
		fingerprint TEXT DEFAULT '',
		type TEXT DEFAULT 'ed25519',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE container_registries (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		url TEXT NOT NULL,
		username TEXT NOT NULL,
		token TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE secrets (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		value TEXT DEFAULT '',
		driver TEXT DEFAULT 'file',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	_ = crypto.InitMasterKey(tmpDir)

	cleanup := func() {
		_ = db.Close()
		_ = os.RemoveAll(tmpDir)
	}

	return db, cleanup
}

func TestCredentialsEncryptionAndRetrieval(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewSQLiteRepository(db)
	svc := NewService(repo)
	ctx := context.Background()

	// 1. Test SSH Key creation & retrieval
	key, err := svc.CreateSSHKey(ctx, SSHKey{
		Name:       "test-deploy-key",
		PrivateKey: "-----BEGIN PRIVATE KEY-----\nMIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQC...\n-----END PRIVATE KEY-----",
		PublicKey:  "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI... test",
	})
	if err != nil {
		t.Fatalf("CreateSSHKey error: %v", err)
	}

	// In response, private key must be masked
	if key.PrivateKey != "" {
		t.Fatalf("Expected masked private key in return, got: %s", key.PrivateKey)
	}

	// Verify database stores encrypted content with prefix enc:v1:
	var rawEncKey string
	err = db.QueryRow("SELECT private_key FROM ssh_keys WHERE id = ?", key.ID).Scan(&rawEncKey)
	if err != nil {
		t.Fatalf("failed to query raw private key: %v", err)
	}
	if len(rawEncKey) < 8 || rawEncKey[:7] != "enc:v1:" {
		t.Fatalf("Expected encrypted key to start with enc:v1:, got: %s", rawEncKey)
	}

	// Service GetPrivateKey returns decrypted key
	privKey, err := svc.GetPrivateKey(ctx, key.ID)
	if err != nil {
		t.Fatalf("GetPrivateKey error: %v", err)
	}
	if privKey != "-----BEGIN PRIVATE KEY-----\nMIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQC...\n-----END PRIVATE KEY-----" {
		t.Fatalf("Decrypted private key mismatch")
	}
}
