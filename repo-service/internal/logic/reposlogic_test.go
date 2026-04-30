package logic

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"sovereign-ai-compliance/repo-service/internal/config"
	"sovereign-ai-compliance/repo-service/internal/types"
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

func TestVerifyWebhookSignatureGitLabToken(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	enc, err := security.NewAESGCMEncryption(key)
	if err != nil {
		t.Fatalf("NewAESGCMEncryption failed: %v", err)
	}

	logic := NewRepositoryLogic(config.Config{TempDir: "/tmp/test"}, nil, enc, zap.NewNop())
	secret := []byte("gitlab-secret-token")
	encryptedSecret, err := enc.Encrypt(secret)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	repository := &model.Repository{
		ID:            uuid.New(),
		TenantID:      uuid.New(),
		WebhookSecret: encryptedSecret,
		Provider:      model.ProviderGitLab,
	}

	valid, err := logic.VerifyWebhookSignature(context.Background(), repository, []byte(`{}`), "gitlab-secret-token", model.ProviderGitLab)
	if err != nil {
		t.Fatalf("VerifyWebhookSignature failed: %v", err)
	}
	if !valid {
		t.Fatal("expected valid gitlab token signature")
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

func TestCreateInfersProviderDefaultsBranchAndResolvesUsername(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	enc, err := security.NewAESGCMEncryption(key)
	if err != nil {
		t.Fatalf("NewAESGCMEncryption failed: %v", err)
	}

	logger := zap.NewNop()
	cfg := config.Config{TempDir: "/tmp/test", WebhookBaseURL: "https://api.example.com"}
	repo := &stubRepository{}
	logic := NewRepositoryLogic(cfg, repo, enc, logger)
	logic.providers = &stubProviderClient{username: "octocat", webhookID: "12345"}

	token := "secret-token"
	req := &types.CreateRepositoryRequest{
		Name:  "repo",
		URL:   "https://github.com/acme/repo",
		Token: &token,
	}

	safe, err := logic.Create(context.Background(), uuid.New(), req)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if safe.Provider != model.ProviderGitHub {
		t.Fatalf("expected provider github, got %q", safe.Provider)
	}
	if safe.DefaultBranch != "main" {
		t.Fatalf("expected default branch main, got %q", safe.DefaultBranch)
	}
	if repo.created == nil {
		t.Fatal("expected repository to be created")
	}
	if !repo.created.WebhookID.Valid || repo.created.WebhookID.String != "12345" {
		t.Fatalf("expected webhook id to be persisted, got %+v", repo.created.WebhookID)
	}
	if len(repo.created.WebhookSecret) == 0 {
		t.Fatal("expected encrypted webhook secret to be persisted")
	}

	username, decryptedToken, err := logic.DecryptCredentials(repo.created.EncryptedCredentials)
	if err != nil {
		t.Fatalf("DecryptCredentials failed: %v", err)
	}
	if username != "octocat" {
		t.Fatalf("expected username octocat, got %q", username)
	}
	if decryptedToken != token {
		t.Fatalf("expected token %q, got %q", token, decryptedToken)
	}
}

func TestCreateFailsWhenProviderTokenCannotBeResolved(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	enc, err := security.NewAESGCMEncryption(key)
	if err != nil {
		t.Fatalf("NewAESGCMEncryption failed: %v", err)
	}

	logger := zap.NewNop()
	cfg := config.Config{TempDir: "/tmp/test", WebhookBaseURL: "https://api.example.com"}
	repo := &stubRepository{}
	logic := NewRepositoryLogic(cfg, repo, enc, logger)
	logic.providers = &stubProviderClient{err: errors.New("bad token")}

	token := "secret-token"
	_, err = logic.Create(context.Background(), uuid.New(), &types.CreateRepositoryRequest{
		Name:  "repo",
		URL:   "https://gitlab.com/acme/repo",
		Token: &token,
	})
	if err == nil {
		t.Fatal("expected Create to fail")
	}
	if repo.created != nil {
		t.Fatal("repository should not be persisted when token verification fails")
	}
}

type stubRepository struct {
	created *model.Repository
	err     error
}

func (s *stubRepository) GetByID(context.Context, uuid.UUID, uuid.UUID) (*model.Repository, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRepository) GetByIDAnyTenant(context.Context, uuid.UUID) (*model.Repository, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRepository) GetByURL(context.Context, uuid.UUID, string) (*model.Repository, error) {
	return nil, errors.New("repository not found")
}

func (s *stubRepository) ListByTenant(context.Context, uuid.UUID) ([]model.Repository, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRepository) Create(_ context.Context, repo *model.Repository) error {
	if s.err != nil {
		return s.err
	}
	copy := *repo
	s.created = &copy
	return nil
}

func (s *stubRepository) Update(context.Context, *model.Repository) error {
	return errors.New("not implemented")
}

func (s *stubRepository) Delete(context.Context, uuid.UUID, uuid.UUID) error {
	return errors.New("not implemented")
}

func (s *stubRepository) GetScanResult(context.Context, uuid.UUID, uuid.UUID) (*model.ScanResult, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRepository) ListScanResults(context.Context, uuid.UUID) ([]model.ScanResult, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRepository) CreateScanResult(context.Context, *model.ScanResult) error {
	return errors.New("not implemented")
}

func (s *stubRepository) UpdateScanResult(context.Context, *model.ScanResult) error {
	return errors.New("not implemented")
}

type stubProviderClient struct {
	username         string
	webhookID        string
	err              error
	deletedWebhookID string
}

func (s *stubProviderClient) ResolveUsername(context.Context, model.Provider, string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.username, nil
}

func (s *stubProviderClient) CreateWebhook(context.Context, model.Provider, string, string, string, string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.webhookID, nil
}

func (s *stubProviderClient) DeleteWebhook(context.Context, model.Provider, string, string, string) error {
	s.deletedWebhookID = s.webhookID
	return nil
}

func TestInferProviderFromURL(t *testing.T) {
	tests := []struct {
		url      string
		expected model.Provider
	}{
		{url: "https://github.com/acme/repo", expected: model.ProviderGitHub},
		{url: "https://gitlab.com/acme/repo", expected: model.ProviderGitLab},
		{url: "https://example.com/acme/repo", expected: ""},
	}

	for _, tt := range tests {
		if got := inferProviderFromURL(tt.url); got != tt.expected {
			t.Fatalf("inferProviderFromURL(%q) = %q, want %q", tt.url, got, tt.expected)
		}
	}
}

func TestBuildGitAuth(t *testing.T) {
	auth := buildGitAuth(model.ProviderGitHub, "octocat", "token")
	if auth == nil || auth.Username != "octocat" || auth.Password != "token" {
		t.Fatal("expected explicit username and token to be preserved")
	}

	auth = buildGitAuth(model.ProviderGitHub, "", "token")
	if auth == nil || auth.Username != "x-access-token" {
		t.Fatalf("expected github fallback username, got %+v", auth)
	}

	auth = buildGitAuth(model.ProviderGitLab, "", "token")
	if auth == nil || auth.Username != "oauth2" {
		t.Fatalf("expected gitlab fallback username, got %+v", auth)
	}
}

func TestCreateCleansUpWebhookWhenRepositoryInsertFails(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	enc, err := security.NewAESGCMEncryption(key)
	if err != nil {
		t.Fatalf("NewAESGCMEncryption failed: %v", err)
	}

	providers := &stubProviderClient{username: "octocat", webhookID: "9876"}
	repo := &stubRepository{err: errors.New("insert failed")}
	logic := NewRepositoryLogic(config.Config{TempDir: "/tmp/test", WebhookBaseURL: "https://api.example.com"}, repo, enc, zap.NewNop())
	logic.providers = providers

	token := "secret-token"
	_, err = logic.Create(context.Background(), uuid.New(), &types.CreateRepositoryRequest{
		Name:  "repo",
		URL:   "https://github.com/acme/repo",
		Token: &token,
	})
	if err == nil {
		t.Fatal("expected Create to fail")
	}
	if providers.deletedWebhookID != "9876" {
		t.Fatalf("expected webhook cleanup to run, got %q", providers.deletedWebhookID)
	}
}
