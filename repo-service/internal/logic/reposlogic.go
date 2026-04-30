// Package logic implements the repository service business logic.
package logic

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"sovereign-ai-compliance/repo-service/internal/analysis"
	"sovereign-ai-compliance/repo-service/internal/config"
	"sovereign-ai-compliance/repo-service/internal/types"
	"sovereign-ai-compliance/repo-service/model"
	"sovereign-ai-compliance/repo-service/repo"
	"sovereign-ai-compliance/shared/security"
)

// RepositoryLogic handles repository connection management business logic.
type RepositoryLogic struct {
	cfg        config.Config
	repo       repo.Repository
	encryption *security.AESGCMEncryption
	tempDir    string
	logger     *zap.Logger
	providers  providerClient
}

type providerClient interface {
	ResolveUsername(ctx context.Context, provider model.Provider, token string) (string, error)
	CreateWebhook(ctx context.Context, provider model.Provider, repoURL, token, callbackURL, secret string) (string, error)
	DeleteWebhook(ctx context.Context, provider model.Provider, repoURL, token, webhookID string) error
}

type httpProviderClient struct {
	client *http.Client
}

// NewRepositoryLogic creates a new RepositoryLogic instance.
func NewRepositoryLogic(cfg config.Config, repo repo.Repository, encryption *security.AESGCMEncryption, logger *zap.Logger) *RepositoryLogic {
	return &RepositoryLogic{
		cfg:        cfg,
		repo:       repo,
		encryption: encryption,
		logger:     logger,
		tempDir:    cfg.TempDir,
		providers: &httpProviderClient{
			client: &http.Client{Timeout: 10 * time.Second},
		},
	}
}

// Create creates a new repository connection with encrypted credentials.
func (l *RepositoryLogic) Create(ctx context.Context, tenantID uuid.UUID, req *types.CreateRepositoryRequest) (*model.SafeRepository, error) {
	repoID := uuid.New()
	now := time.Now()

	// Prevent duplicate repositories for the same tenant + URL
	existing, err := l.repo.GetByURL(ctx, tenantID, req.URL)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("repository already exists: %s", req.URL)
	}

	provider := model.Provider(req.Provider)
	if provider == "" {
		provider = inferProviderFromURL(req.URL)
	}
	if provider == "" {
		return nil, fmt.Errorf("repository provider is required")
	}

	defaultBranch := strings.TrimSpace(req.DefaultBranch)
	if defaultBranch == "" {
		defaultBranch = "main"
	}

	repository := &model.Repository{
		ID:            repoID,
		TenantID:      tenantID,
		Name:          req.Name,
		URL:           req.URL,
		Provider:      provider,
		DefaultBranch: defaultBranch,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	// Encrypt credentials if provided
	token := strings.TrimSpace(derefString(req.Token))
	if token != "" {
		username := strings.TrimSpace(derefString(req.Username))
		if username == "" {
			resolvedUsername, err := l.providers.ResolveUsername(ctx, provider, token)
			if err != nil {
				return nil, fmt.Errorf("resolve username from provider token: %w", err)
			}
			username = resolvedUsername
		}

		// Format credentials as username:token
		credBytes := []byte(username + ":" + token)
		encrypted, err := l.encryption.Encrypt(credBytes)
		if err != nil {
			return nil, fmt.Errorf("encrypt credentials: %w", err)
		}
		repository.EncryptedCredentials = encrypted

		// Clear from memory immediately after encryption
		// Overwrite the buffer - note: this doesn't guarantee clearing all copies in memory
		for i := range credBytes {
			credBytes[i] = 0
		}
	}

	if shouldManageWebhook(provider) {
		if token == "" {
			return nil, fmt.Errorf("provider token is required to configure repository webhook")
		}
		if strings.TrimSpace(l.cfg.WebhookBaseURL) == "" {
			return nil, fmt.Errorf("webhook_base_url must be configured")
		}

		webhookSecret, err := generateWebhookSecret()
		if err != nil {
			return nil, fmt.Errorf("generate webhook secret: %w", err)
		}
		secretBytes := []byte(webhookSecret)
		encrypted, err := l.encryption.Encrypt(secretBytes)
		if err != nil {
			return nil, fmt.Errorf("encrypt webhook secret: %w", err)
		}
		repository.WebhookSecret = encrypted

		callbackURL := buildWebhookCallbackURL(l.cfg.WebhookBaseURL, repoID)
		webhookID, err := l.providers.CreateWebhook(ctx, provider, req.URL, token, callbackURL, webhookSecret)
		for i := range secretBytes {
			secretBytes[i] = 0
		}
		if err != nil {
			return nil, fmt.Errorf("create provider webhook: %w", err)
		}
		repository.WebhookID = sqlNullString(webhookID)
	}

	if err := l.repo.Create(ctx, repository); err != nil {
		if repository.WebhookID.Valid {
			if cleanupErr := l.providers.DeleteWebhook(ctx, provider, req.URL, token, repository.WebhookID.String); cleanupErr != nil {
				l.logger.Warn("failed to cleanup provider webhook after repository create failure", zap.Error(cleanupErr), zap.String("repository_url", req.URL), zap.String("webhook_id", repository.WebhookID.String))
			}
		}
		return nil, fmt.Errorf("create repository in database: %w", err)
	}

	safe := repository.ToSafe()
	return &safe, nil
}

