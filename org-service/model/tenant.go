// Package model defines database models for the organization service.
package model

import (
	"time"

	"github.com/google/uuid"
)

// Tenant represents an organization in the system.
type Tenant struct {
	ID        uuid.UUID `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Slug      string    `json:"slug" db:"slug"`
	Domain    string    `json:"domain" db:"domain"`
	Settings  string    `json:"settings" db:"settings"` // JSON string
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// TableName returns the database table name.
func (Tenant) TableName() string {
	return "tenants"
}
