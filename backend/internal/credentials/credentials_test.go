package credentials

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
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
	CREATE TABLE git_tokens (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		provider TEXT NOT NULL,
		username TEXT DEFAULT '',
		token TEXT DEFAULT '',
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

func TestGitTokenEncryptionAndMasking(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewSQLiteRepository(db)
	svc := NewService(repo)
	ctx := context.Background()

	created, err := svc.CreateGitToken(ctx, GitToken{
		Name:     "deploy token",
		Provider: "gitlab",
		Token:    "glpat-secret-value",
	})
	if err != nil {
		t.Fatalf("CreateGitToken error: %v", err)
	}
	if created.Token != "" {
		t.Fatal("created token must be masked in the response")
	}
	if created.Username != "oauth2" {
		t.Fatalf("expected oauth2 default username, got %q", created.Username)
	}

	var raw string
	if err := db.QueryRow("SELECT token FROM git_tokens WHERE id = ?", created.ID).Scan(&raw); err != nil {
		t.Fatalf("query raw token: %v", err)
	}
	if raw == "glpat-secret-value" || !strings.HasPrefix(raw, "enc:v1:") {
		t.Fatalf("token not encrypted at rest: %q", raw)
	}

	decrypted, err := svc.GetGitToken(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetGitToken error: %v", err)
	}
	if decrypted.Token != "glpat-secret-value" {
		t.Fatalf("decrypted token mismatch: %q", decrypted.Token)
	}

	list, err := svc.ListGitTokens(ctx)
	if err != nil {
		t.Fatalf("ListGitTokens error: %v", err)
	}
	if len(list) != 1 || list[0].Token != "" {
		t.Fatalf("list must mask tokens: %+v", list)
	}

	if _, err := svc.CreateGitToken(ctx, GitToken{Name: "empty", Provider: "github"}); err == nil {
		t.Fatal("token without a value must be rejected")
	}
	if _, err := svc.CreateGitToken(ctx, GitToken{Name: "bad", Provider: "svn", Token: "x"}); err == nil {
		t.Fatal("unsupported provider must be rejected")
	}
	if _, err := svc.CreateGitToken(ctx, GitToken{Name: "custom", Provider: "custom", Token: "x"}); err == nil {
		t.Fatal("custom provider without a username must be rejected")
	}

	if err := svc.DeleteGitToken(ctx, created.ID); err != nil {
		t.Fatalf("DeleteGitToken error: %v", err)
	}
	if _, err := svc.GetGitToken(ctx, created.ID); err == nil {
		t.Fatal("deleted token still resolves")
	}
}

func TestGetGitTokenFailsClosedOnDecryptError(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewSQLiteRepository(db)
	svc := NewService(repo)
	ctx := context.Background()

	created, err := svc.CreateGitToken(ctx, GitToken{
		Name:     "broken token",
		Provider: "github",
		Token:    "ghp-secret",
	})
	if err != nil {
		t.Fatalf("CreateGitToken error: %v", err)
	}
	if _, err := db.Exec("UPDATE git_tokens SET token = 'enc:v1:not-hex' WHERE id = ?", created.ID); err != nil {
		t.Fatalf("corrupt token: %v", err)
	}
	if _, err := svc.GetGitToken(ctx, created.ID); err == nil {
		t.Fatal("undecryptable token must not be returned as a usable credential")
	}

	// The listing endpoint still masks the value; it must not leak ciphertext.
	list, err := svc.ListGitTokens(ctx)
	if err != nil {
		t.Fatalf("ListGitTokens error: %v", err)
	}
	if len(list) != 1 || list[0].Token != "" {
		t.Fatalf("list leaked a token value: %+v", list)
	}
}