// List lists all repositories for a tenant.
func (l *RepositoryLogic) List(ctx context.Context, tenantID uuid.UUID) ([]model.SafeRepository, error) {
	repos, err := l.repo.ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list repositories: %w", err)
	}

	safeRepos := make([]model.SafeRepository, len(repos))
	for i, r := range repos {
		safeRepos[i] = r.ToSafe()
	}

	return safeRepos, nil
}

// Delete deletes a repository connection.
func (l *RepositoryLogic) Delete(ctx context.Context, tenantID, repoID uuid.UUID) (bool, error) {
	err := l.repo.Delete(ctx, tenantID, repoID)
	if err != nil {
		return false, fmt.Errorf("delete repository: %w", err)
	}
	return true, nil
}

// TestConnection tests if the repository connection can be cloned/fetched successfully.
func (l *RepositoryLogic) TestConnection(ctx context.Context, tenantID, repoID uuid.UUID) (bool, string, error) {
	repository, err := l.repo.GetByID(ctx, tenantID, repoID)
	if err != nil {
		return false, "", fmt.Errorf("get repository: %w", err)
	}

	// Decrypt credentials
	var auth *githttp.BasicAuth
	if len(repository.EncryptedCredentials) > 0 {
		credBytes, err := l.encryption.Decrypt(repository.EncryptedCredentials)
		if err != nil {
			return false, "", fmt.Errorf("decrypt credentials: %w", err)
		}

		parts := strings.SplitN(string(credBytes), ":", 2)
		// Clear after use
		for i := range credBytes {
			credBytes[i] = 0
		}

		if len(parts) == 2 {
			auth = &githttp.BasicAuth{
				Username: parts[0],
				Password: parts[1],
			}
		}
	}

	// Create a temporary directory for the test clone
	testDir := filepath.Join(l.tempDir, fmt.Sprintf("test-%s", uuid.New().String()))
	if err := os.MkdirAll(l.tempDir, 0700); err != nil {
		return false, "", fmt.Errorf("create temp directory: %w", err)
	}

	// Cleanup after test
	defer func() {
		if err := os.RemoveAll(testDir); err != nil {
			l.logger.Warn("failed to cleanup test directory", zap.String("path", testDir), zap.Error(err))
		}
	}()

	// Try to clone with a depth of 1 to be quick
	cloneOptions := &git.CloneOptions{
		URL:        repository.URL,
		Depth:      1,
		NoCheckout: true,
	}
	if auth != nil {
		cloneOptions.Auth = auth
	}

	// PlainClone handles directory creation safely; avoid git.Clone with nil storer
	_, err = git.PlainClone(testDir, false, cloneOptions)
	if err != nil {
		return false, fmt.Sprintf("clone failed: %v", err), nil
	}

	return true, "Connection successful", nil
}

