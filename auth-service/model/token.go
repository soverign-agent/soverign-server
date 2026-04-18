package model

import (
	"time"

	"github.com/google/uuid"
)

// RefreshToken represents a stored refresh token for revocation support.
type RefreshToken struct {
	ID        uuid.UUID `json:"id" db:"id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	TenantID  uuid.UUID `json:"tenant_id" db:"tenant_id"`
	TokenHash string    `json:"-" db:"token_hash"`
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
	Revoked   bool      `json:"revoked" db:"revoked"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// TableName returns the database table name.
func (RefreshToken) TableName() string {
	return "refresh_tokens"
}
