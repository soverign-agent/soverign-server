// Package repo provides data access layer for rag-service with RLS support.
package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"
	"sovereign-ai-compliance/rag-service/model"
	"sovereign-ai-compliance/shared/database"
	"sovereign-ai-compliance/shared/tenant"
)

// SQLRepository implements the RAG repository with PostgreSQL and RLS.
type SQLRepository struct {
	base *database.BaseRepository
}

// NewSQLRepository creates a new SQLRepository.
func NewSQLRepository(db *sql.DB) *SQLRepository {
	return &SQLRepository{
		base: database.NewBaseRepository(db),
	}
}

// DB returns the underlying database connection.
func (r *SQLRepository) DB() *sql.DB {
	return r.base.DB()
}

// CreateDocument inserts a new document into the database.
func (r *SQLRepository) CreateDocument(ctx context.Context, doc *model.Document) error {
	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	query := `
		INSERT INTO documents (id, tenant_id, name, description, file_type, file_size, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at`

	err = tx.QueryRowContext(
		ctx, query,
		doc.ID, doc.TenantID, doc.Name, doc.Description,
		doc.FileType, doc.FileSize, doc.Status,
	).Scan(&doc.CreatedAt, &doc.UpdatedAt)

	if err != nil {
		return fmt.Errorf("insert document: %w", err)
	}

	return tx.Commit()
}

// GetDocumentByID retrieves a document by ID.
func (r *SQLRepository) GetDocumentByID(ctx context.Context, id uuid.UUID) (*model.Document, error) {
	tenantID, ok := tenant.FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("tenant context required")
	}

	query := `
		SELECT id, tenant_id, name, description, file_type, file_size,
		       status, error_msg, created_at, updated_at, processed_at
		FROM documents
		WHERE id = $1`

	var doc model.Document
	err := r.base.DB().QueryRowContext(ctx, query, id).Scan(
		&doc.ID, &doc.TenantID, &doc.Name, &doc.Description,
		&doc.FileType, &doc.FileSize, &doc.Status, &doc.ErrorMsg,
		&doc.CreatedAt, &doc.UpdatedAt, &doc.ProcessedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get document: %w", err)
	}

	// Verify tenant access (RLS should already handle this, but double-check)
	if doc.TenantID.String() != tenantID {
		return nil, nil
	}

	return &doc, nil
}

// ListDocuments lists documents for the current tenant with pagination.
func (r *SQLRepository) ListDocuments(ctx context.Context, page, pageSize int) ([]model.Document, int, error) {
	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	offset := (page - 1) * pageSize

	// Get total count
	var total int
	err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count documents: %w", err)
	}

	// Get paginated documents
	query := `
		SELECT id, tenant_id, name, description, file_type, file_size,
		       status, error_msg, created_at, updated_at, processed_at
		FROM documents
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`

	rows, err := tx.QueryContext(ctx, query, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query documents: %w", err)
	}
	defer rows.Close()

	var documents []model.Document
	for rows.Next() {
		var doc model.Document
		err := rows.Scan(
			&doc.ID, &doc.TenantID, &doc.Name, &doc.Description,
			&doc.FileType, &doc.FileSize, &doc.Status, &doc.ErrorMsg,
			&doc.CreatedAt, &doc.UpdatedAt, &doc.ProcessedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan document: %w", err)
		}
		documents = append(documents, doc)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows iteration: %w", err)
	}

	return documents, total, tx.Commit()
}

