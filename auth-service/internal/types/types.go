// Package types defines request and response types for the auth service.
package types

import (
	"github.com/google/uuid"

	"sovereign-ai-compliance/auth-service/model"
)

// LoginRequest represents a login request.
type LoginRequest struct {
	TenantID uuid.UUID `json:"tenant_id" form:"tenant_id"` // optional, may be inferred from domain
	Email    string    `json:"email" form:"email"`
	Password string    `json:"password" form:"password"`
}

// LoginResponse represents a login response.
type LoginResponse struct {
	AccessToken  string        `json:"access_token"`
	RefreshToken string        `json:"refresh_token"`
	User         model.SafeUser `json:"user"`
}

// RefreshRequest represents a token refresh request.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" form:"refresh_token"`
}

// TokenResponse represents a token pair response.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// LogoutRequest represents a logout request.
type LogoutRequest struct {
	UserID uuid.UUID `json:"user_id" form:"user_id"`
}

// PasswordResetRequest represents a password reset request.
type PasswordResetRequest struct {
	TenantID uuid.UUID `json:"tenant_id" form:"tenant_id"`
	Email    string    `json:"email" form:"email"`
}

// ResetPasswordConfirmRequest represents a password reset confirmation.
type ResetPasswordConfirmRequest struct {
	Token       string `json:"token" form:"token"`
	NewPassword string `json:"new_password" form:"new_password"`
}

// MeRequest represents a request for current user info.
type MeRequest struct {
	TenantID uuid.UUID `json:"tenant_id" form:"tenant_id"`
	UserID   uuid.UUID `json:"user_id" form:"user_id"`
}
