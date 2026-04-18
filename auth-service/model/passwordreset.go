package model

import (
	"time"

	"github.com/google/uuid"
)

// PasswordResetToken represents a password reset token.
type PasswordResetToken struct {
	ID        uuid.UUID `json:"id" db:"id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	TenantID  uuid.UUID `json:"tenant_id" db:"tenant_id"`
	TokenHash string    `json:"-" db:"token_hash"`
	Used      bool      `json:"used" db:"used"`
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// TableName returns the database table name.
func (PasswordResetToken) TableName() string {
	return "password_reset_tokens"
}
