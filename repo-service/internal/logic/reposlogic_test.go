package logic

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"sovereign-ai-compliance/repo-service/internal/config"
	"sovereign-ai-compliance/repo-service/model"
	"sovereign-ai-compliance/shared/security"
)

func TestVerifyWebhookSignature(t *testing.T) {
	// Create a 32-byte test key
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	enc, err := security.NewAESGCMEncryption(key)
	if err != nil {
		t.Fatalf("NewAESGCMEncryption failed: %v", err)
	}

	logger, _ := zap.NewProduction()

	// Create logic instance
	cfg := config.Config{
		TempDir: "/tmp/test",
	}
	logic := NewRepositoryLogic(cfg, nil, enc, logger)

	// Create repository with encrypted webhook secret
	secret := []byte("my-webhook-secret")
	encryptedSecret, err := enc.Encrypt(secret)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	repo := &model.Repository{
		ID:            uuid.New(),
		TenantID:      uuid.New(),
		WebhookSecret: encryptedSecret,
		Provider:      model.ProviderGitHub,
	}

	// Test payload
	payload := []byte(`{"ref": "refs/heads/main"}`)

	valid, err := logic.VerifyWebhookSignature(context.Background(), repo, payload, "", model.ProviderGitHub)
	if err != nil {
		t.Errorf("VerifyWebhookSignature failed: %v", err)
	}
	if valid {
		t.Errorf("expected invalid signature when no header provided, got valid")
	}

	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	valid, err = logic.VerifyWebhookSignature(context.Background(), repo, payload, signature, model.ProviderGitHub)
	if err != nil {
		t.Errorf("VerifyWebhookSignature failed: %v", err)
	}
	if !valid {
		t.Errorf("expected valid signature, got invalid")
	}
}

func TestDecryptCredentials(t *testing.T) {
	// Create a 32-byte test key
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	enc, err := security.NewAESGCMEncryption(key)
	if err != nil {
		t.Fatalf("NewAESGCMEncryption failed: %v", err)
	}

	logger, _ := zap.NewProduction()
	cfg := config.Config{
		TempDir: "/tmp/test",
	}
	logic := NewRepositoryLogic(cfg, nil, enc, logger)

	// Test username:token format
	credBytes := []byte("testuser:testtoken")
	encrypted, err := enc.Encrypt(credBytes)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// The encrypted bytes get zeroed after decrypt in DecryptCredentials - check that
	username, token, err := logic.DecryptCredentials(encrypted)
	if err != nil {
		t.Fatalf("DecryptCredentials failed: %v", err)
	}

	if username != "testuser" {
		t.Errorf("expected username testuser, got %s", username)
	}
	if token != "testtoken" {
		t.Errorf("expected token testtoken, got %s", token)
	}
}
