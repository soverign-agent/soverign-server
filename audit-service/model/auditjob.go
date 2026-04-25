package model

import (
	"time"

	"github.com/google/uuid"
)

// AuditJobStatus constants
const (
	AuditJobStatusPending   = "pending"
	AuditJobStatusRunning   = "running"
	AuditJobStatusPaused    = "paused"
	AuditJobStatusCompleted = "completed"
	AuditJobStatusFailed    = "failed"
	AuditJobStatusCancelled = "cancelled"
)

// AuditType constants
const (
	AuditTypeFull        = "full"
	AuditTypeIncremental = "incremental"
)

// RiskSeverity constants
const (
	RiskSeverityLow      = "low"
	RiskSeverityMedium   = "medium"
	RiskSeverityHigh     = "high"
	RiskSeverityCritical = "critical"
)

// AuditJob represents an audit job that processes a repository for EU AI Act compliance
type AuditJob struct {
	ID                 uuid.UUID  `json:"id"`
	TenantID           uuid.UUID  `json:"tenant_id"`
	RepositoryID       uuid.UUID  `json:"repository_id"`
	Name               string     `json:"name"`
	AuditType          string     `json:"audit_type"`
	Status             string     `json:"status"`
	RiskScore          int        `json:"risk_score"`
	RiskSeverity       string     `json:"risk_severity"`
	ProgressPercentage int        `json:"progress_percentage"`
	FindingsCount      int        `json:"findings_count"`
	CriticalFindings   int        `json:"critical_findings"`
	HighFindings       int        `json:"high_findings"`
	MediumFindings     int        `json:"medium_findings"`
	LowFindings        int        `json:"low_findings"`
	PreviousAuditID    *uuid.UUID `json:"previous_audit_id,omitempty"`
	WorkflowID         *string    `json:"workflow_id,omitempty"`
	StartedAt          *time.Time `json:"started_at,omitempty"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// AuditJobSummary is a lightweight summary for listing
type AuditJobSummary struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	AuditType     string    `json:"audit_type"`
	Status        string    `json:"status"`
	RiskScore     int       `json:"risk_score"`
	RiskSeverity  string    `json:"risk_severity"`
	FindingsCount int       `json:"findings_count"`
	CreatedAt     time.Time `json:"created_at"`
}
