// Package repo provides data access layer for audit-service with RLS support.
package repo

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"sovereign-ai-compliance/audit-service/model"
	"sovereign-ai-compliance/shared/database"
)

// Repository defines the interface for audit data access.
type Repository interface {
	// DB returns the underlying database connection.
	DB() *sql.DB

	// CreateAudit creates a new audit job.
	CreateAudit(ctx context.Context, audit *model.AuditJob) error

	// GetAuditByID retrieves an audit job by ID.
	GetAuditByID(ctx context.Context, id uuid.UUID) (*model.AuditJob, error)

	// GetPreviousCompletedAudit gets the most recent completed audit for a repository (excluding the given audit ID).
	GetPreviousCompletedAudit(ctx context.Context, repositoryID, excludeAuditID uuid.UUID) (*model.AuditJob, error)

	// ListAudits lists audit jobs for the current tenant with filtering.
	// statuses is an optional set of status values; when non-empty rows must match any of them.
	ListAudits(ctx context.Context, repoID *uuid.UUID, statuses []string, page, pageSize int) ([]model.AuditJobSummary, int, error)

	// UpdateAuditStatus updates the status of an audit job.
	UpdateAuditStatus(ctx context.Context, id uuid.UUID, status string) error

	// UpdateAuditStep updates the status, current workflow step, and progress percentage in a single statement.
	UpdateAuditStep(ctx context.Context, id uuid.UUID, status, step string, percentage int) error

	// UpdateAuditFailure marks an audit as failed and records the step it died on plus the error message.
	UpdateAuditFailure(ctx context.Context, id uuid.UUID, step string, percentage int, errMsg string) error

	// UpdateAuditProgress updates audit progress and completion stats.
	UpdateAuditProgress(ctx context.Context, id uuid.UUID, progress int, riskScore int, severity string, findingsCount, critical, high, medium, low int) error

	// UpdateAuditWorkflowID sets the Temporal workflow ID for an audit.
	UpdateAuditWorkflowID(ctx context.Context, id uuid.UUID, workflowID string) error

	// CreateFinding creates a new audit finding.
	CreateFinding(ctx context.Context, finding *model.Finding) error

	// FindingExistsByLocation checks whether a finding already exists at the same file location for an audit.
	FindingExistsByLocation(ctx context.Context, auditID uuid.UUID, filePath string, lineNumber *int, issueType string) (bool, error)

	// GetFindingsForAudit gets all findings for an audit job.
	GetFindingsForAudit(ctx context.Context, auditID uuid.UUID) ([]model.Finding, error)

	// CountFindingsBySeverity counts findings by severity for an audit.
	CountFindingsBySeverity(ctx context.Context, auditID uuid.UUID) (critical, high, medium, low int, err error)

	// ListApprovalRequests lists approval requests for the current tenant.
	ListApprovalRequests(ctx context.Context, status string) ([]model.ApprovalRequest, error)

	// GetApprovalRequest retrieves an approval request by ID.
	GetApprovalRequest(ctx context.Context, id uuid.UUID) (*model.ApprovalRequest, error)

	// CreateApprovalRequest creates a new approval request.
	CreateApprovalRequest(ctx context.Context, req *model.ApprovalRequest) error

	// UpdateApprovalStatus updates the status of an approval request.
	UpdateApprovalStatus(ctx context.Context, id uuid.UUID, status, decidedBy string) error
}

// SQLRepository implements the audit repository with PostgreSQL and RLS.
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
