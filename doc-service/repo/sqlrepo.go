package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"sovereign-ai-compliance/doc-service/model"
	"sovereign-ai-compliance/shared/tenant"
)

// CreateDocument creates a new generated document.
func (r *SQLRepository) CreateDocument(ctx context.Context, doc *model.GeneratedDocument) error {
	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	contentJSON, err := json.Marshal(doc.Content)
	if err != nil {
		return fmt.Errorf("marshal content: %w", err)
	}

	query := `
		INSERT INTO generated_documents (id, tenant_id, ai_system_id, doc_type, title, content, version, status, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at, updated_at`

	err = tx.QueryRowContext(
		ctx, query,
		doc.ID, doc.TenantID, doc.AISystemID, doc.DocType,
		doc.Title, contentJSON, doc.Version, doc.Status, doc.CreatedBy,
	).Scan(&doc.CreatedAt, &doc.UpdatedAt)

	if err != nil {
		return fmt.Errorf("insert document: %w", err)
	}

	return tx.Commit()
}

// GetDocumentByID retrieves a document by ID.
func (r *SQLRepository) GetDocumentByID(ctx context.Context, id uuid.UUID) (*model.GeneratedDocument, error) {
	tenantID, ok := tenant.FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("tenant context required")
	}

	query := `
		SELECT id, tenant_id, ai_system_id, doc_type, title, content,
		       version, status, created_by, created_at, updated_at
		FROM generated_documents
		WHERE id = $1`

	var doc model.GeneratedDocument
	var contentJSON []byte
	err := r.base.DB().QueryRowContext(ctx, query, id).Scan(
		&doc.ID, &doc.TenantID, &doc.AISystemID, &doc.DocType,
		&doc.Title, &contentJSON, &doc.Version, &doc.Status,
		&doc.CreatedBy, &doc.CreatedAt, &doc.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get document: %w", err)
	}

	// Double-check tenant access (RLS should already handle this)
	if doc.TenantID.String() != tenantID {
		return nil, nil
	}

	if err := json.Unmarshal(contentJSON, &doc.Content); err != nil {
		return nil, fmt.Errorf("unmarshal content: %w", err)
	}

	return &doc, nil
}

// ListDocuments lists documents for the current tenant with filtering.
func (r *SQLRepository) ListDocuments(ctx context.Context, aiSystemID *uuid.UUID, docType, status *string, page, pageSize int) ([]model.DocumentSummary, int, error) {
	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	// Build query with filters
	whereClause := ""
	var args []interface{}
	argIdx := 1

	if aiSystemID != nil {
		whereClause += " WHERE ai_system_id = $" + fmt.Sprint(argIdx)
		args = append(args, *aiSystemID)
		argIdx++
	}
	if docType != nil {
		if whereClause == "" {
			whereClause += " WHERE"
		} else {
			whereClause += " AND"
		}
		whereClause += " doc_type = $" + fmt.Sprint(argIdx)
		args = append(args, *docType)
		argIdx++
	}
	if status != nil {
		if whereClause == "" {
			whereClause += " WHERE"
		} else {
			whereClause += " AND"
		}
		whereClause += " status = $" + fmt.Sprint(argIdx)
		args = append(args, *status)
		argIdx++
	}

	// Get total count
	var total int
	countQuery := "SELECT COUNT(*) FROM generated_documents" + whereClause
	err = tx.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count documents: %w", err)
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	// Get paginated results
	query := `
		SELECT id, ai_system_id, doc_type, title, version, status, created_at, updated_at
		FROM generated_documents` + whereClause + `
		ORDER BY updated_at DESC
		LIMIT $` + fmt.Sprint(argIdx) + ` OFFSET $` + fmt.Sprint(argIdx+1)

	args = append(args, pageSize, offset)
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query documents: %w", err)
	}
	defer rows.Close()

	var summaries []model.DocumentSummary
	for rows.Next() {
		var summary model.DocumentSummary
		err := rows.Scan(
			&summary.ID, &summary.AISystemID, &summary.DocType,
			&summary.Title, &summary.Version, &summary.Status,
			&summary.CreatedAt, &summary.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan document summary: %w", err)
		}
		summaries = append(summaries, summary)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows iteration: %w", err)
	}

	return summaries, total, tx.Commit()
}

