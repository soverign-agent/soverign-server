// Package database provides Row Level Security support for zero-trust multi-tenant isolation.
package database

import (
	"context"
	"database/sql"
	"fmt"

	"sovereign-ai-compliance/shared/tenant"
)

// BaseRepository provides a foundation for tenant-scoped data access.
type BaseRepository struct {
	db *sql.DB
}

// NewBaseRepository creates a new BaseRepository wrapping the provided *sql.DB.
func NewBaseRepository(db *sql.DB) *BaseRepository {
	return &BaseRepository{db: db}
}

// DB returns the underlying *sql.DB.
func (r *BaseRepository) DB() *sql.DB {
	return r.db
}

// BeginTenantTx starts a new transaction with tenant RLS context enforced.
// It extracts the tenant ID from ctx and executes SET LOCAL app.current_tenant = ?.
// Returns an error if the tenant ID is missing from the context.
func (r *BaseRepository) BeginTenantTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	tenantID, ok := tenant.FromContext(ctx)
	if !ok || tenantID == "" {
		return nil, fmt.Errorf("tenant context required for tenant transaction")
	}

	tx, err := r.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	// SET LOCAL does not support parameterized queries; interpolate the validated UUID.
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("SET LOCAL app.current_tenant = '%s'", tenantID)); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("failed to set tenant RLS context: %w", err)
	}

	return tx, nil
}

// BeginSystemTx starts a new transaction without tenant RLS context.
// Use this only for system-level operations that must operate across tenants.
func (r *BaseRepository) BeginSystemTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	tx, err := r.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to begin system transaction: %w", err)
	}
	return tx, nil
}

// SetTenantContext applies the tenant ID to an existing transaction.
// This is useful when a transaction is created upstream and needs RLS context applied.
func SetTenantContext(ctx context.Context, tx *sql.Tx) error {
	tenantID, ok := tenant.FromContext(ctx)
	if !ok || tenantID == "" {
		return fmt.Errorf("tenant context required to set RLS context on transaction")
	}

	// SET LOCAL does not support parameterized queries; interpolate the validated UUID.
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("SET LOCAL app.current_tenant = '%s'", tenantID)); err != nil {
		return fmt.Errorf("failed to set tenant RLS context: %w", err)
	}
	return nil
}
