package repo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"sovereign-ai-compliance/audit-service/model"
	"sovereign-ai-compliance/shared/tenant"
)

// CreateAudit creates a new audit job.
func (r *SQLRepository) CreateAudit(ctx context.Context, audit *model.AuditJob) error {
	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	query := `
		INSERT INTO audit_jobs (id, tenant_id, repository_id, name, audit_type, previous_audit_id, status, risk_score, risk_severity)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at, updated_at`

	err = tx.QueryRowContext(
		ctx, query,
		audit.ID, audit.TenantID, audit.RepositoryID, audit.Name,
		audit.AuditType, audit.PreviousAuditID, audit.Status, audit.RiskScore, audit.RiskSeverity,
	).Scan(&audit.CreatedAt, &audit.UpdatedAt)

	if err != nil {
		return fmt.Errorf("insert audit: %w", err)
	}

	return tx.Commit()
}

// GetAuditByID retrieves an audit job by ID.
func (r *SQLRepository) GetAuditByID(ctx context.Context, id uuid.UUID) (*model.AuditJob, error) {
	tenantID, ok := tenant.FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("tenant context required")
	}

	query := `
		SELECT id, tenant_id, repository_id, name, audit_type, status,
		       COALESCE(current_step, '') AS current_step,
		       COALESCE(error_message, '') AS error_message,
		       risk_score, risk_severity, progress_percentage, findings_count,
		       critical_findings, high_findings, medium_findings, low_findings,
		       previous_audit_id, workflow_id, started_at, completed_at, created_at, updated_at
		FROM audit_jobs
		WHERE id = $1`

	var audit model.AuditJob
	var prevAuditID sql.NullString
	err := r.base.DB().QueryRowContext(ctx, query, id).Scan(
		&audit.ID, &audit.TenantID, &audit.RepositoryID, &audit.Name,
		&audit.AuditType, &audit.Status, &audit.CurrentStep, &audit.ErrorMessage,
		&audit.RiskScore, &audit.RiskSeverity,
		&audit.ProgressPercentage, &audit.FindingsCount, &audit.CriticalFindings,
		&audit.HighFindings, &audit.MediumFindings, &audit.LowFindings,
		&prevAuditID, &audit.WorkflowID, &audit.StartedAt, &audit.CompletedAt,
		&audit.CreatedAt, &audit.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get audit: %w", err)
	}

	// Double-check tenant access (RLS should already handle this)
	if audit.TenantID.String() != tenantID {
		return nil, nil
	}

	if prevAuditID.Valid {
		pid, err := uuid.Parse(prevAuditID.String)
		if err == nil {
			audit.PreviousAuditID = &pid
		}
	}

	return &audit, nil
}

// GetPreviousCompletedAudit returns the most recent completed audit for a repository, excluding the given audit ID.
func (r *SQLRepository) GetPreviousCompletedAudit(ctx context.Context, repositoryID, excludeAuditID uuid.UUID) (*model.AuditJob, error) {
	tenantID, ok := tenant.FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("tenant context required")
	}

	query := `
		SELECT id, tenant_id, repository_id, name, audit_type, status,
		       COALESCE(current_step, '') AS current_step,
		       COALESCE(error_message, '') AS error_message,
		       risk_score, risk_severity, progress_percentage, findings_count,
		       critical_findings, high_findings, medium_findings, low_findings,
		       previous_audit_id, workflow_id, started_at, completed_at, created_at, updated_at
		FROM audit_jobs
		WHERE repository_id = $1 AND status = $2 AND id != $3 AND tenant_id = $4
		ORDER BY completed_at DESC NULLS LAST, created_at DESC
		LIMIT 1`

	var audit model.AuditJob
	var prevAuditID sql.NullString
	err := r.base.DB().QueryRowContext(ctx, query, repositoryID, model.AuditJobStatusCompleted, excludeAuditID, tenantID).Scan(
		&audit.ID, &audit.TenantID, &audit.RepositoryID, &audit.Name,
		&audit.AuditType, &audit.Status, &audit.CurrentStep, &audit.ErrorMessage,
		&audit.RiskScore, &audit.RiskSeverity,
		&audit.ProgressPercentage, &audit.FindingsCount, &audit.CriticalFindings,
		&audit.HighFindings, &audit.MediumFindings, &audit.LowFindings,
		&prevAuditID, &audit.WorkflowID, &audit.StartedAt, &audit.CompletedAt,
		&audit.CreatedAt, &audit.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get previous completed audit: %w", err)
	}

	if prevAuditID.Valid {
		pid, err := uuid.Parse(prevAuditID.String)
		if err == nil {
			audit.PreviousAuditID = &pid
		}
	}

	return &audit, nil
}