// UpdateDocument updates document content and metadata.
func (r *SQLRepository) UpdateDocument(ctx context.Context, doc *model.GeneratedDocument) error {
	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	contentJSON, err := json.Marshal(doc.Content)
	if err != nil {
		return fmt.Errorf("marshal content: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE generated_documents
		 SET title = $1, content = $2, version = $3, status = $4, updated_at = NOW()
		 WHERE id = $5`,
		doc.Title, contentJSON, doc.Version, doc.Status, doc.ID,
	)
	if err != nil {
		return fmt.Errorf("update document: %w", err)
	}

	return tx.Commit()
}

// UpdateDocumentStatus updates the status of a document.
func (r *SQLRepository) UpdateDocumentStatus(ctx context.Context, id uuid.UUID, status string) error {
	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`UPDATE generated_documents SET status = $1, updated_at = NOW() WHERE id = $2`,
		status, id,
	)
	if err != nil {
		return fmt.Errorf("update document status: %w", err)
	}

	return tx.Commit()
}

// CreateVersion creates a new document version snapshot.
func (r *SQLRepository) CreateVersion(ctx context.Context, version *model.DocumentVersion) error {
	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	contentJSON, err := json.Marshal(version.Content)
	if err != nil {
		return fmt.Errorf("marshal content: %w", err)
	}

	query := `
		INSERT INTO document_versions (id, tenant_id, document_id, version_number, content, created_by, change_summary)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at`

	err = tx.QueryRowContext(
		ctx, query,
		version.ID, version.TenantID, version.DocumentID,
		version.VersionNumber, contentJSON, version.CreatedBy, version.ChangeSummary,
	).Scan(&version.CreatedAt)

	if err != nil {
		return fmt.Errorf("insert version: %w", err)
	}

	return tx.Commit()
}

// GetVersionsForDocument retrieves version history for a document.
func (r *SQLRepository) GetVersionsForDocument(ctx context.Context, documentID uuid.UUID, page, pageSize int) ([]model.DocumentVersion, int, error) {
	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	// Get total count
	var total int
	err = tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM document_versions WHERE document_id = $1", documentID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count versions: %w", err)
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	query := `
		SELECT id, tenant_id, document_id, version_number, content, created_by, created_at, change_summary
		FROM document_versions
		WHERE document_id = $1
		ORDER BY version_number DESC
		LIMIT $2 OFFSET $3`

	rows, err := tx.QueryContext(ctx, query, documentID, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query versions: %w", err)
	}
	defer rows.Close()

	var versions []model.DocumentVersion
	for rows.Next() {
		var v model.DocumentVersion
		var contentJSON []byte
		err := rows.Scan(
			&v.ID, &v.TenantID, &v.DocumentID, &v.VersionNumber,
			&contentJSON, &v.CreatedBy, &v.CreatedAt, &v.ChangeSummary,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan version: %w", err)
		}
		if err := json.Unmarshal(contentJSON, &v.Content); err != nil {
			return nil, 0, fmt.Errorf("unmarshal version content: %w", err)
		}
		versions = append(versions, v)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows iteration: %w", err)
	}

	return versions, total, tx.Commit()
}

// GetVersionByID retrieves a specific version by ID.
func (r *SQLRepository) GetVersionByID(ctx context.Context, id uuid.UUID) (*model.DocumentVersion, error) {
	tenantID, ok := tenant.FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("tenant context required")
	}

	query := `
		SELECT id, tenant_id, document_id, version_number, content, created_by, created_at, change_summary
		FROM document_versions
		WHERE id = $1`

	var v model.DocumentVersion
	var contentJSON []byte
	err := r.base.DB().QueryRowContext(ctx, query, id).Scan(
		&v.ID, &v.TenantID, &v.DocumentID, &v.VersionNumber,
		&contentJSON, &v.CreatedBy, &v.CreatedAt, &v.ChangeSummary,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get version: %w", err)
	}

	// Double-check tenant access
	if v.TenantID.String() != tenantID {
		return nil, nil
	}

	if err := json.Unmarshal(contentJSON, &v.Content); err != nil {
		return nil, fmt.Errorf("unmarshal version content: %w", err)
	}

	return &v, nil
}

// CreateExportJob creates a new export job.
func (r *SQLRepository) CreateExportJob(ctx context.Context, job *model.ExportJob) error {
	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	query := `
		INSERT INTO document_export_jobs (id, tenant_id, document_id, format, status, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at`

	err = tx.QueryRowContext(
		ctx, query,
		job.ID, job.TenantID, job.DocumentID, job.Format, job.Status, job.CreatedBy,
	).Scan(&job.CreatedAt)

	if err != nil {
		return fmt.Errorf("insert export job: %w", err)
	}

	return tx.Commit()
}

// GetExportJobByID retrieves an export job by ID.
func (r *SQLRepository) GetExportJobByID(ctx context.Context, id uuid.UUID) (*model.ExportJob, error) {
	tenantID, ok := tenant.FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("tenant context required")
	}

	query := `
		SELECT id, tenant_id, document_id, format, status, file_path, file_size,
		       error_message, created_by, created_at, completed_at
		FROM document_export_jobs
		WHERE id = $1`

	var job model.ExportJob
	err := r.base.DB().QueryRowContext(ctx, query, id).Scan(
		&job.ID, &job.TenantID, &job.DocumentID, &job.Format, &job.Status,
		&job.FilePath, &job.FileSize, &job.ErrorMessage,
		&job.CreatedBy, &job.CreatedAt, &job.CompletedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get export job: %w", err)
	}

	// Double-check tenant access
	if job.TenantID.String() != tenantID {
		return nil, nil
	}

	return &job, nil
}

// DeleteDocument deletes a document by ID.
func (r *SQLRepository) DeleteDocument(ctx context.Context, id uuid.UUID) error {
	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx,
		`DELETE FROM generated_documents WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("delete document: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("document not found")
	}

	return tx.Commit()
}

// UpdateExportJobStatus updates the status and result of an export job.
func (r *SQLRepository) UpdateExportJobStatus(ctx context.Context, id uuid.UUID, status string, filePath *string, fileSize *int64, errorMessage *string) error {
	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	var completedAt any
	if status == model.ExportStatusCompleted || status == model.ExportStatusFailed {
		completedAt = sql.NullString{String: "NOW()", Valid: true}
		_, err = tx.ExecContext(ctx,
			`UPDATE document_export_jobs
			 SET status = $1, file_path = $2, file_size = $3, error_message = $4, completed_at = NOW()
			 WHERE id = $5`,
			status, filePath, fileSize, errorMessage, id,
		)
	} else {
		_, err = tx.ExecContext(ctx,
			`UPDATE document_export_jobs
			 SET status = $1, file_path = $2, file_size = $3, error_message = $4
			 WHERE id = $5`,
			status, filePath, fileSize, errorMessage, id,
		)
	}
	_ = completedAt

	if err != nil {
		return fmt.Errorf("update export job status: %w", err)
	}

	return tx.Commit()
}
