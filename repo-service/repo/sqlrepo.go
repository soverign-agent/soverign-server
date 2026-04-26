// Package repo provides a concrete PostgreSQL implementation of Repository.
package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"sovereign-ai-compliance/repo-service/model"
)

// sqlRepository implements Repository using database/sql.
type sqlRepository struct {
	db *sql.DB
}

// NewSQLRepository creates a new PostgreSQL-backed repository.
func NewSQLRepository(db *sql.DB) Repository {
	return &sqlRepository{db: db}
}

// GetByURL retrieves a repository by URL for the given tenant.
func (r *sqlRepository) GetByURL(ctx context.Context, tenantID uuid.UUID, url string) (*model.Repository, error) {
	var repo model.Repository
	var webhookID sql.NullString
	var lastScannedAt sql.NullTime

	err := r.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, name, url, provider, encrypted_credentials, webhook_secret, webhook_id, default_branch, last_scanned_at, created_at, updated_at
		 FROM repositories WHERE tenant_id = $1 AND url = $2`,
		tenantID, url,
	).Scan(
		&repo.ID,
		&repo.TenantID,
		&repo.Name,
		&repo.URL,
		&repo.Provider,
		&repo.EncryptedCredentials,
		&repo.WebhookSecret,
		&webhookID,
		&repo.DefaultBranch,
		&lastScannedAt,
		&repo.CreatedAt,
		&repo.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("repository not found")
		}
		return nil, fmt.Errorf("query repository by url: %w", err)
	}

	repo.WebhookID = webhookID
	repo.LastScannedAt = lastScannedAt
	return &repo, nil
}

// GetByID retrieves a repository by ID for the given tenant.
func (r *sqlRepository) GetByID(ctx context.Context, tenantID, repoID uuid.UUID) (*model.Repository, error) {
	var repo model.Repository
	var webhookID sql.NullString
	var lastScannedAt sql.NullTime

	err := r.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, name, url, provider, encrypted_credentials, webhook_secret, webhook_id, default_branch, last_scanned_at, created_at, updated_at
		 FROM repositories WHERE tenant_id = $1 AND id = $2`,
		tenantID, repoID,
	).Scan(
		&repo.ID,
		&repo.TenantID,
		&repo.Name,
		&repo.URL,
		&repo.Provider,
		&repo.EncryptedCredentials,
		&repo.WebhookSecret,
		&webhookID,
		&repo.DefaultBranch,
		&lastScannedAt,
		&repo.CreatedAt,
		&repo.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("repository not found")
		}
		return nil, fmt.Errorf("query repository: %w", err)
	}

	repo.WebhookID = webhookID
	repo.LastScannedAt = lastScannedAt
	return &repo, nil
}

// GetByIDAnyTenant retrieves a repository by ID without a tenant filter.
func (r *sqlRepository) GetByIDAnyTenant(ctx context.Context, repoID uuid.UUID) (*model.Repository, error) {
	var repo model.Repository
	var webhookID sql.NullString
	var lastScannedAt sql.NullTime

	err := r.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, name, url, provider, encrypted_credentials, webhook_secret, webhook_id, default_branch, last_scanned_at, created_at, updated_at
		 FROM repositories WHERE id = $1`,
		repoID,
	).Scan(
		&repo.ID,
		&repo.TenantID,
		&repo.Name,
		&repo.URL,
		&repo.Provider,
		&repo.EncryptedCredentials,
		&repo.WebhookSecret,
		&webhookID,
		&repo.DefaultBranch,
		&lastScannedAt,
		&repo.CreatedAt,
		&repo.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("repository not found")
		}
		return nil, fmt.Errorf("query repository for webhook: %w", err)
	}

	repo.WebhookID = webhookID
	repo.LastScannedAt = lastScannedAt
	return &repo, nil
}

// ListByTenant lists all repositories for a tenant.
func (r *sqlRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]model.Repository, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, tenant_id, name, url, provider, encrypted_credentials, webhook_secret, webhook_id, default_branch, last_scanned_at, created_at, updated_at
		 FROM repositories WHERE tenant_id = $1 ORDER BY created_at DESC`,
		tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("query repositories: %w", err)
	}
	defer rows.Close()

	var repos []model.Repository
	for rows.Next() {
		var repo model.Repository
		var webhookID sql.NullString
		var lastScannedAt sql.NullTime

		if err := rows.Scan(
			&repo.ID,
			&repo.TenantID,
			&repo.Name,
			&repo.URL,
			&repo.Provider,
			&repo.EncryptedCredentials,
			&repo.WebhookSecret,
			&webhookID,
			&repo.DefaultBranch,
			&lastScannedAt,
			&repo.CreatedAt,
			&repo.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan repository: %w", err)
		}

		repo.WebhookID = webhookID
		repo.LastScannedAt = lastScannedAt
		repos = append(repos, repo)
	}

	return repos, rows.Err()
}