// ListAudits lists audit jobs for the current tenant with filtering.
// statuses is an optional set of status values; when non-empty rows must match any of them.
func (r *SQLRepository) ListAudits(ctx context.Context, repoID *uuid.UUID, statuses []string, page, pageSize int) ([]model.AuditJobSummary, int, error) {
	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	// Build query with filters
	var whereClause string
	var args []interface{}
	argIdx := 1

	if repoID != nil {
		whereClause += " WHERE repository_id = $" + fmt.Sprint(argIdx)
		args = append(args, *repoID)
		argIdx++
	}
	if len(statuses) > 0 {
		if whereClause == "" {
			whereClause += " WHERE"
		} else {
			whereClause += " AND"
		}
		whereClause += " status = ANY($" + fmt.Sprint(argIdx) + "::text[])"
		args = append(args, pq.Array(statuses))
		argIdx++
	}

	// Get total count
	var total int
	countQuery := "SELECT COUNT(*) FROM audit_jobs" + whereClause
	err = tx.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count audits: %w", err)
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	// Get paginated results
	query := `
		SELECT id, name, audit_type, status,
		       COALESCE(current_step, '') AS current_step,
		       progress_percentage,
		       risk_score, risk_severity, findings_count, created_at
		FROM audit_jobs` + whereClause + `
		ORDER BY created_at DESC
		LIMIT $` + fmt.Sprint(argIdx) + ` OFFSET $` + fmt.Sprint(argIdx+1)

	args = append(args, pageSize, offset)
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query audits: %w", err)
	}
	defer rows.Close()

	var summaries []model.AuditJobSummary
	for rows.Next() {
		var summary model.AuditJobSummary
		err := rows.Scan(
			&summary.ID, &summary.Name, &summary.AuditType, &summary.Status,
			&summary.CurrentStep, &summary.ProgressPercentage,
			&summary.RiskScore, &summary.RiskSeverity, &summary.FindingsCount,
			&summary.CreatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan audit summary: %w", err)
		}
		summaries = append(summaries, summary)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows iteration: %w", err)
	}

	return summaries, total, tx.Commit()
}

// UpdateAuditStatus updates the status of an audit job.
func (r *SQLRepository) UpdateAuditStatus(ctx context.Context, id uuid.UUID, status string) error {
	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`UPDATE audit_jobs SET status = $1, updated_at = NOW() WHERE id = $2`,
		status, id,
	)
	if err != nil {
		return fmt.Errorf("update audit status: %w", err)
	}

	return tx.Commit()
}

// UpdateAuditStep updates status, current workflow step, and progress percentage in one statement.
// Used by the workflow to surface step-level progress to the streaming/list endpoints.
func (r *SQLRepository) UpdateAuditStep(ctx context.Context, id uuid.UUID, status, step string, percentage int) error {
	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`UPDATE audit_jobs
		 SET status = $1, current_step = $2, progress_percentage = $3, updated_at = NOW()
		 WHERE id = $4`,
		status, step, percentage, id,
	)
	if err != nil {
		return fmt.Errorf("update audit step: %w", err)
	}

	return tx.Commit()
}

// UpdateAuditFailure marks an audit as failed and records the step it died on plus the error message.
// Used by the workflow's failure paths so the UI can show users which step crashed and why.
func (r *SQLRepository) UpdateAuditFailure(ctx context.Context, id uuid.UUID, step string, percentage int, errMsg string) error {
	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`UPDATE audit_jobs
		 SET status = $1, current_step = $2, progress_percentage = $3, error_message = $4, updated_at = NOW()
		 WHERE id = $5`,
		model.AuditJobStatusFailed, step, percentage, errMsg, id,
	)
	if err != nil {
		return fmt.Errorf("update audit failure: %w", err)
	}

	return tx.Commit()
}

