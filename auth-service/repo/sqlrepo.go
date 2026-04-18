// Package repo provides a concrete PostgreSQL implementation of Repository.
package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"sovereign-ai-compliance/auth-service/model"
)

// sqlRepository implements Repository using database/sql.
type sqlRepository struct {
	db *sql.DB
}

// NewSQLRepository creates a new PostgreSQL-backed repository.
func NewSQLRepository(db *sql.DB) Repository {
	return &sqlRepository{db: db}
}

// GetUserByEmail looks up a user by email within a tenant.
func (r *sqlRepository) GetUserByEmail(ctx context.Context, tenantID uuid.UUID, email string) (*model.User, error) {
	var user model.User
	var lastLogin sql.NullTime
	err := r.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, email, password_hash, role, is_active, last_login, created_at, updated_at
		 FROM users WHERE tenant_id = $1 AND email = $2`,
		tenantID, email,
	).Scan(
		&user.ID, &user.TenantID, &user.Email, &user.PasswordHash,
		&user.Role, &user.IsActive, &lastLogin, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("query user by email: %w", err)
	}
	user.LastLogin = lastLogin
	return &user, nil
}

// GetUserByID looks up a user by ID within a tenant.
func (r *sqlRepository) GetUserByID(ctx context.Context, tenantID, userID uuid.UUID) (*model.User, error) {
	var user model.User
	var lastLogin sql.NullTime
	err := r.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, email, password_hash, role, is_active, last_login, created_at, updated_at
		 FROM users WHERE tenant_id = $1 AND id = $2`,
		tenantID, userID,
	).Scan(
		&user.ID, &user.TenantID, &user.Email, &user.PasswordHash,
		&user.Role, &user.IsActive, &lastLogin, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("query user by id: %w", err)
	}
	user.LastLogin = lastLogin
	return &user, nil
}

// CreateUser inserts a new user.
func (r *sqlRepository) CreateUser(ctx context.Context, user *model.User) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO users (id, tenant_id, email, password_hash, role, is_active, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		user.ID, user.TenantID, user.Email, user.PasswordHash,
		user.Role, user.IsActive, user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}

// UpdateUser updates an existing user.
func (r *sqlRepository) UpdateUser(ctx context.Context, user *model.User) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET email = $1, password_hash = $2, role = $3, is_active = $4, updated_at = $5
		 WHERE id = $6 AND tenant_id = $7`,
		user.Email, user.PasswordHash, user.Role, user.IsActive, time.Now(),
		user.ID, user.TenantID,
	)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}

// UpdateLastLogin sets the last_login timestamp for a user.
func (r *sqlRepository) UpdateLastLogin(ctx context.Context, tenantID, userID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET last_login = $1 WHERE id = $2 AND tenant_id = $3`,
		time.Now(), userID, tenantID,
	)
	if err != nil {
		return fmt.Errorf("update last login: %w", err)
	}
	return nil
}

// StoreRefreshToken inserts a refresh token record.
func (r *sqlRepository) StoreRefreshToken(ctx context.Context, token *model.RefreshToken) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO refresh_tokens (id, user_id, tenant_id, token_hash, expires_at, revoked, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		token.ID, token.UserID, token.TenantID, token.TokenHash, token.ExpiresAt, token.Revoked, token.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert refresh token: %w", err)
	}
	return nil
}

// GetRefreshToken looks up a non-revoked refresh token by user and hash.
func (r *sqlRepository) GetRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash string) (*model.RefreshToken, error) {
	var token model.RefreshToken
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, tenant_id, token_hash, expires_at, revoked, created_at
		 FROM refresh_tokens WHERE user_id = $1 AND token_hash = $2 AND revoked = false`,
		userID, tokenHash,
	).Scan(
		&token.ID, &token.UserID, &token.TenantID, &token.TokenHash,
		&token.ExpiresAt, &token.Revoked, &token.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("refresh token not found")
		}
		return nil, fmt.Errorf("query refresh token: %w", err)
	}
	return &token, nil
}

// RevokeRefreshToken marks a refresh token as revoked.
func (r *sqlRepository) RevokeRefreshToken(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE refresh_tokens SET revoked = true WHERE id = $1`, id,
	)
	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}

// RevokeAllUserTokens revokes all refresh tokens for a user.
func (r *sqlRepository) RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE refresh_tokens SET revoked = true WHERE user_id = $1`, userID,
	)
	if err != nil {
		return fmt.Errorf("revoke all user tokens: %w", err)
	}
	return nil
}

// StorePasswordResetToken inserts a password reset token record.
func (r *sqlRepository) StorePasswordResetToken(ctx context.Context, token *model.PasswordResetToken) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO password_reset_tokens (id, user_id, tenant_id, token_hash, used, expires_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		token.ID, token.UserID, token.TenantID, token.TokenHash, token.Used, token.ExpiresAt, token.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert password reset token: %w", err)
	}
	return nil
}

// GetPasswordResetToken looks up an unused password reset token by hash.
func (r *sqlRepository) GetPasswordResetToken(ctx context.Context, tokenHash string) (*model.PasswordResetToken, error) {
	var token model.PasswordResetToken
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, tenant_id, token_hash, used, expires_at, created_at
		 FROM password_reset_tokens WHERE token_hash = $1 AND used = false`,
		tokenHash,
	).Scan(
		&token.ID, &token.UserID, &token.TenantID, &token.TokenHash,
		&token.Used, &token.ExpiresAt, &token.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("reset token not found")
		}
		return nil, fmt.Errorf("query reset token: %w", err)
	}
	return &token, nil
}

// MarkResetTokenUsed marks a password reset token as used.
func (r *sqlRepository) MarkResetTokenUsed(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE password_reset_tokens SET used = true WHERE id = $1`, id,
	)
	if err != nil {
		return fmt.Errorf("mark reset token used: %w", err)
	}
	return nil
}