// VerifyWebhookSignature verifies that the webhook signature is valid.
// This supports GitHub and GitLab signature formats.
func (l *RepositoryLogic) VerifyWebhookSignature(ctx context.Context, repository *model.Repository, payload []byte, signatureHeader string, provider model.Provider) (bool, error) {
	if len(repository.WebhookSecret) == 0 {
		// No secret configured - skip verification (not recommended but allowed)
		return true, nil
	}

	// Decrypt the webhook secret
	secretBytes, err := l.encryption.Decrypt(repository.WebhookSecret)
	if err != nil {
		return false, fmt.Errorf("decrypt webhook secret: %w", err)
	}
	defer func() {
		// Clear from memory after use
		for i := range secretBytes {
			secretBytes[i] = 0
		}
	}()

	var expectedSignature string

	switch provider {
	case model.ProviderGitHub:
		// GitHub: sha256=...
		if strings.HasPrefix(signatureHeader, "sha256=") {
			expectedSignature = strings.TrimPrefix(signatureHeader, "sha256=")
		} else {
			expectedSignature = signatureHeader
		}

		mac := hmac.New(sha256.New, secretBytes)
		mac.Write(payload)
		calculatedMAC := mac.Sum(nil)
		calculatedStr := hex.EncodeToString(calculatedMAC)

		// Use constant time comparison to prevent timing attacks
		return hmac.Equal([]byte(calculatedStr), []byte(expectedSignature)), nil
	case model.ProviderGitLab:
		// GitLab sends the raw webhook token back in X-Gitlab-Token.
		return hmac.Equal(secretBytes, []byte(signatureHeader)), nil

	default:
		// For self-hosted, we still do the same verification
		mac := hmac.New(sha256.New, secretBytes)
		mac.Write(payload)
		calculatedMAC := mac.Sum(nil)
		calculatedStr := hex.EncodeToString(calculatedMAC)

		return hmac.Equal([]byte(calculatedStr), []byte(expectedSignature)), nil
	}
}

// DecryptCredentials decrypts the repository credentials and returns username and token.
// The caller is responsible for zeroing the credentials after use.
func (l *RepositoryLogic) DecryptCredentials(encrypted []byte) (string, string, error) {
	if len(encrypted) == 0 {
		return "", "", nil
	}

	credBytes, err := l.encryption.Decrypt(encrypted)
	if err != nil {
		return "", "", fmt.Errorf("decrypt: %w", err)
	}

	parts := strings.SplitN(string(credBytes), ":", 2)
	token := ""
	if len(parts) == 2 {
		token = parts[1]
	} else {
		token = string(credBytes)
	}
	// Zero the buffer immediately after splitting
	for i := range credBytes {
		credBytes[i] = 0
	}

	if len(parts) != 2 {
		// If it's just a token (no username), username can be "token"
		return "token", token, nil
	}

	return parts[0], token, nil
}

func inferProviderFromURL(rawURL string) model.Provider {
	lower := strings.ToLower(rawURL)
	switch {
	case strings.Contains(lower, "github.com"):
		return model.ProviderGitHub
	case strings.Contains(lower, "gitlab.com"):
		return model.ProviderGitLab
	default:
		return ""
	}
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func (v *httpProviderClient) ResolveUsername(ctx context.Context, provider model.Provider, token string) (string, error) {
	switch provider {
	case model.ProviderGitHub:
		return v.resolveGitHubUsername(ctx, token)
	case model.ProviderGitLab:
		return v.resolveGitLabUsername(ctx, token)
	default:
		return "", fmt.Errorf("token verification is not supported for provider %q", provider)
	}
}

func (v *httpProviderClient) resolveGitHubUsername(ctx context.Context, token string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "sovereign-repo-service")

	var payload struct {
		Login string `json:"login"`
	}
	if err := v.doJSON(req, &payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.Login) == "" {
		return "", fmt.Errorf("github response did not include login")
	}
	return payload.Login, nil
}

func (v *httpProviderClient) resolveGitLabUsername(ctx context.Context, token string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://gitlab.com/api/v4/user", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "sovereign-repo-service")

	var payload struct {
		Username string `json:"username"`
	}
	if err := v.doJSON(req, &payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.Username) == "" {
		return "", fmt.Errorf("gitlab response did not include username")
	}
	return payload.Username, nil
}

