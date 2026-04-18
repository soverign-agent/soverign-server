// Package password provides secure password hashing and verification.
package password

import (
	"crypto/subtle"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const minPasswordLength = 8
const bcryptCost = bcrypt.DefaultCost + 2 // 12 rounds

// Hash generates a bcrypt hash from a plaintext password.
func Hash(plaintext string) (string, error) {
	if len(plaintext) < minPasswordLength {
		return "", fmt.Errorf("password must be at least %d characters", minPasswordLength)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hash), nil
}

// Verify checks if a plaintext password matches a bcrypt hash.
// Returns nil on success, error on mismatch or validation failure.
func Verify(plaintext, hash string) error {
	// Always perform the comparison to avoid timing attacks leaking
	// whether the user exists or not.
	if len(hash) == 0 {
		// Perform a dummy comparison with a known-invalid hash
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$12$dummy.dummy.dummy.dummy.dummy.dummy.dummy."), []byte(plaintext))
		return fmt.Errorf("invalid credentials")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext)); err != nil {
		return fmt.Errorf("invalid credentials")
	}
	return nil
}

// CompareStrings performs a constant-time comparison of two strings.
func CompareStrings(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// MinLength returns the minimum required password length.
func MinLength() int {
	return minPasswordLength
}
