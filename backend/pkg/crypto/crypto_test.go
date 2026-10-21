package crypto

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	if err := InitMasterKey(t.TempDir()); err != nil {
		t.Fatalf("InitMasterKey error: %v", err)
	}

	plain := "my-secret-ssh-key-value"
	enc, err := Encrypt(plain)
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}

	if enc == plain {
		t.Fatalf("Expected ciphertext to differ from plaintext")
	}

	dec, err := Decrypt(enc)
	if err != nil {
		t.Fatalf("Decrypt error: %v", err)
	}

	if dec != plain {
		t.Fatalf("Decrypted mismatch. Got: %s, Want: %s", dec, plain)
	}

	// Test backward compatibility with unencrypted string
	rawPlain := "legacy-unencrypted-key"
	decRaw, err := Decrypt(rawPlain)
	if err != nil {
		t.Fatalf("Legacy decrypt error: %v", err)
	}
	if decRaw != rawPlain {
		t.Fatalf("Legacy decrypt mismatch. Got: %s, Want: %s", decRaw, rawPlain)
	}
}

// TestInitMasterKeyFailsWhenKeyCannotPersist pins the A3 guarantee: a key that
// cannot be written to disk must fail initialization instead of falling back to
// an in-memory key (which would silently make stored secrets undecryptable on
// the next restart). A regular file placed where the keys directory must be
// created blocks persistence deterministically, without chmod tricks.
func TestInitMasterKeyFailsWhenKeyCannotPersist(t *testing.T) {
	dataDir := t.TempDir()
	blocker := filepath.Join(dataDir, "keys")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0600); err != nil {
		t.Fatalf("setup blocker file: %v", err)
	}

	err := InitMasterKey(dataDir)
	if err == nil {
		t.Fatalf("expected InitMasterKey to fail when the key cannot be persisted")
	}
	if !strings.Contains(err.Error(), "keys") {
		t.Fatalf("expected the error to name the key location, got: %v", err)
	}
}

// TestInitMasterKeyRejectsCorruptKeyFile: an existing but short key file must
// abort startup rather than being silently replaced (which would strand every
// secret encrypted with the previous key).
func TestInitMasterKeyRejectsCorruptKeyFile(t *testing.T) {
	dataDir := t.TempDir()
	keyDir := filepath.Join(dataDir, "keys")
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		t.Fatalf("setup key dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(keyDir, "master.key"), []byte("short"), 0600); err != nil {
		t.Fatalf("setup key file: %v", err)
	}

	if err := InitMasterKey(dataDir); err == nil {
		t.Fatalf("expected InitMasterKey to reject a corrupt key file")
	}
}

// TestInitMasterKeyReloadsExistingKey verifies the happy path: the same key is
// read back on the next init.
func TestInitMasterKeyReloadsExistingKey(t *testing.T) {
	dataDir := t.TempDir()
	if err := InitMasterKey(dataDir); err != nil {
		t.Fatalf("first InitMasterKey: %v", err)
	}
	first, err := getKey()
	if err != nil {
		t.Fatalf("getKey after first init: %v", err)
	}
	copied := append([]byte(nil), first...)

	if err := InitMasterKey(dataDir); err != nil {
		t.Fatalf("second InitMasterKey: %v", err)
	}
	second, err := getKey()
	if err != nil {
		t.Fatalf("getKey after second init: %v", err)
	}
	if string(copied) != string(second) {
		t.Fatalf("key changed across restarts")
	}
}

func TestUninitializedMasterKeyReturnsError(t *testing.T) {
	keyMu.Lock()
	saved := masterKey
	masterKey = nil
	keyMu.Unlock()
	defer func() {
		keyMu.Lock()
		masterKey = saved
		keyMu.Unlock()
	}()

	_, err := Encrypt("test")
	if err == nil {
		t.Fatalf("Expected error when encrypting with uninitialized master key, got nil")
	}
}