func (v *httpProviderClient) CreateWebhook(ctx context.Context, provider model.Provider, repoURL, token, callbackURL, secret string) (string, error) {
	switch provider {
	case model.ProviderGitHub:
		return v.createGitHubWebhook(ctx, repoURL, token, callbackURL, secret)
	case model.ProviderGitLab:
		return v.createGitLabWebhook(ctx, repoURL, token, callbackURL, secret)
	default:
		return "", fmt.Errorf("webhook creation is not supported for provider %q", provider)
	}
}

func (v *httpProviderClient) DeleteWebhook(ctx context.Context, provider model.Provider, repoURL, token, webhookID string) error {
	switch provider {
	case model.ProviderGitHub:
		return v.deleteGitHubWebhook(ctx, repoURL, token, webhookID)
	case model.ProviderGitLab:
		return v.deleteGitLabWebhook(ctx, repoURL, token, webhookID)
	default:
		return fmt.Errorf("webhook deletion is not supported for provider %q", provider)
	}
}

func (v *httpProviderClient) createGitHubWebhook(ctx context.Context, repoURL, token, callbackURL, secret string) (string, error) {
	owner, repoName, err := parseRepoURL(repoURL)
	if err != nil {
		return "", err
	}
	body := map[string]any{
		"name":   "web",
		"active": true,
		"events": []string{"push"},
		"config": map[string]string{
			"url":          callbackURL,
			"content_type": "json",
			"secret":       secret,
		},
	}
	endpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/hooks", owner, repoName)
	req, err := v.newJSONRequest(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "sovereign-repo-service")

	var payload struct {
		ID int64 `json:"id"`
	}
	if err := v.doJSON(req, &payload); err != nil {
		return "", err
	}
	if payload.ID == 0 {
		return "", fmt.Errorf("github webhook response did not include id")
	}
	return fmt.Sprintf("%d", payload.ID), nil
}

func (v *httpProviderClient) createGitLabWebhook(ctx context.Context, repoURL, token, callbackURL, secret string) (string, error) {
	owner, repoName, err := parseRepoURL(repoURL)
	if err != nil {
		return "", err
	}
	projectPath := url.PathEscape(owner + "/" + repoName)
	body := map[string]any{
		"url":                     callbackURL,
		"push_events":             true,
		"token":                   secret,
		"enable_ssl_verification": false,
	}
	endpoint := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s/hooks", projectPath)
	req, err := v.newJSONRequest(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "sovereign-repo-service")

	var payload struct {
		ID int64 `json:"id"`
	}
	if err := v.doJSON(req, &payload); err != nil {
		return "", err
	}
	if payload.ID == 0 {
		return "", fmt.Errorf("gitlab webhook response did not include id")
	}
	return fmt.Sprintf("%d", payload.ID), nil
}

func (v *httpProviderClient) deleteGitHubWebhook(ctx context.Context, repoURL, token, webhookID string) error {
	owner, repoName, err := parseRepoURL(repoURL)
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/hooks/%s", owner, repoName, webhookID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "sovereign-repo-service")
	return v.doNoContent(req)
}

func (v *httpProviderClient) deleteGitLabWebhook(ctx context.Context, repoURL, token, webhookID string) error {
	owner, repoName, err := parseRepoURL(repoURL)
	if err != nil {
		return err
	}
	projectPath := url.PathEscape(owner + "/" + repoName)
	endpoint := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s/hooks/%s", projectPath, webhookID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "sovereign-repo-service")
	return v.doNoContent(req)
}

func (v *httpProviderClient) doJSON(req *http.Request, dest any) error {
	resp, err := v.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("provider API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return fmt.Errorf("decode provider response: %w", err)
	}
	return nil
}

