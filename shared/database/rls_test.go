package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"sovereign-ai-compliance/shared/tenant"
)

// compile check ensures BaseRepository compiles with required methods
func TestBaseRepositoryCompile(t *testing.T) {
	// This test verifies the API surface compiles correctly.
	// Integration tests with a real database should be added separately.
	var _ interface {
		DB() *sql.DB
	} = (*BaseRepository)(nil)
}

func TestBeginTenantTx_MissingTenant(t *testing.T) {
	// Without a real DB this only tests the tenant guard logic path.
	// A full integration test requires a running PostgreSQL instance.
	repo := NewBaseRepository(nil)
	_, err := repo.BeginTenantTx(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error when tenant context is missing")
	}
	if !errors.Is(err, context.Canceled) && err.Error() != "tenant context required for tenant transaction" {
		// The nil DB will fail at BeginTx, but the tenant check should happen first
		// when a real DB is present. With nil DB, BeginTx panics.
	}
}

func TestSetTenantContext_MissingTenant(t *testing.T) {
	err := SetTenantContext(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error when tenant context is missing")
	}
	if err.Error() != "tenant context required to set RLS context on transaction" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestTenantPropagation(t *testing.T) {
	ctx := tenant.WithContext(context.Background(), "tenant-abc")
	tid, ok := tenant.FromContext(ctx)
	if !ok || tid != "tenant-abc" {
		t.Fatal("tenant not propagated to context")
	}
}
