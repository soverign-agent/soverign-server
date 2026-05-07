// Package types defines request and response types for doc-service API endpoints.
package types

import (
	"github.com/google/uuid"
	"sovereign-ai-compliance/doc-service/model"
)

// GenerateDocumentRequest is the request to generate a new document.
type GenerateDocumentRequest struct {
	AISystemID uuid.UUID         `json:"ai_system_id,optional" form:"ai_system_id,optional"`
	DocType    string            `json:"doc_type,optional" form:"doc_type,optional"`
	AuditJobID *uuid.UUID        `json:"audit_job_id,optional" form:"audit_job_id,optional"`
	Title      string            `json:"title,optional" form:"title,optional"`
	Options    map[string]string `json:"options,optional" form:"options,optional"`
}

// GenerateDocumentResponse is the response after triggering document generation.
type GenerateDocumentResponse struct {
	DocumentID uuid.UUID `json:"document_id"`
	Status     string    `json:"status"`
}

// ListDocumentsRequest is the request to list documents with filtering.
type ListDocumentsRequest struct {
	AISystemID *uuid.UUID `json:"ai_system_id,optional" form:"ai_system_id,optional"`
	DocType    *string    `json:"doc_type,optional" form:"doc_type,optional"`
	Status     *string    `json:"status,optional" form:"status,optional"`
	Page       int        `json:"page,optional" form:"page,optional"`
	PageSize   int        `json:"page_size,optional" form:"page_size,optional"`
}

// ListDocumentsResponse is the response containing a list of documents.
type ListDocumentsResponse struct {
	Items []model.DocumentSummary `json:"items"`
	Total int                     `json:"total"`
	Page  int                     `json:"page"`
	Pages int                     `json:"pages"`
}

// GetDocumentRequest is the request to get a specific document.
type GetDocumentRequest struct {
	DocumentID uuid.UUID `json:"-" form:"-"`
}

// GetDocumentResponse is the response containing the document details.
type GetDocumentResponse struct {
	model.GeneratedDocument
}

// UpdateDocumentRequest is the request to update document content (human editing).
type UpdateDocumentRequest struct {
	DocumentID uuid.UUID             `json:"-" form:"-"`
	Content    model.DocumentContent `json:"content,optional" form:"content,optional"`
	UpdatedBy  uuid.UUID             `json:"updated_by,optional" form:"updated_by,optional"`
}

// UpdateDocumentResponse is the response after updating a document.
type UpdateDocumentResponse struct {
	model.GeneratedDocument
}

// UpdateDocumentStatusRequest is the request to update document status.
type UpdateDocumentStatusRequest struct {
	DocumentID uuid.UUID `json:"-" form:"-"`
	Status     string    `json:"status,optional" form:"status,optional"`
}

// UpdateDocumentStatusResponse is the response after updating document status.
type UpdateDocumentStatusResponse struct {
	Success bool   `json:"success"`
	Status  string `json:"status"`
}

// ListVersionsRequest is the request to list document versions.
type ListVersionsRequest struct {
	DocumentID uuid.UUID `json:"-" form:"-"`
	Page       int       `json:"page,optional" form:"page,optional"`
	PageSize   int       `json:"page_size,optional" form:"page_size,optional"`
}

// ListVersionsResponse is the response containing version history.
type ListVersionsResponse struct {
	Items []model.DocumentVersion `json:"items"`
	Total int                     `json:"total"`
	Page  int                     `json:"page"`
	Pages int                     `json:"pages"`
}

// RollbackVersionRequest is the request to rollback to a specific version.
type RollbackVersionRequest struct {
	DocumentID    uuid.UUID `json:"-" form:"-"`
	VersionNumber int       `json:"version_number,optional" form:"version_number,optional"`
}

// RollbackVersionResponse is the response after rolling back.
type RollbackVersionResponse struct {
	model.GeneratedDocument
}

// CreateExportJobRequest is the request to create an export job.
type CreateExportJobRequest struct {
	DocumentID uuid.UUID `json:"-" form:"-"`
	Format     string    `json:"format,optional" form:"format,optional"`
}

// CreateExportJobResponse is the response after creating an export job.
type CreateExportJobResponse struct {
	JobID  uuid.UUID `json:"job_id"`
	Status string    `json:"status"`
}

// GetExportJobRequest is the request to get an export job status.
type GetExportJobRequest struct {
	JobID uuid.UUID `json:"-" form:"-"`
}

// GetExportJobResponse is the response containing export job details.
type GetExportJobResponse struct {
	model.ExportJob
}

// DeleteDocumentRequest is the request to delete a document.
type DeleteDocumentRequest struct {
	DocumentID uuid.UUID `json:"-" form:"-"`
}

// DeleteDocumentResponse is the response after deleting a document.
type DeleteDocumentResponse struct {
	Success bool `json:"success"`
}
