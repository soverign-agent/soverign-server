package security

import (
	"bytes"
	"testing"
)

func TestAESGCMEncryption(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	enc, err := NewAESGCMEncryption(key)
	if err != nil {
		t.Fatalf("failed to create encryption: %v", err)
	}

	plaintext := []byte("sensitive git credential data")
	ciphertext, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	if len(ciphertext) == 0 {
		t.Fatal("ciphertext should not be empty")
	}

	decrypted, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("decrypted text does not match original: got %s, want %s", decrypted, plaintext)
	}
}

func TestAESGCMEncryption_KeySize(t *testing.T) {
	_, err := NewAESGCMEncryption([]byte("short"))
	if err == nil {
		t.Error("expected error for invalid key size")
	}
}

func TestAESGCMEncryption_TamperedCiphertext(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	enc, _ := NewAESGCMEncryption(key)
	ciphertext, _ := enc.Encrypt([]byte("test"))
	ciphertext[len(ciphertext)-1] ^= 0xFF

	_, err := enc.Decrypt(ciphertext)
	if err == nil {
		t.Error("expected error for tampered ciphertext")
	}
}
