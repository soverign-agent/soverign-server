// Package logic provides business logic for audit-service.
package logic

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"sovereign-ai-compliance/audit-service/internal/types"
	"sovereign-ai-compliance/audit-service/model"
	"sovereign-ai-compliance/audit-service/repo"
	"sovereign-ai-compliance/audit-service/scoring"
	"sovereign-ai-compliance/shared/tenant"
)

// AuditLogic handles audit lifecycle business logic.
type AuditLogic struct {
	repo       repo.Repository
	calculator *scoring.Calculator
}

// NewAuditLogic creates a new AuditLogic.
func NewAuditLogic(repo repo.Repository, calculator *scoring.Calculator) *AuditLogic {
	return &AuditLogic{
		repo:       repo,
		calculator: calculator,
	}
}

// TriggerAudit creates a new audit job and returns it.
func (l *AuditLogic) TriggerAudit(ctx context.Context, req *types.TriggerAuditRequest) (*model.AuditJob, error) {
	tenantID, ok := tenant.FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("tenant context required")
	}

	tenantUUID, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, fmt.Errorf("invalid tenant ID: %w", err)
	}

	audit := &model.AuditJob{
		ID:                 uuid.New(),
		TenantID:           tenantUUID,
		RepositoryID:       req.RepositoryID,
		Name:               req.Name,
		AuditType:          req.AuditType,
		Status:             model.AuditJobStatusPending,
		RiskScore:          0,
		RiskSeverity:       model.RiskSeverityLow,
		ProgressPercentage: 0,
		FindingsCount:      0,
		CriticalFindings:   0,
		HighFindings:       0,
		MediumFindings:     0,
		LowFindings:        0,
	}

	if err := l.repo.CreateAudit(ctx, audit); err != nil {
		return nil, fmt.Errorf("create audit: %w", err)
	}

	return audit, nil
}

// ListAudits lists audits with filtering and pagination.
func (l *AuditLogic) ListAudits(ctx context.Context, req *types.ListAuditsRequest) ([]model.AuditJobSummary, int, error) {
	summaries, total, err := l.repo.ListAudits(ctx, req.RepositoryID, req.Status, req.Page, req.PageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("list audits: %w", err)
	}

	return summaries, total, nil
}

// GetAudit gets an audit by ID with all findings.
func (l *AuditLogic) GetAudit(ctx context.Context, id uuid.UUID) (*model.AuditJob, []model.Finding, error) {
	audit, err := l.repo.GetAuditByID(ctx, id)
	if err != nil {
		return nil, nil, fmt.Errorf("get audit: %w", err)
	}
	if audit == nil {
		return nil, nil, nil
	}

	findings, err := l.repo.GetFindingsForAudit(ctx, id)
	if err != nil {
		return audit, nil, fmt.Errorf("get findings: %w", err)
	}

	return audit, findings, nil
}

// PauseAudit pauses a running audit.
func (l *AuditLogic) PauseAudit(ctx context.Context, id uuid.UUID) error {
	audit, err := l.repo.GetAuditByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get audit: %w", err)
	}
	if audit == nil {
		return fmt.Errorf("audit not found")
	}

	if audit.Status != model.AuditJobStatusRunning {
		return fmt.Errorf("cannot pause audit in status: %s", audit.Status)
	}

	err = l.repo.UpdateAuditStatus(ctx, id, model.AuditJobStatusPaused)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	return nil
}

// ResumeAudit resumes a paused audit.
func (l *AuditLogic) ResumeAudit(ctx context.Context, id uuid.UUID) error {
	audit, err := l.repo.GetAuditByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get audit: %w", err)
	}
	if audit == nil {
		return fmt.Errorf("audit not found")
	}

	if audit.Status != model.AuditJobStatusPaused {
		return fmt.Errorf("cannot resume audit in status: %s", audit.Status)
	}

	err = l.repo.UpdateAuditStatus(ctx, id, model.AuditJobStatusRunning)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	return nil
}

// UpdateAuditCompletion updates the audit completion after workflow finishes.
func (l *AuditLogic) UpdateAuditCompletion(ctx context.Context, id uuid.UUID) error {
	critical, high, medium, low, err := l.repo.CountFindingsBySeverity(ctx, id)
	if err != nil {
		return fmt.Errorf("count findings: %w", err)
	}

	totalScore := l.calculator.CalculateCompositeScore(critical, high, medium, low)
	severity := l.calculator.ClassifySeverity(totalScore)
	totalFindings := critical + high + medium + low

	err = l.repo.UpdateAuditProgress(ctx, id, 100, totalScore, severity, totalFindings, critical, high, medium, low)
	if err != nil {
		return fmt.Errorf("update progress: %w", err)
	}

	err = l.repo.UpdateAuditStatus(ctx, id, model.AuditJobStatusCompleted)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	return nil
}

// GenerateReport generates a full audit report with grouping by severity and issue type.
func (l *AuditLogic) GenerateReport(ctx context.Context, id uuid.UUID) (*types.GetAuditReportResponse, error) {
	audit, findings, err := l.GetAudit(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get audit: %w", err)
	}
	if audit == nil {
		return nil, nil
	}

	bySeverity := l.calculator.GroupFindingsBySeverity(findings)
	byIssueType := l.calculator.GroupFindingsByIssueType(findings)

	summary := types.AuditReportSummary{
		TotalFindings: len(findings),
		Critical:      audit.CriticalFindings,
		High:          audit.HighFindings,
		Medium:        audit.MediumFindings,
		Low:           audit.LowFindings,
		RiskScore:     audit.RiskScore,
		RiskSeverity:  audit.RiskSeverity,
		Compliant:     audit.RiskScore < l.calculator.GetHighThreshold(),
	}

	return &types.GetAuditReportResponse{
		Audit:       *audit,
		Findings:    findings,
		BySeverity:  bySeverity,
		ByIssueType: byIssueType,
		Summary:     summary,
	}, nil
}

// UpdateWorkflowID updates the workflow ID for an audit.
func (l *AuditLogic) UpdateWorkflowID(ctx context.Context, auditID uuid.UUID, workflowID string) error {
	err := l.repo.UpdateAuditWorkflowID(ctx, auditID, workflowID)
	if err != nil {
		return fmt.Errorf("update workflow ID: %w", err)
	}
	return nil
}
