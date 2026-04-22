// Package model defines the domain models for doc-service.
package model

import (
	"time"

	"github.com/google/uuid"
)

// Document status constants.
const (
	StatusGenerating = "generating"
	StatusEditing    = "editing"
	StatusApproved   = "approved"
	StatusPublished  = "published"
	StatusFailed     = "failed"
)

// Document type constants.
const (
	DocTypeAnnexIV    = "annex_iv"
	DocTypeRiskReport = "risk_report"
	DocTypeCompliance = "compliance_summary"
)

// Export format constants.
const (
	ExportFormatPDF  = "pdf"
	ExportFormatDOCX = "docx"
)

// Export status constants.
const (
	ExportStatusPending    = "pending"
	ExportStatusProcessing = "processing"
	ExportStatusCompleted  = "completed"
	ExportStatusFailed     = "failed"
)

// GeneratedDocument represents a compliance document.
type GeneratedDocument struct {
	ID         uuid.UUID       `json:"id"`
	TenantID   uuid.UUID       `json:"tenant_id"`
	AISystemID uuid.UUID       `json:"ai_system_id"`
	DocType    string          `json:"doc_type"`
	Title      string          `json:"title"`
	Content    DocumentContent `json:"content"`
	Version    int             `json:"version"`
	Status     string          `json:"status"`
	CreatedBy  uuid.UUID       `json:"created_by"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// DocumentContent holds the structured document with sections and metadata.
type DocumentContent struct {
	Sections []Section         `json:"sections,optional"`
	Metadata map[string]string `json:"metadata,optional"`
}

// Section represents a document section.
type Section struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Content     string       `json:"content"`
	Order       int          `json:"order"`
	SubSections []SubSection `json:"subsections,optional"`
}

// SubSection represents a document subsection.
type SubSection struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Order   int    `json:"order"`
}

// DocumentVersion tracks version history for a document.
type DocumentVersion struct {
	ID            uuid.UUID       `json:"id"`
	TenantID      uuid.UUID       `json:"tenant_id"`
	DocumentID    uuid.UUID       `json:"document_id"`
	VersionNumber int             `json:"version_number"`
	Content       DocumentContent `json:"content"`
	CreatedBy     uuid.UUID       `json:"created_by"`
	CreatedAt     time.Time       `json:"created_at"`
	ChangeSummary string          `json:"change_summary,omitempty"`
}

// ExportJob represents an async document export operation.
type ExportJob struct {
	ID           uuid.UUID  `json:"id"`
	TenantID     uuid.UUID  `json:"tenant_id"`
	DocumentID   uuid.UUID  `json:"document_id"`
	Format       string     `json:"format"`
	Status       string     `json:"status"`
	FilePath     *string    `json:"file_path,omitempty"`
	FileSize     *int64     `json:"file_size,omitempty"`
	ErrorMessage *string    `json:"error_message,omitempty"`
	CreatedBy    uuid.UUID  `json:"created_by"`
	CreatedAt    time.Time  `json:"created_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

// DocumentSummary is a lightweight summary for listing documents.
type DocumentSummary struct {
	ID         uuid.UUID `json:"id"`
	AISystemID uuid.UUID `json:"ai_system_id"`
	DocType    string    `json:"doc_type"`
	Title      string    `json:"title"`
	Version    int       `json:"version"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
