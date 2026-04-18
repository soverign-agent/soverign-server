package password

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashAndVerify(t *testing.T) {
	hash, err := Hash("securepassword123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}

	// Verify correct password
	if err := Verify("securepassword123", hash); err != nil {
		t.Fatalf("failed to verify correct password: %v", err)
	}

	// Verify wrong password
	if err := Verify("wrongpassword", hash); err == nil {
		t.Fatal("expected error for wrong password")
	}
}

func TestHash_MinLength(t *testing.T) {
	_, err := Hash("short")
	if err == nil {
		t.Fatal("expected error for short password")
	}
}

func TestVerify_EmptyHash(t *testing.T) {
	// Should not panic and should return error
	if err := Verify("somepassword", ""); err == nil {
		t.Fatal("expected error for empty hash")
	}
}

func TestHash_Unique(t *testing.T) {
	hash1, _ := Hash("securepassword123")
	hash2, _ := Hash("securepassword123")
	if hash1 == hash2 {
		t.Error("expected different hashes for same password (salted)")
	}
}

func TestVerify_BcryptFormat(t *testing.T) {
	// Ensure the hash is actually bcrypt
	hash, _ := Hash("securepassword123")
	if len(hash) < 60 {
		t.Errorf("expected bcrypt hash length >= 60, got %d", len(hash))
	}
	// bcrypt hashes start with $2a$ or $2b$
	if hash[0:4] != "$2a$" && hash[0:4] != "$2b$" {
		t.Errorf("expected bcrypt prefix, got %s", hash[0:4])
	}
}

func TestCompareStrings(t *testing.T) {
	if !CompareStrings("hello", "hello") {
		t.Error("expected equal strings to match")
	}
	if CompareStrings("hello", "world") {
		t.Error("expected different strings to not match")
	}
}

func TestMinLength(t *testing.T) {
	if MinLength() != 8 {
		t.Errorf("expected min length 8, got %d", MinLength())
	}
}

func TestBcryptCost(t *testing.T) {
	// Verify we're using a reasonable cost
	if bcryptCost < bcrypt.DefaultCost {
		t.Errorf("expected cost >= %d, got %d", bcrypt.DefaultCost, bcryptCost)
	}
}