// DeleteDocument deletes a document and all its embeddings.
func (r *SQLRepository) DeleteDocument(ctx context.Context, id uuid.UUID) error {
	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	// Due to ON DELETE CASCADE in schema, this also deletes all embeddings
	_, err = tx.ExecContext(ctx, `DELETE FROM documents WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete document: %w", err)
	}

	return tx.Commit()
}

// UpdateDocumentStatus updates the document processing status.
func (r *SQLRepository) UpdateDocumentStatus(ctx context.Context, id uuid.UUID, status string, errorMsg *string) error {
	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	if errorMsg == nil && status == model.DocumentStatusCompleted {
		now := time.Now()
		_, err = tx.ExecContext(ctx,
			`UPDATE documents SET status = $1, processed_at = $2, updated_at = NOW() WHERE id = $3`,
			status, &now, id,
		)
	} else {
		_, err = tx.ExecContext(ctx,
			`UPDATE documents SET status = $1, error_msg = $2, updated_at = NOW() WHERE id = $3`,
			status, errorMsg, id,
		)
	}

	if err != nil {
		return fmt.Errorf("update document status: %w", err)
	}

	return tx.Commit()
}

// InsertEmbedding inserts a single embedding.
func (r *SQLRepository) InsertEmbedding(ctx context.Context, emb *model.Embedding) error {
	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	query := `
		INSERT INTO embeddings (id, tenant_id, document_id, chunk_index, text, embedding, checksum)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err = tx.ExecContext(
		ctx, query,
		emb.ID, emb.TenantID, emb.DocumentID, emb.ChunkIndex,
		emb.Text, emb.Embedding, emb.Checksum[:],
	)

	if err != nil {
		return fmt.Errorf("insert embedding: %w", err)
	}

	return tx.Commit()
}

// ExistsChecksum checks if an embedding with this checksum already exists (for deduplication).
func (r *SQLRepository) ExistsChecksum(ctx context.Context, checksum [32]byte) (bool, error) {
	tenantID, ok := tenant.FromContext(ctx)
	if !ok {
		return false, fmt.Errorf("tenant context required")
	}

	var exists bool
	err := r.base.DB().QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM embeddings WHERE tenant_id = $1 AND checksum = $2)
	`, uuid.MustParse(tenantID), checksum[:]).Scan(&exists)

	if err != nil {
		return false, fmt.Errorf("check checksum exists: %w", err)
	}

	return exists, nil
}

// DeleteEmbeddingsByDocument deletes all embeddings for a document.
func (r *SQLRepository) DeleteEmbeddingsByDocument(ctx context.Context, documentID uuid.UUID) error {
	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `DELETE FROM embeddings WHERE document_id = $1`, documentID)
	if err != nil {
		return fmt.Errorf("delete embeddings: %w", err)
	}

	return tx.Commit()
}

// SearchSimilar performs cosine similarity search for the given query embedding.
func (r *SQLRepository) SearchSimilar(ctx context.Context, queryEmb pgvector.Vector, topK int) ([]SearchResult, error) {
	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	query := `
		SELECT e.id, e.document_id, d.name, e.text, e.embedding <-> $1 AS distance
		FROM embeddings e
		JOIN documents d ON d.id = e.document_id
		WHERE d.status = 'completed'
		ORDER BY e.embedding <-> $1
		LIMIT $2`

	rows, err := tx.QueryContext(ctx, query, queryEmb, topK)
	if err != nil {
		return nil, fmt.Errorf("similarity search: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var res SearchResult
		err := rows.Scan(&res.EmbeddingID, &res.DocumentID, &res.DocumentName, &res.Text, &res.Distance)
		if err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}
		results = append(results, res)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	return results, tx.Commit()
}

// SearchResult represents a single search result from similarity search.
type SearchResult struct {
	EmbeddingID  uuid.UUID `json:"embedding_id"`
	DocumentID   uuid.UUID `json:"document_id"`
	DocumentName string    `json:"document_name"`
	Text         string    `json:"text"`
	Distance     float64   `json:"distance"` // Lower = more similar
}

// GetStats returns RAG statistics for the current tenant.
func (r *SQLRepository) GetStats(ctx context.Context) (*Stats, error) {
	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	var stats Stats
	err = tx.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FROM documents
	`).Scan(&stats.DocumentCount)
	if err != nil {
		return nil, fmt.Errorf("count documents: %w", err)
	}

	err = tx.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(file_size), 0) FROM documents
	`).Scan(&stats.TotalFileSizeBytes)
	if err != nil {
		return nil, fmt.Errorf("sum file sizes: %w", err)
	}

	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM embeddings
	`).Scan(&stats.EmbeddingCount)
	if err != nil {
		return nil, fmt.Errorf("count embeddings: %w", err)
	}

	var completedDocs int
	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM documents WHERE status = 'completed'
	`).Scan(&completedDocs)
	if err != nil {
		return nil, fmt.Errorf("count completed documents: %w", err)
	}
	stats.CompletedDocuments = completedDocs
	stats.PendingDocuments = stats.DocumentCount - completedDocs

	return &stats, tx.Commit()
}

// Stats contains RAG statistics for the current tenant.
type Stats struct {
	DocumentCount      int   `json:"document_count"`
	CompletedDocuments int   `json:"completed_documents"`
	PendingDocuments   int   `json:"pending_documents"`
	EmbeddingCount     int   `json:"embedding_count"`
	TotalFileSizeBytes int64 `json:"total_file_size_bytes"`
}