// UpdateAuditProgress updates audit progress and completion stats.
func (r *SQLRepository) UpdateAuditProgress(ctx context.Context, id uuid.UUID, progress int, riskScore int, severity string, findingsCount, critical, high, medium, low int) error {
	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`UPDATE audit_jobs
		 SET progress_percentage = $1, risk_score = $2, risk_severity = $3,
		     findings_count = $4, critical_findings = $5, high_findings = $6,
		     medium_findings = $7, low_findings = $8, updated_at = NOW()
		 WHERE id = $9`,
		progress, riskScore, severity, findingsCount, critical, high, medium, low, id,
	)
	if err != nil {
		return fmt.Errorf("update audit progress: %w", err)
	}

	return tx.Commit()
}

// UpdateAuditWorkflowID sets the Temporal workflow ID for an audit.
func (r *SQLRepository) UpdateAuditWorkflowID(ctx context.Context, id uuid.UUID, workflowID string) error {
	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`UPDATE audit_jobs SET workflow_id = $1, updated_at = NOW() WHERE id = $2`,
		workflowID, id,
	)
	if err != nil {
		return fmt.Errorf("update workflow ID: %w", err)
	}

	return tx.Commit()
}

// CreateFinding creates a new audit finding.
func (r *SQLRepository) CreateFinding(ctx context.Context, finding *model.Finding) error {
	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	query := `
		INSERT INTO audit_findings (id, tenant_id, audit_job_id, file_path, line_number,
		                           issue_type, severity, title, description, remediation)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING created_at`

	err = tx.QueryRowContext(
		ctx, query,
		finding.ID, finding.TenantID, finding.AuditJobID, finding.FilePath,
		finding.LineNumber, finding.IssueType, finding.Severity, finding.Title,
		finding.Description, finding.Remediation,
	).Scan(&finding.CreatedAt)

	if err != nil {
		return fmt.Errorf("insert finding: %w", err)
	}

	return tx.Commit()
}

// FindingExistsByLocation checks whether a finding already exists at the same file location for an audit.
func (r *SQLRepository) FindingExistsByLocation(ctx context.Context, auditID uuid.UUID, filePath string, lineNumber *int, issueType string) (bool, error) {
	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	var query string
	var args []interface{}
	if lineNumber != nil {
		query = `
			SELECT EXISTS(
				SELECT 1 FROM audit_findings
				WHERE audit_job_id = $1 AND file_path = $2 AND line_number = $3 AND issue_type = $4
			)`
		args = []interface{}{auditID, filePath, *lineNumber, issueType}
	} else {
		query = `
			SELECT EXISTS(
				SELECT 1 FROM audit_findings
				WHERE audit_job_id = $1 AND file_path = $2 AND line_number IS NULL AND issue_type = $3
			)`
		args = []interface{}{auditID, filePath, issueType}
	}

	var exists bool
	err = tx.QueryRowContext(ctx, query, args...).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check finding existence: %w", err)
	}

	return exists, tx.Commit()
}