// Create creates a new repository connection.
func (r *sqlRepository) Create(ctx context.Context, repo *model.Repository) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO repositories (id, tenant_id, name, url, provider, encrypted_credentials, webhook_secret, webhook_id, default_branch, last_scanned_at, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		repo.ID,
		repo.TenantID,
		repo.Name,
		repo.URL,
		repo.Provider,
		repo.EncryptedCredentials,
		repo.WebhookSecret,
		repo.WebhookID,
		repo.DefaultBranch,
		repo.LastScannedAt,
		repo.CreatedAt,
		repo.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert repository: %w", err)
	}
	return nil
}

// Update updates an existing repository connection.
func (r *sqlRepository) Update(ctx context.Context, repo *model.Repository) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE repositories SET name = $1, url = $2, provider = $3, encrypted_credentials = $4, webhook_secret = $5, webhook_id = $6, default_branch = $7, last_scanned_at = $8, updated_at = $9
		 WHERE id = $10 AND tenant_id = $11`,
		repo.Name,
		repo.URL,
		repo.Provider,
		repo.EncryptedCredentials,
		repo.WebhookSecret,
		repo.WebhookID,
		repo.DefaultBranch,
		repo.LastScannedAt,
		time.Now(),
		repo.ID,
		repo.TenantID,
	)
	if err != nil {
		return fmt.Errorf("update repository: %w", err)
	}
	return nil
}

// Delete deletes a repository connection.
func (r *sqlRepository) Delete(ctx context.Context, tenantID, repoID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM repositories WHERE id = $1 AND tenant_id = $2`,
		repoID, tenantID,
	)
	if err != nil {
		return fmt.Errorf("delete repository: %w", err)
	}
	return nil
}

// GetScanResult retrieves a scan result by ID.
func (r *sqlRepository) GetScanResult(ctx context.Context, tenantID, scanID uuid.UUID) (*model.ScanResult, error) {
	var result model.ScanResult
	var completedAt sql.NullTime

	err := r.db.QueryRowContext(ctx,
		`SELECT id, repository_id, tenant_id, branch, commit_hash, ai_uses, data_flows, sensitive_data, total_files, scanned_files, started_at, completed_at, created_at
		 FROM repository_scan_results WHERE tenant_id = $1 AND id = $2`,
		tenantID, scanID,
	).Scan(
		&result.ID,
		&result.RepositoryID,
		&result.TenantID,
		&result.Branch,
		&result.CommitHash,
		&result.AIUses,
		&result.DataFlows,
		&result.SensitiveData,
		&result.TotalFiles,
		&result.ScannedFiles,
		&result.StartedAt,
		&completedAt,
		&result.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("scan result not found")
		}
		return nil, fmt.Errorf("query scan result: %w", err)
	}

	result.CompletedAt = completedAt
	return &result, nil
}

// ListScanResults lists scan results for a repository.
func (r *sqlRepository) ListScanResults(ctx context.Context, repoID uuid.UUID) ([]model.ScanResult, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, repository_id, tenant_id, branch, commit_hash, ai_uses, data_flows, sensitive_data, total_files, scanned_files, started_at, completed_at, created_at
		 FROM repository_scan_results WHERE repository_id = $1 ORDER BY created_at DESC`,
		repoID,
	)
	if err != nil {
		return nil, fmt.Errorf("query scan results: %w", err)
	}
	defer rows.Close()

	var results []model.ScanResult
	for rows.Next() {
		var result model.ScanResult
		var completedAt sql.NullTime

		if err := rows.Scan(
			&result.ID,
			&result.RepositoryID,
			&result.TenantID,
			&result.Branch,
			&result.CommitHash,
			&result.AIUses,
			&result.DataFlows,
			&result.SensitiveData,
			&result.TotalFiles,
			&result.ScannedFiles,
			&result.StartedAt,
			&completedAt,
			&result.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan scan result: %w", err)
		}

		result.CompletedAt = completedAt
		results = append(results, result)
	}

	return results, rows.Err()
}

// CreateScanResult creates a new scan result record.
func (r *sqlRepository) CreateScanResult(ctx context.Context, result *model.ScanResult) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO repository_scan_results (id, repository_id, tenant_id, branch, commit_hash, ai_uses, data_flows, sensitive_data, total_files, scanned_files, started_at, completed_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		result.ID,
		result.RepositoryID,
		result.TenantID,
		result.Branch,
		result.CommitHash,
		result.AIUses,
		result.DataFlows,
		result.SensitiveData,
		result.TotalFiles,
		result.ScannedFiles,
		result.StartedAt,
		result.CompletedAt,
		result.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert scan result: %w", err)
	}
	return nil
}

// UpdateScanResult updates an existing scan result record.
func (r *sqlRepository) UpdateScanResult(ctx context.Context, result *model.ScanResult) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE repository_scan_results SET ai_uses = $1, data_flows = $2, sensitive_data = $3, total_files = $4, scanned_files = $5, completed_at = $6
		 WHERE id = $7 AND repository_id = $8`,
		result.AIUses,
		result.DataFlows,
		result.SensitiveData,
		result.TotalFiles,
		result.ScannedFiles,
		result.CompletedAt,
		result.ID,
		result.RepositoryID,
	)
	if err != nil {
		return fmt.Errorf("update scan result: %w", err)
	}
	return nil
}
