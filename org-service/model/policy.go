// Package model defines database models for the organization service.
package model

import (
	"time"

	"github.com/google/uuid"
)

// CompliancePolicy represents a tenant's compliance policy configuration.
type CompliancePolicy struct {
	ID         uuid.UUID `json:"id" db:"id"`
	TenantID   uuid.UUID `json:"tenant_id" db:"tenant_id"`
	Name       string    `json:"name" db:"name"`
	PolicyType string    `json:"policy_type" db:"policy_type"`
	Rules      string    `json:"rules" db:"rules"` // JSON string
	IsActive   bool      `json:"is_active" db:"is_active"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// TableName returns the database table name.
func (CompliancePolicy) TableName() string {
	return "compliance_policies"
}
