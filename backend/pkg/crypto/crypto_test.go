package crypto

import (
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	_ = InitMasterKey("")

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
