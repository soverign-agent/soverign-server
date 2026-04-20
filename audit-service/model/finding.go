package model

import (
	"time"

	"github.com/google/uuid"
)

// IssueType constants for EU AI Act categories
const (
	IssueTypeDataPrivacy    = "data_privacy"
	IssueTypeTransparency   = "transparency"
	IssueTypeHumanOversight = "human_oversight"
	IssueTypeAccuracy       = "accuracy"
	IssueTypeSecurity       = "security"
	IssueTypeRecordKeeping  = "record_keeping"
)

// Severity constants
const (
	SeverityCritical = "critical"
	SeverityHigh     = "high"
	SeverityMedium   = "medium"
	SeverityLow      = "low"
)

// Severity weights for risk scoring
const (
	SeverityWeightCritical = 50
	SeverityWeightHigh     = 25
	SeverityWeightMedium   = 10
	SeverityWeightLow      = 1
)

// Finding represents a single issue found during an audit
type Finding struct {
	ID          uuid.UUID `json:"id"`
	TenantID    uuid.UUID `json:"tenant_id"`
	AuditJobID  uuid.UUID `json:"audit_job_id"`
	FilePath    string    `json:"file_path"`
	LineNumber  *int      `json:"line_number,omitempty"`
	IssueType   string    `json:"issue_type"`
	Severity    string    `json:"severity"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Remediation *string   `json:"remediation,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// FindingSummary is a lightweight summary for findings
type FindingSummary struct {
	ID        uuid.UUID `json:"id"`
	IssueType string    `json:"issue_type"`
	Severity  string    `json:"severity"`
	Title     string    `json:"title"`
}

// IssueTypeInfo contains metadata about an EU AI Act issue type
type IssueTypeInfo struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// GetAllIssueTypes returns all EU AI Act issue types with metadata
func GetAllIssueTypes() []IssueTypeInfo {
	return []IssueTypeInfo{
		{
			Type:        IssueTypeDataPrivacy,
			Name:        "Data Privacy",
			Description: "Issues related to personal data protection and privacy compliance",
		},
		{
			Type:        IssueTypeTransparency,
			Name:        "Transparency",
			Description: "Issues related to documentation and explainability of AI systems",
		},
		{
			Type:        IssueTypeHumanOversight,
			Name:        "Human Oversight",
			Description: "Issues related to human agency and oversight mechanisms",
		},
		{
			Type:        IssueTypeAccuracy,
			Name:        "Accuracy",
			Description: "Issues related to model accuracy, reliability, and performance",
		},
		{
			Type:        IssueTypeSecurity,
			Name:        "Security",
			Description: "Issues related to cybersecurity and robustness against attacks",
		},
		{
			Type:        IssueTypeRecordKeeping,
			Name:        "Record Keeping",
			Description: "Issues related to documentation and audit trail requirements",
		},
	}
}
