package middleware

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestJWTAuth_PublicEndpoint(t *testing.T) {
	keyPath := writeTestPublicKey(t)
	mw := JWTAuth(keyPath, []string{"/public"})

	called := false
	handler := mw(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/public", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if !called {
		t.Error("expected handler to be called for public endpoint")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestJWTAuth_MissingToken(t *testing.T) {
	keyPath := writeTestPublicKey(t)
	mw := JWTAuth(keyPath, []string{})

	handler := mw(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called without token")
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestJWTAuth_InvalidToken(t *testing.T) {
	keyPath := writeTestPublicKey(t)
	mw := JWTAuth(keyPath, []string{})

	handler := mw(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called with invalid token")
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestJWTAuth_ValidToken(t *testing.T) {
	keyPath, privateKey := writeTestKeyPair(t)
	mw := JWTAuth(keyPath, []string{})

	claims := jwt.MapClaims{
		"sub":       "user-123",
		"tenant_id": "tenant-abc",
		"exp":       time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	var ctxClaims jwt.MapClaims
	handler := mw(func(w http.ResponseWriter, r *http.Request) {
		ctxClaims = ClaimsFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if ctxClaims == nil {
		t.Fatal("expected claims in context")
	}
	if ctxClaims["tenant_id"] != "tenant-abc" {
		t.Errorf("expected tenant-abc, got %v", ctxClaims["tenant_id"])
	}
}

func TestJWTAuth_WrongSigningMethod(t *testing.T) {
	keyPath := writeTestPublicKey(t)
	mw := JWTAuth(keyPath, []string{})

	// Create token with HMAC instead of RSA
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "user-123",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	tokenString, _ := token.SignedString([]byte("secret"))

	handler := mw(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called with wrong signing method")
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func writeTestPublicKey(t *testing.T) string {
	t.Helper()
	path, _ := writeTestKeyPair(t)
	return path
}

func writeTestKeyPair(t *testing.T) (string, *rsa.PrivateKey) {
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

	pubPath := filepath.Join(dir, "jwt-public.pem")
	pubBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("failed to marshal public key: %v", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	})
	if err := os.WriteFile(pubPath, pubPEM, 0644); err != nil {
		t.Fatalf("failed to write public key: %v", err)
	}

	return pubPath, privateKey
}