func (v *httpProviderClient) doNoContent(req *http.Request) error {
	resp, err := v.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("provider API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (v *httpProviderClient) newJSONRequest(ctx context.Context, method, endpoint string, body any) (*http.Request, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal provider request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, strings.NewReader(string(raw)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

// GetTempDir returns the base temporary directory for scanning.
func (l *RepositoryLogic) GetTempDir() string {
	return l.tempDir
}

// CreateScanRecord creates a new scan record in the database.
func (l *RepositoryLogic) CreateScanRecord(ctx context.Context, tenantID, repoID uuid.UUID, branch, commitHash string) (*model.ScanResult, error) {
	scanID := uuid.New()
	now := time.Now()

	scan := &model.ScanResult{
		ID:            scanID,
		RepositoryID:  repoID,
		TenantID:      tenantID,
		Branch:        branch,
		CommitHash:    commitHash,
		AIUses:        []byte("[]"),
		DataFlows:     []byte("[]"),
		SensitiveData: []byte("[]"),
		TotalFiles:    0,
		ScannedFiles:  0,
		StartedAt:     now,
		CreatedAt:     now,
	}

	if err := l.repo.CreateScanResult(ctx, scan); err != nil {
		return nil, fmt.Errorf("create scan record: %w", err)
	}

	return scan, nil
}

// UpdateScanRecord updates an existing scan record with results.
func (l *RepositoryLogic) UpdateScanRecord(ctx context.Context, result *model.ScanResult) error {
	result.CompletedAt.Time = time.Now()
	result.CompletedAt.Valid = true
	if err := l.repo.UpdateScanResult(ctx, result); err != nil {
		return fmt.Errorf("update scan record: %w", err)
	}
	return nil
}

// GetScanResult retrieves a scan result by ID.
func (l *RepositoryLogic) GetScanResult(ctx context.Context, tenantID, scanID uuid.UUID) (*model.ScanResult, error) {
	return l.repo.GetScanResult(ctx, tenantID, scanID)
}

// GetByID retrieves a repository by ID.
func (l *RepositoryLogic) GetByID(ctx context.Context, tenantID, repoID uuid.UUID) (*model.Repository, error) {
	return l.repo.GetByID(ctx, tenantID, repoID)
}

// GetByIDAnyTenant retrieves a repository by ID for webhook handling.
func (l *RepositoryLogic) GetByIDAnyTenant(ctx context.Context, repoID uuid.UUID) (*model.Repository, error) {
	return l.repo.GetByIDAnyTenant(ctx, repoID)
}

// ListScanResults lists all scan results for a repository.
func (l *RepositoryLogic) ListScanResults(ctx context.Context, repoID uuid.UUID) ([]model.ScanResult, error) {
	return l.repo.ListScanResults(ctx, repoID)
}

// PullCode clones or pulls the latest code from the repository to a temporary directory.
// Returns the path to the cloned repository. Caller is responsible for cleanup.
func (l *RepositoryLogic) PullCode(ctx context.Context, repository *model.Repository, branch string) (string, error) {
	// Decrypt credentials
	var auth *githttp.BasicAuth
	if len(repository.EncryptedCredentials) > 0 {
		username, token, err := l.DecryptCredentials(repository.EncryptedCredentials)
		if err != nil {
			return "", fmt.Errorf("decrypt credentials: %w", err)
		}
		auth = buildGitAuth(repository.Provider, username, token)
	}

	// Create unique temp directory for this clone
	cloneDir := filepath.Join(l.tempDir, fmt.Sprintf("scan-%s", uuid.New().String()))
	if err := os.MkdirAll(l.tempDir, 0700); err != nil {
		return "", fmt.Errorf("create temp directory: %w", err)
	}

	var cloneOptions *git.CloneOptions
	if branch != "" {
		cloneOptions = &git.CloneOptions{
			URL:           repository.URL,
			Depth:         1,
			ReferenceName: plumbing.ReferenceName("refs/heads/" + branch),
			SingleBranch:  true,
		}
	} else {
		cloneOptions = &git.CloneOptions{
			URL:          repository.URL,
			Depth:        1,
			SingleBranch: true,
		}
		if repository.DefaultBranch != "" {
			cloneOptions.ReferenceName = plumbing.ReferenceName("refs/heads/" + repository.DefaultBranch)
		}
	}

	if auth != nil {
		cloneOptions.Auth = auth
	}

	// PlainClone handles directory creation safely; avoid git.Clone with nil storer
	_, err := git.PlainClone(cloneDir, false, cloneOptions)
	if err != nil {
		// Cleanup on failure
		_ = os.RemoveAll(cloneDir)
		return "", fmt.Errorf("clone repository: %w", err)
	}

	return cloneDir, nil
}

// CleanupCode removes the cloned repository from temporary storage.
func (l *RepositoryLogic) CleanupCode(cloneDir string) error {
	err := os.RemoveAll(cloneDir)
	if err != nil {
		l.logger.Warn("failed to cleanup clone directory", zap.String("path", cloneDir), zap.Error(err))
		return err
	}
	return nil
}

// FullScan performs a full scan: pull code, run static analysis, save results, cleanup.
func (l *RepositoryLogic) FullScan(ctx context.Context, tenantID, repoID uuid.UUID, branch string) (*model.ScanResult, error) {
	// Get repository
	repository, err := l.repo.GetByID(ctx, tenantID, repoID)
	if err != nil {
		return nil, fmt.Errorf("get repository: %w", err)
	}

	// If no branch specified, use default
	if branch == "" {
		branch = repository.DefaultBranch
	}

	// Pull code to temp dir
	cloneDir, err := l.PullCode(ctx, repository, branch)
	if err != nil {
		return nil, fmt.Errorf("pull code: %w", err)
	}
	defer l.CleanupCode(cloneDir)

	// Get commit hash from cloned repo
	clonedRepo, err := git.PlainOpen(cloneDir)
	if err != nil {
		return nil, fmt.Errorf("open cloned repository: %w", err)
	}
	head, err := clonedRepo.Head()
	if err != nil {
		return nil, fmt.Errorf("get HEAD: %w", err)
	}
	commitHash := head.Hash().String()

	// Create scan record
	scan, err := l.CreateScanRecord(ctx, tenantID, repoID, branch, commitHash)
	if err != nil {
		return nil, fmt.Errorf("create scan record: %w", err)
	}

	// Run static analysis
	analysisResult, err := analysis.AnalyzeDirectory(cloneDir)
	if err != nil {
		l.logger.Error("static analysis failed", zap.Error(err))
		// Still save what we have
		scan.TotalFiles = 0
		scan.ScannedFiles = 0
		l.UpdateScanRecord(ctx, scan)
		return scan, fmt.Errorf("static analysis: %w", err)
	}

	// Convert analysis to JSON for storage
	aiJSON, _ := json.Marshal(analysisResult.AIDetections)
	dfJSON, _ := json.Marshal(analysisResult.DataFlows)
	sdJSON, _ := json.Marshal(analysisResult.SensitiveDetections)

	scan.AIUses = aiJSON
	scan.DataFlows = dfJSON
	scan.SensitiveData = sdJSON
	scan.TotalFiles = analysisResult.TotalFiles
	scan.ScannedFiles = analysisResult.ScannedFiles

	// Update scan record with results
	if err := l.UpdateScanRecord(ctx, scan); err != nil {
		return scan, fmt.Errorf("update scan record: %w", err)
	}

	return scan, nil
}

func buildGitAuth(provider model.Provider, username, token string) *githttp.BasicAuth {
	if token == "" {
		return nil
	}

	user := username
	if provider == model.ProviderGitHub && user == "" {
		user = "x-access-token"
	}
	if provider == model.ProviderGitLab && user == "" {
		user = "oauth2"
	}
	if user == "" {
		user = tokenAsBasicUsername(token)
	}

	return &githttp.BasicAuth{
		Username: user,
		Password: token,
	}
}

func tokenAsBasicUsername(token string) string {
	encoded := base64.RawURLEncoding.EncodeToString([]byte(token))
	if encoded == "" {
		return "token"
	}
	return encoded
}

func shouldManageWebhook(provider model.Provider) bool {
	return provider == model.ProviderGitHub || provider == model.ProviderGitLab
}

func generateWebhookSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func buildWebhookCallbackURL(base string, repoID uuid.UUID) string {
	return strings.TrimRight(base, "/") + "/api/v1/repository-webhooks/" + repoID.String()
}

func parseRepoURL(raw string) (string, string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", "", fmt.Errorf("parse repository url: %w", err)
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("repository url must include owner and repository name")
	}
	repoName := strings.TrimSuffix(parts[1], ".git")
	if repoName == "" || parts[0] == "" {
		return "", "", fmt.Errorf("repository url must include owner and repository name")
	}
	return parts[0], repoName, nil
}

func sqlNullString(value string) sql.NullString {
	if strings.TrimSpace(value) == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}
