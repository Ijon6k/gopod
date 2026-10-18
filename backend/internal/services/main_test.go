package services

import (
	"os"
	"testing"

	"gopod/pkg/crypto"
)

// TestMain initializes a throwaway AES master key once for the package. The
// repository refuses to persist env blobs when encryption is unavailable, so
// every test run needs a key.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "gopod-services-test-key-*")
	if err != nil {
		panic(err)
	}
	if err := crypto.InitMasterKey(dir); err != nil {
		panic(err)
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
