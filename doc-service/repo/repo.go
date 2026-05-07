// Package repo provides data access layer for doc-service with RLS support.
package repo

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"sovereign-ai-compliance/doc-service/model"
	"sovereign-ai-compliance/shared/database"
)

// Repository defines the interface for document data access.
type Repository interface {
	// DB returns the underlying database connection.
	DB() *sql.DB

	// CreateDocument creates a new generated document.
	CreateDocument(ctx context.Context, doc *model.GeneratedDocument) error

	// GetDocumentByID retrieves a document by ID.
	GetDocumentByID(ctx context.Context, id uuid.UUID) (*model.GeneratedDocument, error)

	// ListDocuments lists documents for the current tenant with filtering.
	ListDocuments(ctx context.Context, aiSystemID *uuid.UUID, docType, status *string, page, pageSize int) ([]model.DocumentSummary, int, error)

	// UpdateDocument updates document content and metadata.
	UpdateDocument(ctx context.Context, doc *model.GeneratedDocument) error

	// UpdateDocumentStatus updates the status of a document.
	UpdateDocumentStatus(ctx context.Context, id uuid.UUID, status string) error

	// DeleteDocument deletes a document by ID.
	DeleteDocument(ctx context.Context, id uuid.UUID) error

	// CreateVersion creates a new document version snapshot.
	CreateVersion(ctx context.Context, version *model.DocumentVersion) error

	// GetVersionsForDocument retrieves version history for a document.
	GetVersionsForDocument(ctx context.Context, documentID uuid.UUID, page, pageSize int) ([]model.DocumentVersion, int, error)

	// GetVersionByID retrieves a specific version by ID.
	GetVersionByID(ctx context.Context, id uuid.UUID) (*model.DocumentVersion, error)

	// CreateExportJob creates a new export job.
	CreateExportJob(ctx context.Context, job *model.ExportJob) error

	// GetExportJobByID retrieves an export job by ID.
	GetExportJobByID(ctx context.Context, id uuid.UUID) (*model.ExportJob, error)

	// UpdateExportJobStatus updates the status and result of an export job.
	UpdateExportJobStatus(ctx context.Context, id uuid.UUID, status string, filePath *string, fileSize *int64, errorMessage *string) error
}

// SQLRepository implements the document repository with PostgreSQL and RLS.
type SQLRepository struct {
	base *database.BaseRepository
}

// NewSQLRepository creates a new SQLRepository.
func NewSQLRepository(db *sql.DB) *SQLRepository {
	return &SQLRepository{
		base: database.NewBaseRepository(db),
	}
}

// DB returns the underlying database connection.
func (r *SQLRepository) DB() *sql.DB {
	return r.base.DB()
}
