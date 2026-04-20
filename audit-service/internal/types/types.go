// Package types defines request and response types for audit-service API endpoints.
package types

import (
	"github.com/google/uuid"
	"sovereign-ai-compliance/audit-service/model"
)

// TriggerAuditRequest is the request to trigger a new audit.
type TriggerAuditRequest struct {
	RepositoryID uuid.UUID `json:"repository_id"`
	Name         string    `json:"name"`
	AuditType    string    `json:"audit_type"` // full or incremental
}

// TriggerAuditResponse is the response after triggering an audit.
type TriggerAuditResponse struct {
	AuditID    uuid.UUID `json:"audit_id"`
	Status     string    `json:"status"`
	WorkflowID *string   `json:"workflow_id,omitempty"`
}

// ListAuditsRequest is the request to list audits with filtering.
type ListAuditsRequest struct {
	RepositoryID *uuid.UUID `json:"repository_id,omitempty"`
	Status       *string    `json:"status,omitempty"`
	Page         int        `json:"page"`
	PageSize     int        `json:"page_size"`
}

// ListAuditsResponse is the response containing a list of audits.
type ListAuditsResponse struct {
	Items []model.AuditJobSummary `json:"items"`
	Total int                     `json:"total"`
	Page  int                     `json:"page"`
	Pages int                     `json:"pages"`
}

// GetAuditRequest is the request to get a specific audit.
type GetAuditRequest struct {
	AuditID uuid.UUID `json:"-"`
}

// GetAuditResponse is the response containing the audit details with findings.
type GetAuditResponse struct {
	model.AuditJob
	Findings []model.Finding `json:"findings,omitempty"`
}

// PauseAuditRequest is the request to pause an audit.
type PauseAuditRequest struct {
	AuditID uuid.UUID `json:"-"`
}

// PauseAuditResponse is the response after pausing an audit.
type PauseAuditResponse struct {
	Success bool   `json:"success"`
	Status  string `json:"status"`
}

// ResumeAuditRequest is the request to resume an audit.
type ResumeAuditRequest struct {
	AuditID  uuid.UUID `json:"-"`
	Approved bool      `json:"approved"` // Whether the audit is approved after pause/high-risk check
}

// ResumeAuditResponse is the response after resuming an audit.
type ResumeAuditResponse struct {
	Success bool   `json:"success"`
	Status  string `json:"status"`
}

// GetAuditReportRequest is the request to get an audit report.
type GetAuditReportRequest struct {
	AuditID uuid.UUID `json:"-"`
}

// GetAuditReportResponse is the response containing the full audit report.
type GetAuditReportResponse struct {
	Audit       model.AuditJob             `json:"audit"`
	Findings    []model.Finding            `json:"findings"`
	BySeverity  map[string][]model.Finding `json:"by_severity"`
	ByIssueType map[string][]model.Finding `json:"by_issue_type"`
	Summary     AuditReportSummary         `json:"summary"`
}

// AuditReportSummary contains summary statistics for the report.
type AuditReportSummary struct {
	TotalFindings int    `json:"total_findings"`
	Critical      int    `json:"critical"`
	High          int    `json:"high"`
	Medium        int    `json:"medium"`
	Low           int    `json:"low"`
	RiskScore     int    `json:"risk_score"`
	RiskSeverity  string `json:"risk_severity"`
	Compliant     bool   `json:"compliant"` // Whether the audit passes compliance checks
}

// SSEProgressEvent represents a progress event for SSE streaming.
type SSEProgressEvent struct {
	Percentage int    `json:"percentage"`
	Step       string `json:"step"`
	Message    string `json:"message"`
	Timestamp  int64  `json:"timestamp"`
}

// ApproveAuditSignal is the signal sent to Temporal workflow to approve an audit.
type ApproveAuditSignal struct {
	Approved bool `json:"approved"`
}

// GetAuditStatusQuery is the query to get current audit status from Temporal.
type GetAuditStatusQuery struct {
}

// GetAuditStatusResult is the result of the status query.
type GetAuditStatusResult struct {
	Status      string `json:"status"`
	Progress    int    `json:"progress"`
	CurrentStep string `json:"current_step"`
}
