// Package model defines the data models for rag-service.
package model

import (
	"time"

	"github.com/google/uuid"
)

// Document represents an uploaded document with metadata.
type Document struct {
	ID          uuid.UUID    `json:"id"`
	TenantID    uuid.UUID    `json:"tenant_id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	FileType    string       `json:"file_type"` // pdf, docx, md, txt
	FileSize    int64        `json:"file_size"`
	Status      string       `json:"status"` // pending, processing, completed, failed
	ErrorMsg    *string      `json:"error_msg,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	ProcessedAt *time.Time   `json:"processed_at,omitempty"`
}

// DocumentStatus enumerates possible document processing statuses.
const (
	DocumentStatusPending    = "pending"
	DocumentStatusProcessing = "processing"
	DocumentStatusCompleted  = "completed"
	DocumentStatusFailed     = "failed"
)
