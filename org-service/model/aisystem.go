// Package model defines database models for the organization service.
package model

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// AISystem represents an AI system managed by a tenant.
type AISystem struct {
	ID                uuid.UUID    `json:"id" db:"id"`
	TenantID          uuid.UUID    `json:"tenant_id" db:"tenant_id"`
	Name              string       `json:"name" db:"name"`
	Description       string       `json:"description" db:"description"`
	RiskClassification string      `json:"risk_classification" db:"risk_classification"`
	Status            string       `json:"status" db:"status"`
	Metadata          string       `json:"metadata" db:"metadata"` // JSON string
	CreatedAt         time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at" db:"updated_at"`
	DeletedAt         sql.NullTime `json:"deleted_at" db:"deleted_at"`
}

// TableName returns the database table name.
func (AISystem) TableName() string {
	return "ai_systems"
}

// IsDeleted returns true if the AI system has been soft deleted.
func (s *AISystem) IsDeleted() bool {
	return s.DeletedAt.Valid
}
