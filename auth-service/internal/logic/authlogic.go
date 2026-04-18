// Package logic implements the auth-service business logic.
package logic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"

	"sovereign-ai-compliance/auth-service/jwt"
	"sovereign-ai-compliance/auth-service/model"
	"sovereign-ai-compliance/auth-service/password"
	"sovereign-ai-compliance/auth-service/rbac"
	"sovereign-ai-compliance/auth-service/repo"
)

// Auth handles authentication business logic.
type Auth struct {
	repo       repo.Repository
	jwtManager *jwt.Manager
}

// NewAuth creates a new Auth logic instance.
func NewAuth(r repo.Repository, jwtManager *jwt.Manager) *Auth {
	return &Auth{repo: r, jwtManager: jwtManager}
}

// Login authenticates a user and returns a token pair.
func (a *Auth) Login(ctx context.Context, tenantID uuid.UUID, email, plaintext string) (*jwt.TokenPair, *model.SafeUser, error) {
	user, err := a.repo.GetUserByEmail(ctx, tenantID, email)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid credentials")
	}

	if !user.IsActive {
		return nil, nil, fmt.Errorf("invalid credentials")
	}

	if err := password.Verify(plaintext, user.PasswordHash); err != nil {
		return nil, nil, fmt.Errorf("invalid credentials")
	}

	if err := a.repo.UpdateLastLogin(ctx, user.TenantID, user.ID); err != nil {
		// Non-fatal: log but continue
	}

	pair, err := a.jwtManager.GenerateTokenPair(
		user.ID.String(),
		user.TenantID.String(),
		user.Role,
		user.Email,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	// Store refresh token hash for revocation support
	tokenHash := hashToken(pair.RefreshToken)
	if err := a.repo.StoreRefreshToken(ctx, &model.RefreshToken{
		ID:        uuid.Must(uuid.NewRandom()),
		UserID:    user.ID,
		TenantID:  user.TenantID,
		TokenHash: tokenHash,
		ExpiresAt: pair.RefreshExpiry,
	}); err != nil {
		return nil, nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	safe := user.ToSafeUser()
	return pair, &safe, nil
}

// RefreshToken validates a refresh token and issues a new access token.
func (a *Auth) RefreshToken(ctx context.Context, refreshToken string) (*jwt.TokenPair, error) {
	claims, err := a.jwtManager.Parse(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token")
	}

	if claims.TokenType != "refresh" {
		return nil, fmt.Errorf("invalid token type")
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user id")
	}

	tokenHash := hashToken(refreshToken)
	stored, err := a.repo.GetRefreshToken(ctx, userID, tokenHash)
	if err != nil || stored == nil || stored.Revoked || stored.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("invalid or revoked refresh token")
	}

	// Revoke old token and issue new pair (token rotation)
	if err := a.repo.RevokeRefreshToken(ctx, stored.ID); err != nil {
		return nil, fmt.Errorf("failed to revoke old token")
	}

	pair, err := a.jwtManager.GenerateTokenPair(
		claims.UserID,
		claims.TenantID,
		claims.Role,
		claims.Email,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	newHash := hashToken(pair.RefreshToken)
	tenantID, _ := uuid.Parse(claims.TenantID)
	if err := a.repo.StoreRefreshToken(ctx, &model.RefreshToken{
		ID:        uuid.Must(uuid.NewRandom()),
		UserID:    userID,
		TenantID:  tenantID,
		TokenHash: newHash,
		ExpiresAt: pair.RefreshExpiry,
	}); err != nil {
		return nil, fmt.Errorf("failed to store new refresh token")
	}

	return pair, nil
}

// Logout revokes all refresh tokens for a user.
func (a *Auth) Logout(ctx context.Context, userID uuid.UUID) error {
	return a.repo.RevokeAllUserTokens(ctx, userID)
}

// ValidateToken parses and validates an access token, returning user metadata.
func (a *Auth) ValidateToken(tokenString string) (userID, tenantID, role string, err error) {
	claims, err := a.jwtManager.Parse(tokenString)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid token")
	}
	if claims.TokenType != "access" {
		return "", "", "", fmt.Errorf("invalid token type")
	}
	return claims.UserID, claims.TenantID, claims.Role, nil
}

// HasPermission delegates to the RBAC package.
func (a *Auth) HasPermission(role, resource, action string) bool {
	return rbac.HasPermission(role, resource, action)
}

// RequestPasswordReset creates a password reset token.
func (a *Auth) RequestPasswordReset(ctx context.Context, tenantID uuid.UUID, email string) error {
	user, err := a.repo.GetUserByEmail(ctx, tenantID, email)
	if err != nil {
		// Don't leak email existence
		return nil
	}

	token := uuid.Must(uuid.NewRandom()).String()
	tokenHash := hashToken(token)

	if err := a.repo.StorePasswordResetToken(ctx, &model.PasswordResetToken{
		ID:        uuid.Must(uuid.NewRandom()),
		UserID:    user.ID,
		TenantID:  user.TenantID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}); err != nil {
		return fmt.Errorf("failed to store reset token")
	}

	// In production, send email with the raw token here
	return nil
}

// ResetPassword validates a reset token and updates the user's password.
func (a *Auth) ResetPassword(ctx context.Context, token, newPassword string) error {
	tokenHash := hashToken(token)
	stored, err := a.repo.GetPasswordResetToken(ctx, tokenHash)
	if err != nil || stored == nil || stored.Used || stored.ExpiresAt.Before(time.Now()) {
		return fmt.Errorf("invalid or expired reset token")
	}

	if err := a.repo.MarkResetTokenUsed(ctx, stored.ID); err != nil {
		return fmt.Errorf("failed to mark token used")
	}

	hash, err := password.Hash(newPassword)
	if err != nil {
		return fmt.Errorf("invalid password: %w", err)
	}

	user, err := a.repo.GetUserByID(ctx, stored.TenantID, stored.UserID)
	if err != nil {
		return fmt.Errorf("user not found")
	}

	user.PasswordHash = hash
	if err := a.repo.UpdateUser(ctx, user); err != nil {
		return fmt.Errorf("failed to update password")
	}

	return nil
}

// Me returns the current user by ID.
func (a *Auth) Me(ctx context.Context, tenantID, userID uuid.UUID) (*model.SafeUser, error) {
	user, err := a.repo.GetUserByID(ctx, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}
	safe := user.ToSafeUser()
	return &safe, nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
