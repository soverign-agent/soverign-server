package model

import (
	"time"

	"github.com/google/uuid"
)

// Approval status constants
const (
	ApprovalStatusPending   = "pending"
	ApprovalStatusApproved  = "approved"
	ApprovalStatusRejected  = "rejected"
)

// ApprovalRequest represents a human-in-the-loop approval item.
type ApprovalRequest struct {
	ID          uuid.UUID `json:"id"`
	TenantID    uuid.UUID `json:"tenant_id"`
	AuditJobID  uuid.UUID `json:"audit_job_id"`
	RequestedBy string    `json:"requested_by"`
	AssignedTo  []string  `json:"assigned_to"`
	Title       string    `json:"title"`
	ContextJSON string    `json:"context_json"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	DecidedAt   *time.Time `json:"decided_at,omitempty"`
	DecidedBy   *string    `json:"decided_by,omitempty"`
}
