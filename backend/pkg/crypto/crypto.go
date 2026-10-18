package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const prefix = "enc:v1:"

var (
	masterKey []byte
	keyMu     sync.RWMutex
)

// InitMasterKey initializes or loads the 256-bit AES master encryption key.
// Initialization fails hard on an unusable key: a key that cannot be persisted
// (or a corrupt key file) would silently make every stored secret undecryptable
// after the next restart.
func InitMasterKey(dataDir string) error {
	keyMu.Lock()
	defer keyMu.Unlock()

	// 1. Check environment variable
	if envKey := os.Getenv("GOPOD_ENCRYPTION_KEY"); envKey != "" {
		hash := sha256.Sum256([]byte(envKey))
		masterKey = hash[:]
		return nil
	}

	// 2. Check / create key file on disk
	if dataDir == "" {
		dataDir = "./data"
	}
	keyDir := filepath.Join(dataDir, "keys")
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		return fmt.Errorf("create key directory %s: %w", keyDir, err)
	}
	keyFile := filepath.Join(keyDir, "master.key")

	if data, err := os.ReadFile(keyFile); err == nil {
		if len(data) < 32 {
			return fmt.Errorf("master key file %s is corrupt (%d bytes, want at least 32); refusing to start with a key that cannot decrypt existing secrets", keyFile, len(data))
		}
		masterKey = data[:32]
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read master key file %s: %w", keyFile, err)
	}

	// Generate new cryptographically secure 32-byte key
	newKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, newKey); err != nil {
		return fmt.Errorf("failed to generate random encryption key: %w", err)
	}

	if err := os.WriteFile(keyFile, newKey, 0600); err != nil {
		return fmt.Errorf("persist master key to %s: %w (secrets would be lost on restart, refusing to start)", keyFile, err)
	}

	masterKey = newKey
	return nil
}

func getKey() ([]byte, error) {
	keyMu.RLock()
	defer keyMu.RUnlock()
	if len(masterKey) == 32 {
		return masterKey, nil
	}
	return nil, fmt.Errorf("master encryption key not initialized")
}

// KeyFilePath returns the on-disk location of the master key for a data
// directory. Used by the startup doctor to report where secrets are anchored.
func KeyFilePath(dataDir string) string {
	if dataDir == "" {
		dataDir = "./data"
	}
	return filepath.Join(dataDir, "keys", "master.key")
}

// KeyFromEnv reports whether the master key is supplied via
// GOPOD_ENCRYPTION_KEY instead of the on-disk key file.
func KeyFromEnv() bool {
	return os.Getenv("GOPOD_ENCRYPTION_KEY") != ""
}

// Encrypt encrypts plaintext using AES-256-GCM.
func Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	key, err := getKey()
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	sealed := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	return fmt.Sprintf("%s%s:%s", prefix, hex.EncodeToString(nonce), hex.EncodeToString(sealed)), nil
}

// Decrypt decrypts AES-256-GCM encrypted string, or returns unchanged if plaintext.
func Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	if !strings.HasPrefix(ciphertext, prefix) {
		// Unencrypted backward compatibility
		return ciphertext, nil
	}

	raw := strings.TrimPrefix(ciphertext, prefix)
	parts := strings.Split(raw, ":")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid encrypted payload format")
	}

	nonce, err := hex.DecodeString(parts[0])
	if err != nil {
		return "", err
	}

	data, err := hex.DecodeString(parts[1])
	if err != nil {
		return "", err
	}

	key, err := getKey()
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	decrypted, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}

	return string(decrypted), nil
}
