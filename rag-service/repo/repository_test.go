package repo

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	_ "sovereign-ai-compliance/rag-service/model"
	"sovereign-ai-compliance/shared/tenant"
)

func TestListDocuments(t *testing.T) {
	// This test requires a real database connection with pgvector
	// For unit testing, we test the RLS context propagation
	db, err := sql.Open("postgres", "host=localhost port=5432 user=postgres password=postgres dbname=sovereign sslmode=disable")
	assert.NoError(t, err)
	defer db.Close()

	repo := NewSQLRepository(db)
	tenantID := uuid.New()
	ctx := tenant.WithContext(context.Background(), tenantID.String())

	documents, total, err := repo.ListDocuments(ctx, 1, 10)
	assert.NoError(t, err)
	assert.NotNil(t, documents)
	assert.Equal(t, 0, total) // Empty initially
}
