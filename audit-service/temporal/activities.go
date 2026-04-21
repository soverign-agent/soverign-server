package temporal

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"sovereign-ai-compliance/audit-service/internal/logic"
	"sovereign-ai-compliance/audit-service/model"
	"sovereign-ai-compliance/audit-service/repo"
	"sovereign-ai-compliance/audit-service/scoring"
	"time"
)

// AnalysisFinding represents a single static analysis finding from repo-service.
type AnalysisFinding struct {
	FilePath    string `json:"file_path"`
	LineNumber  int    `json:"line_number"`
	IssueType   string `json:"issue_type"`
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Remediation string `json:"remediation"`
}

// RepoServiceClient is the interface for repo-service gRPC client.
// When protobuf contracts are generated, this interface will be implemented by the generated client.
type RepoServiceClient interface {
	// FetchRepository fetches repository information and code.
	FetchRepository(ctx context.Context, repositoryID uuid.UUID) error
	// RunStaticAnalysis runs static analysis on the repository.
	RunStaticAnalysis(ctx context.Context, repositoryID uuid.UUID) error
	// GetAnalysisResults gets the static analysis findings.
	GetAnalysisResults(ctx context.Context, repositoryID uuid.UUID) ([]AnalysisFinding, error)
}

// NotificationServiceClient is the interface for notification-service gRPC client.
// When protobuf contracts are generated, this interface will be implemented by the generated client.
type NotificationServiceClient interface {
	// SendAuditCompletedNotification sends a notification that audit completed.
	SendAuditCompletedNotification(ctx context.Context, auditID uuid.UUID) error
}

// Activities contains all activities for the compliance audit workflow.
type Activities struct {
	repo                 repo.Repository
	logic                *logic.AuditLogic
	calculator            *scoring.Calculator
	repoServiceClient    RepoServiceClient
	notificationClient   NotificationServiceClient
}

// NewActivities creates a new Activities instance.
func NewActivities(
	repo repo.Repository,
	logic *logic.AuditLogic,
	calculator *scoring.Calculator,
	repoServiceClient RepoServiceClient,
	notificationClient NotificationServiceClient,
) *Activities {
	return &Activities{
		repo:                 repo,
		logic:                logic,
		calculator:            calculator,
		repoServiceClient:    repoServiceClient,
		notificationClient:   notificationClient,
	}
}

// InitializeAudit updates the audit status to running and sets the start time.
func (a *Activities) InitializeAudit(ctx context.Context, auditID string) error {
	auditUUID, err := uuid.Parse(auditID)
	if err != nil {
		return fmt.Errorf("invalid audit ID: %w", err)
	}

	audit, err := a.repo.GetAuditByID(ctx, auditUUID)
	if err != nil {
		return fmt.Errorf("get audit: %w", err)
	}
	if audit == nil {
		return fmt.Errorf("audit not found: %s", auditID)
	}

	now := time.Now()
	// Update status and started_at
	// We do this manually because UpdateAuditProgress doesn't set started_at
	// Since this is a Temporal activity, we expect it to be quick - no RLS issue here
	tx, err := a.repo.DB().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`UPDATE audit_jobs SET status = $1, started_at = $2, updated_at = NOW() WHERE id = $3`,
		model.AuditJobStatusRunning, now, auditUUID,
	)
	if err != nil {
		return fmt.Errorf("update audit: %w", err)
	}

	return tx.Commit()
}

// FetchRepository fetches the repository code from repo-service.
// Calls repo-service via gRPC to get the repository content.
func (a *Activities) FetchRepository(ctx context.Context, auditID string) error {
	if a.repoServiceClient == nil {
		// No client configured - this is expected when running without full integration
		// Placeholder implementation succeeds to allow workflow to continue
		return nil
	}

	auditUUID, err := uuid.Parse(auditID)
	if err != nil {
		return fmt.Errorf("invalid audit ID: %w", err)
	}

	audit, err := a.repo.GetAuditByID(ctx, auditUUID)
	if err != nil {
		return fmt.Errorf("get audit: %w", err)
	}
	if audit == nil {
		return fmt.Errorf("audit not found: %s", auditID)
	}

	return a.repoServiceClient.FetchRepository(ctx, audit.RepositoryID)
}

// RunStaticAnalysis runs static analysis on the repository code.
// Reuses static analysis from repo-service that's already been done.
func (a *Activities) RunStaticAnalysis(ctx context.Context, auditID string) error {
	if a.repoServiceClient == nil {
		// No client configured - placeholder implementation
		return nil
	}

	auditUUID, err := uuid.Parse(auditID)
	if err != nil {
		return fmt.Errorf("invalid audit ID: %w", err)
	}

	audit, err := a.repo.GetAuditByID(ctx, auditUUID)
	if err != nil {
		return fmt.Errorf("get audit: %w", err)
	}
	if audit == nil {
		return fmt.Errorf("audit not found: %s", auditID)
	}

	return a.repoServiceClient.RunStaticAnalysis(ctx, audit.RepositoryID)
}

