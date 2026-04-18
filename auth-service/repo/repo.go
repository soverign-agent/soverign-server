// Package repo defines the authentication repository interface.
package repo

import (
	"context"

	"github.com/google/uuid"

	"sovereign-ai-compliance/auth-service/model"
)

// Repository defines the data access interface for auth operations.
type Repository interface {
	// User operations
	GetUserByEmail(ctx context.Context, tenantID uuid.UUID, email string) (*model.User, error)
	GetUserByID(ctx context.Context, tenantID, userID uuid.UUID) (*model.User, error)
	CreateUser(ctx context.Context, user *model.User) error
	UpdateUser(ctx context.Context, user *model.User) error
	UpdateLastLogin(ctx context.Context, tenantID, userID uuid.UUID) error

	// Refresh token operations
	StoreRefreshToken(ctx context.Context, token *model.RefreshToken) error
	GetRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash string) (*model.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, id uuid.UUID) error
	RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error

	// Password reset operations
	StorePasswordResetToken(ctx context.Context, token *model.PasswordResetToken) error
	GetPasswordResetToken(ctx context.Context, tokenHash string) (*model.PasswordResetToken, error)
	MarkResetTokenUsed(ctx context.Context, id uuid.UUID) error
}
