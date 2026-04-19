// Package logic implements the repository service business logic.
package logic

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
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
}

// NewRepositoryLogic creates a new RepositoryLogic instance.
func NewRepositoryLogic(cfg config.Config, repo repo.Repository, encryption *security.AESGCMEncryption, logger *zap.Logger) *RepositoryLogic {
	return &RepositoryLogic{
		cfg:        cfg,
		repo:       repo,
		encryption: encryption,
		logger:     logger,
		tempDir:    cfg.TempDir,
	}
}

// Create creates a new repository connection with encrypted credentials.
func (l *RepositoryLogic) Create(ctx context.Context, tenantID uuid.UUID, req *types.CreateRepositoryRequest) (*model.SafeRepository, error) {
	repoID := uuid.New()
	now := time.Now()

	repository := &model.Repository{
		ID:            repoID,
		TenantID:      tenantID,
		Name:          req.Name,
		URL:           req.URL,
		Provider:      model.Provider(req.Provider),
		DefaultBranch: req.DefaultBranch,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	// Encrypt credentials if provided
	if req.Username != nil && req.Token != nil {
		// Format credentials as username:token
		credBytes := []byte(*req.Username + ":" + *req.Token)
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

	// Encrypt webhook secret if provided
	if req.WebhookSecret != nil {
		secretBytes := []byte(*req.WebhookSecret)
		encrypted, err := l.encryption.Encrypt(secretBytes)
		if err != nil {
			return nil, fmt.Errorf("encrypt webhook secret: %w", err)
		}
		repository.WebhookSecret = encrypted

		// Clear from memory
		for i := range secretBytes {
			secretBytes[i] = 0
		}
	}

	if err := l.repo.Create(ctx, repository); err != nil {
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
	var auth *http.BasicAuth
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
			auth = &http.BasicAuth{
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

	// Create filesystem
	fs := osfs.New(testDir)

	// Clone - we don't need context since git.Clone doesn't accept it
	// The clone will complete quickly with depth 1
	_, err = git.Clone(nil, fs, cloneOptions)
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
	case model.ProviderGitHub, model.ProviderGitLab:
		// GitHub: sha256=...
		// GitLab: same format
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
	// Zero the buffer immediately after splitting
	for i := range credBytes {
		credBytes[i] = 0
	}

	if len(parts) != 2 {
		// If it's just a token (no username), username can be "token"
		return "token", string(credBytes), nil
	}

	return parts[0], parts[1], nil
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
	var auth *http.BasicAuth
	if len(repository.EncryptedCredentials) > 0 {
		username, token, err := l.DecryptCredentials(repository.EncryptedCredentials)
		if err != nil {
			return "", fmt.Errorf("decrypt credentials: %w", err)
		}
		auth = &http.BasicAuth{
			Username: username,
			Password: token,
		}
	}

	// Create unique temp directory for this clone
	cloneDir := filepath.Join(l.tempDir, fmt.Sprintf("scan-%s", uuid.New().String()))
	if err := os.MkdirAll(cloneDir, 0700); err != nil {
		return "", fmt.Errorf("create clone directory: %w", err)
	}

	// Check if this repository already exists - if so, do incremental pull
	fs := osfs.New(cloneDir)

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

	// Do the clone
	_, err := git.Clone(nil, fs, cloneOptions)
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
