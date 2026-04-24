package jwt

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestNewManager(t *testing.T) {
	keyPath := writeTestPrivateKey(t)
	m, err := NewManager(keyPath, "test-issuer", 15*time.Minute, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestGenerateTokenPair(t *testing.T) {
	keyPath := writeTestPrivateKey(t)
	m, _ := NewManager(keyPath, "test-issuer", 15*time.Minute, 7*24*time.Hour)

	pair, err := m.GenerateTokenPair("user-1", "tenant-1", "admin", "test@example.com")
	if err != nil {
		t.Fatalf("failed to generate tokens: %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("expected non-empty tokens")
	}
	if pair.AccessToken == pair.RefreshToken {
		t.Fatal("access_token and refresh_token must be different")
	}
	if pair.AccessExpiry.Before(time.Now()) || pair.RefreshExpiry.Before(time.Now()) {
		t.Fatal("expected future expiry times")
	}
}

func TestParse_ValidAccessToken(t *testing.T) {
	keyPath := writeTestPrivateKey(t)
	m, _ := NewManager(keyPath, "test-issuer", 15*time.Minute, 7*24*time.Hour)

	pair, _ := m.GenerateTokenPair("user-1", "tenant-1", "admin", "test@example.com")

	claims, err := m.Parse(pair.AccessToken)
	if err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}
	if claims.UserID != "user-1" {
		t.Errorf("expected user-1, got %s", claims.UserID)
	}
	if claims.TenantID != "tenant-1" {
		t.Errorf("expected tenant-1, got %s", claims.TenantID)
	}
	if claims.Role != "admin" {
		t.Errorf("expected admin, got %s", claims.Role)
	}
	if claims.TokenType != "access" {
		t.Errorf("expected access, got %s", claims.TokenType)
	}
}

func TestParse_ExpiredToken(t *testing.T) {
	keyPath := writeTestPrivateKey(t)
	m, _ := NewManager(keyPath, "test-issuer", -1*time.Second, 7*24*time.Hour)

	pair, _ := m.GenerateTokenPair("user-1", "tenant-1", "admin", "test@example.com")

	_, err := m.Parse(pair.AccessToken)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestParse_InvalidToken(t *testing.T) {
	keyPath := writeTestPrivateKey(t)
	m, _ := NewManager(keyPath, "test-issuer", 15*time.Minute, 7*24*time.Hour)

	_, err := m.Parse("invalid.token.here")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestGenerateAccessToken(t *testing.T) {
	keyPath := writeTestPrivateKey(t)
	m, _ := NewManager(keyPath, "test-issuer", 15*time.Minute, 7*24*time.Hour)

	token, expiry, err := m.GenerateAccessToken("user-1", "tenant-1", "admin", "test@example.com")
	if err != nil {
		t.Fatalf("failed to generate access token: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if expiry.Before(time.Now()) {
		t.Fatal("expected future expiry")
	}
}

func TestParse_RefreshTokenType(t *testing.T) {
	keyPath := writeTestPrivateKey(t)
	m, _ := NewManager(keyPath, "test-issuer", 15*time.Minute, 7*24*time.Hour)

	pair, _ := m.GenerateTokenPair("user-1", "tenant-1", "admin", "test@example.com")

	claims, err := m.Parse(pair.RefreshToken)
	if err != nil {
		t.Fatalf("failed to parse refresh token: %v", err)
	}
	if claims.TokenType != "refresh" {
		t.Errorf("expected refresh, got %s", claims.TokenType)
	}
}

func TestParse_WrongSigningMethod(t *testing.T) {
	keyPath := writeTestPrivateKey(t)
	m, _ := NewManager(keyPath, "test-issuer", 15*time.Minute, 7*24*time.Hour)

	// Create HMAC-signed token
	claims := jwt.MapClaims{
		"sub": "user-1",
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte("secret"))

	_, err := m.Parse(tokenString)
	if err == nil {
		t.Fatal("expected error for wrong signing method")
	}
}

func writeTestPrivateKey(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	privPath := filepath.Join(dir, "jwt-private.pem")
	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})
	if err := os.WriteFile(privPath, privPEM, 0600); err != nil {
		t.Fatalf("failed to write private key: %v", err)
	}

	return privPath
}
