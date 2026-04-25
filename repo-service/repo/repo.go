// Package repo defines the repository interface and implementations for repository service.
package repo

import (
	"context"

	"github.com/google/uuid"

	"sovereign-ai-compliance/repo-service/model"
)

// Repository defines the data access interface for repository management.
type Repository interface {
	// GetByID retrieves a repository by ID for the given tenant.
	GetByID(ctx context.Context, tenantID, repoID uuid.UUID) (*model.Repository, error)
	// GetByIDAnyTenant retrieves a repository by ID for unauthenticated webhook callbacks.
	GetByIDAnyTenant(ctx context.Context, repoID uuid.UUID) (*model.Repository, error)
	// GetByURL retrieves a repository by URL for the given tenant.
	GetByURL(ctx context.Context, tenantID uuid.UUID, url string) (*model.Repository, error)
	// ListByTenant lists all repositories for a tenant.
	ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]model.Repository, error)
	// Create creates a new repository connection.
	Create(ctx context.Context, repo *model.Repository) error
	// Update updates an existing repository connection.
	Update(ctx context.Context, repo *model.Repository) error
	// Delete deletes a repository connection (soft delete not needed as it's a full removal).
	Delete(ctx context.Context, tenantID, repoID uuid.UUID) error
	// GetScanResult retrieves a scan result by ID.
	GetScanResult(ctx context.Context, tenantID, scanID uuid.UUID) (*model.ScanResult, error)
	// ListScanResults lists scan results for a repository.
	ListScanResults(ctx context.Context, repoID uuid.UUID) ([]model.ScanResult, error)
	// CreateScanResult creates a new scan result record.
	CreateScanResult(ctx context.Context, result *model.ScanResult) error
	// UpdateScanResult updates an existing scan result record.
	UpdateScanResult(ctx context.Context, result *model.ScanResult) error
}