// GetFindingsForAudit gets all findings for an audit job.
func (r *SQLRepository) GetFindingsForAudit(ctx context.Context, auditID uuid.UUID) ([]model.Finding, error) {
	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	query := `
		SELECT id, tenant_id, audit_job_id, file_path, line_number,
		       issue_type, severity, title, description, remediation, created_at
		FROM audit_findings
		WHERE audit_job_id = $1
		ORDER BY severity DESC, created_at ASC`

	rows, err := tx.QueryContext(ctx, query, auditID)
	if err != nil {
		return nil, fmt.Errorf("query findings: %w", err)
	}
	defer rows.Close()

	var findings []model.Finding
	for rows.Next() {
		var finding model.Finding
		err := rows.Scan(
			&finding.ID, &finding.TenantID, &finding.AuditJobID, &finding.FilePath,
			&finding.LineNumber, &finding.IssueType, &finding.Severity, &finding.Title,
			&finding.Description, &finding.Remediation, &finding.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan finding: %w", err)
		}
		findings = append(findings, finding)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	return findings, tx.Commit()
}

// ListApprovalRequests lists approval requests for the current tenant filtered by status.
func (r *SQLRepository) ListApprovalRequests(ctx context.Context, status string) ([]model.ApprovalRequest, error) {
	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	query := `
		SELECT id, tenant_id, audit_job_id, requested_by, assigned_to, title, context_json, status, created_at, updated_at, decided_at, decided_by
		FROM approval_requests`
	var args []interface{}
	if status != "" {
		query += " WHERE status = $1"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC"

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query approval requests: %w", err)
	}
	defer rows.Close()

	var requests []model.ApprovalRequest
	for rows.Next() {
		var req model.ApprovalRequest
		var decidedAt sql.NullTime
		var decidedBy sql.NullString
		err := rows.Scan(
			&req.ID, &req.TenantID, &req.AuditJobID, &req.RequestedBy, pq.Array(&req.AssignedTo),
			&req.Title, &req.ContextJSON, &req.Status, &req.CreatedAt, &req.UpdatedAt,
			&decidedAt, &decidedBy,
		)
		if err != nil {
			return nil, fmt.Errorf("scan approval request: %w", err)
		}
		if decidedAt.Valid {
			req.DecidedAt = &decidedAt.Time
		}
		if decidedBy.Valid {
			req.DecidedBy = &decidedBy.String
		}
		requests = append(requests, req)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	return requests, tx.Commit()
}

// GetApprovalRequest retrieves an approval request by ID.
func (r *SQLRepository) GetApprovalRequest(ctx context.Context, id uuid.UUID) (*model.ApprovalRequest, error) {
	query := `
		SELECT id, tenant_id, audit_job_id, requested_by, assigned_to, title, context_json, status, created_at, updated_at, decided_at, decided_by
		FROM approval_requests
		WHERE id = $1`

	var req model.ApprovalRequest
	var decidedAt sql.NullTime
	var decidedBy sql.NullString
	err := r.base.DB().QueryRowContext(ctx, query, id).Scan(
		&req.ID, &req.TenantID, &req.AuditJobID, &req.RequestedBy, pq.Array(&req.AssignedTo),
		&req.Title, &req.ContextJSON, &req.Status, &req.CreatedAt, &req.UpdatedAt,
		&decidedAt, &decidedBy,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get approval request: %w", err)
	}

	if decidedAt.Valid {
		req.DecidedAt = &decidedAt.Time
	}
	if decidedBy.Valid {
		req.DecidedBy = &decidedBy.String
	}

	return &req, nil
}

// CreateApprovalRequest creates a new approval request.
func (r *SQLRepository) CreateApprovalRequest(ctx context.Context, req *model.ApprovalRequest) error {
	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	query := `
		INSERT INTO approval_requests (id, tenant_id, audit_job_id, requested_by, assigned_to, title, context_json, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING created_at, updated_at`

	err = tx.QueryRowContext(
		ctx, query,
		req.ID, req.TenantID, req.AuditJobID, req.RequestedBy, pq.Array(req.AssignedTo),
		req.Title, req.ContextJSON, req.Status,
	).Scan(&req.CreatedAt, &req.UpdatedAt)

	if err != nil {
		return fmt.Errorf("insert approval request: %w", err)
	}

	return tx.Commit()
}

// UpdateApprovalStatus updates the status of an approval request.
func (r *SQLRepository) UpdateApprovalStatus(ctx context.Context, id uuid.UUID, status, decidedBy string) error {
	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`UPDATE approval_requests SET status = $1, decided_by = $2, decided_at = NOW(), updated_at = NOW() WHERE id = $3`,
		status, decidedBy, id,
	)
	if err != nil {
		return fmt.Errorf("update approval status: %w", err)
	}

	return tx.Commit()
}

// CountFindingsBySeverity counts findings by severity for an audit.
func (r *SQLRepository) CountFindingsBySeverity(ctx context.Context, auditID uuid.UUID) (critical, high, medium, low int, err error) {
	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
		SELECT severity, COUNT(*)
		FROM audit_findings
		WHERE audit_job_id = $1
		GROUP BY severity
	`, auditID)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("count findings by severity: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var sev string
		var count int
		if err := rows.Scan(&sev, &count); err != nil {
			return 0, 0, 0, 0, fmt.Errorf("scan count: %w", err)
		}
		switch sev {
		case model.SeverityCritical:
			critical = count
		case model.SeverityHigh:
			high = count
		case model.SeverityMedium:
			medium = count
		case model.SeverityLow:
			low = count
		}
	}

	if err = rows.Err(); err != nil {
		return 0, 0, 0, 0, fmt.Errorf("rows iteration: %w", err)
	}

	return critical, high, medium, low, tx.Commit()
}
