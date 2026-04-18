// Package model defines database models for the organization service.
package model

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// User represents a user account in the system.
type User struct {
	ID           uuid.UUID    `json:"id" db:"id"`
	TenantID     uuid.UUID    `json:"tenant_id" db:"tenant_id"`
	Email        string       `json:"email" db:"email"`
	PasswordHash string       `json:"-" db:"password_hash"`
	Role         string       `json:"role" db:"role"`
	IsActive     bool         `json:"is_active" db:"is_active"`
	LastLogin    sql.NullTime `json:"last_login" db:"last_login"`
	CreatedAt    time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at" db:"updated_at"`
}

// TableName returns the database table name.
func (User) TableName() string {
	return "users"
}

// SafeUser is a user model without sensitive fields.
type SafeUser struct {
	ID        uuid.UUID    `json:"id"`
	TenantID  uuid.UUID    `json:"tenant_id"`
	Email     string       `json:"email"`
	Role      string       `json:"role"`
	IsActive  bool         `json:"is_active"`
	LastLogin sql.NullTime `json:"last_login"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// ToSafeUser returns a copy with sensitive fields removed.
func (u *User) ToSafeUser() SafeUser {
	return SafeUser{
		ID:        u.ID,
		TenantID:  u.TenantID,
		Email:     u.Email,
		Role:      u.Role,
		IsActive:  u.IsActive,
		LastLogin: u.LastLogin,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}