// GenerateFindings generates findings from the static analysis results.
func (a *Activities) GenerateFindings(ctx context.Context, auditID string) error {
	if a.repoServiceClient == nil {
		// No client configured - skip finding generation to allow workflow to continue
		return nil
	}

	auditUUID, err := uuid.Parse(auditID)
	if err != nil {
		return fmt.Errorf("invalid audit ID: %w", err)
	}

	audit, err := a.repo.GetAuditByID(ctx, auditUUID)
	if err != nil {
		return fmt.Errorf("get audit: %w", err)
	}
	if audit == nil {
		return fmt.Errorf("audit not found: %s", auditID)
	}

	findings, err := a.repoServiceClient.GetAnalysisResults(ctx, audit.RepositoryID)
	if err != nil {
		return fmt.Errorf("get analysis results: %w", err)
	}

	for _, af := range findings {
		finding := &model.Finding{
			ID:          uuid.New(),
			TenantID:    audit.TenantID,
			AuditJobID:  auditUUID,
			FilePath:    af.FilePath,
			IssueType:   normalizeIssueType(af.IssueType),
			Severity:    normalizeSeverity(af.Severity),
			Title:       af.Title,
			Description: af.Description,
			CreatedAt:   time.Now(),
		}

		if af.LineNumber > 0 {
			lineNum := af.LineNumber
			finding.LineNumber = &lineNum
		}

		if af.Remediation != "" {
			rem := af.Remediation
			finding.Remediation = &rem
		}

		if err := a.repo.CreateFinding(ctx, finding); err != nil {
			return fmt.Errorf("create finding for file %s: %w", af.FilePath, err)
		}
	}

	return nil
}

// normalizeIssueType maps incoming issue types to the canonical model values.
func normalizeIssueType(t string) string {
	switch t {
	case model.IssueTypeDataPrivacy, model.IssueTypeTransparency,
		model.IssueTypeHumanOversight, model.IssueTypeAccuracy,
		model.IssueTypeSecurity, model.IssueTypeRecordKeeping:
		return t
	default:
		return model.IssueTypeSecurity
	}
}

// normalizeSeverity maps incoming severities to the canonical model values.
func normalizeSeverity(s string) string {
	switch s {
	case model.SeverityCritical, model.SeverityHigh,
		model.SeverityMedium, model.SeverityLow:
		return s
	default:
		return model.SeverityMedium
	}
}

// CalculateRiskScore calculates the risk score from all findings.
func (a *Activities) CalculateRiskScore(ctx context.Context, auditID string) error {
	auditUUID, err := uuid.Parse(auditID)
	if err != nil {
		return fmt.Errorf("invalid audit ID: %w", err)
	}

	critical, high, medium, low, err := a.repo.CountFindingsBySeverity(ctx, auditUUID)
	if err != nil {
		return fmt.Errorf("count findings: %w", err)
	}

	score := a.calculator.CalculateCompositeScore(critical, high, medium, low)
	severity := a.calculator.ClassifySeverity(score)
	total := critical + high + medium + low

	err = a.repo.UpdateAuditProgress(ctx, auditUUID, 50, score, severity, total, critical, high, medium, low)
	if err != nil {
		return fmt.Errorf("update progress: %w", err)
	}

	return nil
}

// CheckApprovalGate checks if approval is required based on risk score.
// Returns true if approval is required (high or critical risk).
func (a *Activities) CheckApprovalGate(ctx context.Context, auditID string) (bool, error) {
	auditUUID, err := uuid.Parse(auditID)
	if err != nil {
		return false, fmt.Errorf("invalid audit ID: %w", err)
	}

	audit, err := a.repo.GetAuditByID(ctx, auditUUID)
	if err != nil {
		return false, fmt.Errorf("get audit: %w", err)
	}

	return a.calculator.IsApprovalRequired(audit.RiskScore), nil
}

// UpdateAuditStatus updates audit status and progress (called from workflow).
func (a *Activities) UpdateAuditStatus(ctx context.Context, auditID string, status string, progress int, step string) error {
	auditUUID, err := uuid.Parse(auditID)
	if err != nil {
		return fmt.Errorf("invalid audit ID: %w", err)
	}

	err = a.repo.UpdateAuditStatus(ctx, auditUUID, status)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	// We don't update score here, just progress
	// Progress is just for UI display
	audit, err := a.repo.GetAuditByID(ctx, auditUUID)
	if err != nil {
		return fmt.Errorf("get audit: %w", err)
	}

	err = a.repo.UpdateAuditProgress(
		ctx, auditUUID, progress,
		audit.RiskScore, audit.RiskSeverity,
		audit.FindingsCount, audit.CriticalFindings,
		audit.HighFindings, audit.MediumFindings, audit.LowFindings,
	)
	if err != nil {
		return fmt.Errorf("update progress: %w", err)
	}

	return nil
}

// CancelAudit marks an audit as cancelled after rejection.
func (a *Activities) CancelAudit(ctx context.Context, auditID string) error {
	auditUUID, err := uuid.Parse(auditID)
	if err != nil {
		return fmt.Errorf("invalid audit ID: %w", err)
	}

	err = a.repo.UpdateAuditStatus(ctx, auditUUID, model.AuditJobStatusCancelled)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	return nil
}

// GenerateReport generates the final audit report after all processing.
func (a *Activities) GenerateReport(ctx context.Context, auditID string) error {
	// The report is generated on-demand when requested by the API
	// This activity doesn't need to do anything because the report is built
	// from existing audit and finding data in the database
	return nil
}

// NotifyCompletion sends a notification that the audit has completed.
func (a *Activities) NotifyCompletion(ctx context.Context, auditID string) error {
	if a.notificationClient == nil {
		// No client configured - placeholder implementation
		return nil
	}

	auditUUID, err := uuid.Parse(auditID)
	if err != nil {
		return fmt.Errorf("invalid audit ID: %w", err)
	}

	return a.notificationClient.SendAuditCompletedNotification(ctx, auditUUID)
}

// CompleteAudit marks the audit as completed and sets completion time.
func (a *Activities) CompleteAudit(ctx context.Context, auditID string) error {
	auditUUID, err := uuid.Parse(auditID)
	if err != nil {
		return fmt.Errorf("invalid audit ID: %w", err)
	}

	err = a.logic.UpdateAuditCompletion(ctx, auditUUID)
	if err != nil {
		return fmt.Errorf("update audit completion: %w", err)
	}

	return nil
}
