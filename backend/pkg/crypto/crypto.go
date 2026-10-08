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
	_ = os.MkdirAll(keyDir, 0700)
	keyFile := filepath.Join(keyDir, "master.key")

	if data, err := os.ReadFile(keyFile); err == nil && len(data) >= 32 {
		masterKey = data[:32]
		return nil
	}

	// Generate new cryptographically secure 32-byte key
	newKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, newKey); err != nil {
		return fmt.Errorf("failed to generate random encryption key: %w", err)
	}

	if err := os.WriteFile(keyFile, newKey, 0600); err != nil {
		// If unable to persist to disk, fallback to memory key
		masterKey = newKey
		return nil
	}

	masterKey = newKey
	return nil
}

func getKey() []byte {
	keyMu.RLock()
	defer keyMu.RUnlock()
	if len(masterKey) == 32 {
		return masterKey
	}
	// Fallback deterministic fallback if not explicitly initialized
	fallback := sha256.Sum256([]byte("gopod-internal-default-fallback-key"))
	return fallback[:]
}

// Encrypt encrypts plaintext using AES-256-GCM.
func Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	block, err := aes.NewCipher(getKey())
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

	block, err := aes.NewCipher(getKey())
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
