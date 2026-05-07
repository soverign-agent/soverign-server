package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sovereign-ai-compliance/doc-service/model"
	"sovereign-ai-compliance/shared/tenant"
)

func setupMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, func()) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	return db, mock, func() { db.Close() }
}

func expectTenantTx(mock sqlmock.Sqlmock, tenantID string) {
	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL app.current_tenant = '" + tenantID + "'").
		WillReturnResult(sqlmock.NewResult(0, 1))
}

func TestSQLRepository_CreateDocument(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	tenantID := uuid.New()
	ctx := tenant.WithContext(context.Background(), tenantID.String())

	doc := &model.GeneratedDocument{
		ID:         uuid.New(),
		TenantID:   tenantID,
		AISystemID: uuid.New(),
		DocType:    model.DocTypeAnnexIV,
		Title:      "Test Document",
		Content: model.DocumentContent{
			Sections: []model.Section{{ID: "s1", Title: "Section 1", Content: "content"}},
			Metadata: map[string]string{"key": "value"},
		},
		Version:   1,
		Status:    model.StatusGenerating,
		CreatedBy: tenantID,
	}

	contentJSON, _ := json.Marshal(doc.Content)

	expectTenantTx(mock, tenantID.String())
	mock.ExpectQuery(`INSERT INTO generated_documents`).
		WithArgs(doc.ID, doc.TenantID, doc.AISystemID, doc.DocType, doc.Title, contentJSON, doc.Version, doc.Status, doc.CreatedBy).
		WillReturnRows(sqlmock.NewRows([]string{"created_at", "updated_at"}).AddRow(time.Now(), time.Now()))
	mock.ExpectCommit()

	err := repo.CreateDocument(ctx, doc)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepository_CreateDocument_MarshalError(t *testing.T) {
	db, _, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	doc := &model.GeneratedDocument{
		ID:       uuid.New(),
		TenantID: uuid.New(),
		Content: model.DocumentContent{
			Metadata: map[string]string{"bad": string([]byte{0xff})},
		},
	}

	// DocumentContent only contains basic types, json.Marshal rarely fails.
	// We test the error wrapping path by calling without a mocked DB transaction.
	err := repo.CreateDocument(ctx, doc)
	assert.Error(t, err)
}

func TestSQLRepository_GetDocumentByID(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	tenantID := uuid.New()
	ctx := tenant.WithContext(context.Background(), tenantID.String())

	docID := uuid.New()
	content := model.DocumentContent{
		Sections: []model.Section{{ID: "s1", Title: "Title", Content: "Body"}},
	}
	contentJSON, _ := json.Marshal(content)
	now := time.Now()

	mock.ExpectQuery(`SELECT id, tenant_id, ai_system_id, doc_type, title, content, version, status, created_by, created_at, updated_at FROM generated_documents WHERE id = \$1`).
		WithArgs(docID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "tenant_id", "ai_system_id", "doc_type", "title", "content",
			"version", "status", "created_by", "created_at", "updated_at",
		}).AddRow(docID, tenantID, uuid.New(), model.DocTypeAnnexIV, "Test", contentJSON, 1, model.StatusEditing, tenantID, now, now))

	doc, err := repo.GetDocumentByID(ctx, docID)
	require.NoError(t, err)
	require.NotNil(t, doc)
	assert.Equal(t, docID, doc.ID)
	assert.Equal(t, "Test", doc.Title)
	assert.Len(t, doc.Content.Sections, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepository_GetDocumentByID_NotFound(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	docID := uuid.New()
	mock.ExpectQuery(`SELECT id, tenant_id, ai_system_id, doc_type, title, content, version, status, created_by, created_at, updated_at FROM generated_documents WHERE id = \$1`).
		WithArgs(docID).
		WillReturnError(sql.ErrNoRows)

	doc, err := repo.GetDocumentByID(ctx, docID)
	require.NoError(t, err)
	assert.Nil(t, doc)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepository_GetDocumentByID_WrongTenant(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	tenantID := uuid.New()
	otherTenant := uuid.New()
	ctx := tenant.WithContext(context.Background(), tenantID.String())

	docID := uuid.New()
	contentJSON := []byte(`{"sections":[]}`)
	now := time.Now()

	mock.ExpectQuery(`SELECT id, tenant_id, ai_system_id, doc_type, title, content, version, status, created_by, created_at, updated_at FROM generated_documents WHERE id = \$1`).
		WithArgs(docID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "tenant_id", "ai_system_id", "doc_type", "title", "content",
			"version", "status", "created_by", "created_at", "updated_at",
		}).AddRow(docID, otherTenant, uuid.New(), model.DocTypeAnnexIV, "Test", contentJSON, 1, model.StatusEditing, otherTenant, now, now))

	doc, err := repo.GetDocumentByID(ctx, docID)
	require.NoError(t, err)
	assert.Nil(t, doc)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepository_GetDocumentByID_MissingTenant(t *testing.T) {
	db, _, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	ctx := context.Background()

	_, err := repo.GetDocumentByID(ctx, uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tenant context required")
}

func TestSQLRepository_ListDocuments(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	tenantID := uuid.New()
	ctx := tenant.WithContext(context.Background(), tenantID.String())

	aiSystemID := uuid.New()
	docType := model.DocTypeAnnexIV
	status := model.StatusEditing
	now := time.Now()

	expectTenantTx(mock, tenantID.String())
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM generated_documents WHERE ai_system_id = \$1 AND doc_type = \$2 AND status = \$3`).
		WithArgs(aiSystemID, docType, status).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	mock.ExpectQuery(`SELECT id, ai_system_id, doc_type, title, version, status, created_at, updated_at FROM generated_documents WHERE ai_system_id = \$1 AND doc_type = \$2 AND status = \$3 ORDER BY updated_at DESC LIMIT \$4 OFFSET \$5`).
		WithArgs(aiSystemID, docType, status, 10, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "ai_system_id", "doc_type", "title", "version", "status", "created_at", "updated_at",
		}).
			AddRow(uuid.New(), aiSystemID, docType, "Doc 1", 1, status, now, now).
			AddRow(uuid.New(), aiSystemID, docType, "Doc 2", 2, status, now, now))
	mock.ExpectCommit()

	summaries, total, err := repo.ListDocuments(ctx, &aiSystemID, &docType, &status, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, summaries, 2)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepository_ListDocuments_NoFilters(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())
	now := time.Now()

	expectTenantTx(mock, tenant.MustFromContext(ctx))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM generated_documents`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT id, ai_system_id, doc_type, title, version, status, created_at, updated_at FROM generated_documents ORDER BY updated_at DESC LIMIT \$1 OFFSET \$2`).
		WithArgs(20, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "ai_system_id", "doc_type", "title", "version", "status", "created_at", "updated_at",
		}).AddRow(uuid.New(), uuid.New(), model.DocTypeAnnexIV, "Doc", 1, model.StatusEditing, now, now))
	mock.ExpectCommit()

	summaries, total, err := repo.ListDocuments(ctx, nil, nil, nil, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, summaries, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepository_UpdateDocument(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	doc := &model.GeneratedDocument{
		ID:    uuid.New(),
		Title: "Updated Title",
		Content: model.DocumentContent{
			Sections: []model.Section{{ID: "s1", Title: "Section", Content: "Updated content"}},
		},
		Version: 2,
		Status:  model.StatusEditing,
	}
	contentJSON, _ := json.Marshal(doc.Content)

	expectTenantTx(mock, tenant.MustFromContext(ctx))
	mock.ExpectExec(`UPDATE generated_documents SET title = \$1, content = \$2, version = \$3, status = \$4, updated_at = NOW\(\) WHERE id = \$5`).
		WithArgs(doc.Title, contentJSON, doc.Version, doc.Status, doc.ID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := repo.UpdateDocument(ctx, doc)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepository_UpdateDocumentStatus(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	docID := uuid.New()
	expectTenantTx(mock, tenant.MustFromContext(ctx))
	mock.ExpectExec(`UPDATE generated_documents SET status = \$1, updated_at = NOW\(\) WHERE id = \$2`).
		WithArgs(model.StatusApproved, docID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := repo.UpdateDocumentStatus(ctx, docID, model.StatusApproved)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepository_CreateVersion(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	version := &model.DocumentVersion{
		ID:            uuid.New(),
		TenantID:      uuid.New(),
		DocumentID:    uuid.New(),
		VersionNumber: 1,
		Content: model.DocumentContent{
			Sections: []model.Section{{ID: "s1", Title: "Section", Content: "Body"}},
		},
		CreatedBy:     uuid.New(),
		ChangeSummary: "Initial version",
	}
	contentJSON, _ := json.Marshal(version.Content)

	expectTenantTx(mock, tenant.MustFromContext(ctx))
	mock.ExpectQuery(`INSERT INTO document_versions`).
		WithArgs(version.ID, version.TenantID, version.DocumentID, version.VersionNumber, contentJSON, version.CreatedBy, version.ChangeSummary).
		WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(time.Now()))
	mock.ExpectCommit()

	err := repo.CreateVersion(ctx, version)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepository_GetVersionsForDocument(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	docID := uuid.New()
	contentJSON := []byte(`{"sections":[]}`)
	now := time.Now()

	expectTenantTx(mock, tenant.MustFromContext(ctx))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM document_versions WHERE document_id = \$1`).
		WithArgs(docID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT id, tenant_id, document_id, version_number, content, created_by, created_at, change_summary FROM document_versions WHERE document_id = \$1 ORDER BY version_number DESC LIMIT \$2 OFFSET \$3`).
		WithArgs(docID, 10, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "tenant_id", "document_id", "version_number", "content", "created_by", "created_at", "change_summary",
		}).AddRow(uuid.New(), uuid.New(), docID, 1, contentJSON, uuid.New(), now, "v1"))
	mock.ExpectCommit()

	versions, total, err := repo.GetVersionsForDocument(ctx, docID, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, versions, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepository_GetVersionByID(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	tenantID := uuid.New()
	ctx := tenant.WithContext(context.Background(), tenantID.String())

	versionID := uuid.New()
	contentJSON := []byte(`{"sections":[{"id":"s1","title":"T","content":"C"}]}`)
	now := time.Now()

	mock.ExpectQuery(`SELECT id, tenant_id, document_id, version_number, content, created_by, created_at, change_summary FROM document_versions WHERE id = \$1`).
		WithArgs(versionID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "tenant_id", "document_id", "version_number", "content", "created_by", "created_at", "change_summary",
		}).AddRow(versionID, tenantID, uuid.New(), 1, contentJSON, uuid.New(), now, "v1"))

	version, err := repo.GetVersionByID(ctx, versionID)
	require.NoError(t, err)
	require.NotNil(t, version)
	assert.Equal(t, versionID, version.ID)
	assert.Len(t, version.Content.Sections, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepository_GetVersionByID_MissingTenant(t *testing.T) {
	db, _, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	ctx := context.Background()

	_, err := repo.GetVersionByID(ctx, uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tenant context required")
}

func TestSQLRepository_CreateExportJob(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	job := &model.ExportJob{
		ID:         uuid.New(),
		TenantID:   uuid.New(),
		DocumentID: uuid.New(),
		Format:     model.ExportFormatPDF,
		Status:     model.ExportStatusPending,
		CreatedBy:  uuid.New(),
	}

	expectTenantTx(mock, tenant.MustFromContext(ctx))
	mock.ExpectQuery(`INSERT INTO document_export_jobs`).
		WithArgs(job.ID, job.TenantID, job.DocumentID, job.Format, job.Status, job.CreatedBy).
		WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(time.Now()))
	mock.ExpectCommit()

	err := repo.CreateExportJob(ctx, job)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepository_GetExportJobByID(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	tenantID := uuid.New()
	ctx := tenant.WithContext(context.Background(), tenantID.String())

	jobID := uuid.New()
	filePath := "/exports/test.pdf"
	fileSize := int64(1024)
	now := time.Now()

	mock.ExpectQuery(`SELECT id, tenant_id, document_id, format, status, file_path, file_size, error_message, created_by, created_at, completed_at FROM document_export_jobs WHERE id = \$1`).
		WithArgs(jobID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "tenant_id", "document_id", "format", "status", "file_path", "file_size",
			"error_message", "created_by", "created_at", "completed_at",
		}).AddRow(jobID, tenantID, uuid.New(), model.ExportFormatPDF, model.ExportStatusCompleted, &filePath, &fileSize, nil, uuid.New(), now, &now))

	job, err := repo.GetExportJobByID(ctx, jobID)
	require.NoError(t, err)
	require.NotNil(t, job)
	assert.Equal(t, jobID, job.ID)
	assert.Equal(t, model.ExportStatusCompleted, job.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepository_GetExportJobByID_MissingTenant(t *testing.T) {
	db, _, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	ctx := context.Background()

	_, err := repo.GetExportJobByID(ctx, uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tenant context required")
}

func TestSQLRepository_UpdateExportJobStatus(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	jobID := uuid.New()
	filePath := "/exports/test.pdf"
	fileSize := int64(2048)

	expectTenantTx(mock, tenant.MustFromContext(ctx))
	mock.ExpectExec(`UPDATE document_export_jobs SET status = \$1, file_path = \$2, file_size = \$3, error_message = \$4, completed_at = NOW\(\) WHERE id = \$5`).
		WithArgs(model.ExportStatusCompleted, &filePath, &fileSize, nil, jobID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := repo.UpdateExportJobStatus(ctx, jobID, model.ExportStatusCompleted, &filePath, &fileSize, nil)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepository_UpdateExportJobStatus_Processing(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	jobID := uuid.New()

	expectTenantTx(mock, tenant.MustFromContext(ctx))
	mock.ExpectExec(`UPDATE document_export_jobs SET status = \$1, file_path = \$2, file_size = \$3, error_message = \$4 WHERE id = \$5`).
		WithArgs(model.ExportStatusProcessing, nil, nil, nil, jobID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := repo.UpdateExportJobStatus(ctx, jobID, model.ExportStatusProcessing, nil, nil, nil)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepository_DeleteDocument(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	docID := uuid.New()
	expectTenantTx(mock, tenant.MustFromContext(ctx))
	mock.ExpectExec(`DELETE FROM generated_documents WHERE id = \$1`).
		WithArgs(docID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := repo.DeleteDocument(ctx, docID)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepository_DeleteDocument_BeginTxError(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	mock.ExpectBegin().WillReturnError(errors.New("connection refused"))

	err := repo.DeleteDocument(ctx, uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "begin tenant tx")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepository_DeleteDocument_ExecError(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	docID := uuid.New()
	expectTenantTx(mock, tenant.MustFromContext(ctx))
	mock.ExpectExec(`DELETE FROM generated_documents WHERE id = \$1`).
		WithArgs(docID).
		WillReturnError(errors.New("foreign key violation"))
	mock.ExpectRollback()

	err := repo.DeleteDocument(ctx, docID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "delete document")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepository_DeleteDocument_NotFound(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	docID := uuid.New()
	expectTenantTx(mock, tenant.MustFromContext(ctx))
	mock.ExpectExec(`DELETE FROM generated_documents WHERE id = \$1`).
		WithArgs(docID).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	err := repo.DeleteDocument(ctx, docID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "document not found")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepository_DeleteDocument_MissingTenant(t *testing.T) {
	db, _, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	ctx := context.Background()

	err := repo.DeleteDocument(ctx, uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tenant context required")
}

func TestSQLRepository_DB(t *testing.T) {
	db, _, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	assert.Equal(t, db, repo.DB())
}

func TestSQLRepository_BeginTenantTx_MissingTenant(t *testing.T) {
	db, _, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	ctx := context.Background()

	doc := &model.GeneratedDocument{ID: uuid.New()}
	err := repo.CreateDocument(ctx, doc)
	assert.Error(t, err)
}

func TestSQLRepository_ListDocuments_DBError(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	expectTenantTx(mock, tenant.MustFromContext(ctx))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM generated_documents`).
		WillReturnError(errors.New("db connection lost"))
	mock.ExpectRollback()

	_, _, err := repo.ListDocuments(ctx, nil, nil, nil, 1, 20)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "count documents")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepository_CreateDocument_BeginTxError(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	mock.ExpectBegin().WillReturnError(errors.New("connection refused"))

	err := repo.CreateDocument(ctx, &model.GeneratedDocument{ID: uuid.New()})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "begin tenant tx")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepository_CreateDocument_InsertError(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	expectTenantTx(mock, tenant.MustFromContext(ctx))
	mock.ExpectQuery(`INSERT INTO generated_documents`).
		WillReturnError(errors.New("unique violation"))
	mock.ExpectRollback()

	err := repo.CreateDocument(ctx, &model.GeneratedDocument{ID: uuid.New()})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insert document")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepository_GetDocumentByID_UnmarshalError(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	tenantID := uuid.New()
	ctx := tenant.WithContext(context.Background(), tenantID.String())

	docID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(`SELECT id, tenant_id, ai_system_id, doc_type, title, content, version, status, created_by, created_at, updated_at FROM generated_documents WHERE id = \$1`).
		WithArgs(docID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "tenant_id", "ai_system_id", "doc_type", "title", "content",
			"version", "status", "created_by", "created_at", "updated_at",
		}).AddRow(docID, tenantID, uuid.New(), model.DocTypeAnnexIV, "Test", []byte(`invalid json`), 1, model.StatusEditing, tenantID, now, now))

	_, err := repo.GetDocumentByID(ctx, docID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal content")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepository_UpdateDocument_BeginTxError(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	mock.ExpectBegin().WillReturnError(errors.New("connection refused"))

	err := repo.UpdateDocument(ctx, &model.GeneratedDocument{ID: uuid.New()})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "begin tenant tx")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepository_UpdateDocument_ExecError(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	expectTenantTx(mock, tenant.MustFromContext(ctx))
	mock.ExpectExec(`UPDATE generated_documents`).
		WillReturnError(errors.New("lock timeout"))
	mock.ExpectRollback()

	err := repo.UpdateDocument(ctx, &model.GeneratedDocument{ID: uuid.New()})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "update document")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepository_UpdateDocumentStatus_BeginTxError(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	mock.ExpectBegin().WillReturnError(errors.New("connection refused"))

	err := repo.UpdateDocumentStatus(ctx, uuid.New(), model.StatusApproved)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "begin tenant tx")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepository_CreateVersion_InsertError(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	expectTenantTx(mock, tenant.MustFromContext(ctx))
	mock.ExpectQuery(`INSERT INTO document_versions`).
		WillReturnError(errors.New("fk violation"))
	mock.ExpectRollback()

	err := repo.CreateVersion(ctx, &model.DocumentVersion{ID: uuid.New()})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insert version")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepository_GetVersionsForDocument_UnmarshalError(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	docID := uuid.New()
	now := time.Now()

	expectTenantTx(mock, tenant.MustFromContext(ctx))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM document_versions WHERE document_id = \$1`).
		WithArgs(docID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT id, tenant_id, document_id, version_number, content, created_by, created_at, change_summary FROM document_versions WHERE document_id = \$1 ORDER BY version_number DESC LIMIT \$2 OFFSET \$3`).
		WithArgs(docID, 10, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "tenant_id", "document_id", "version_number", "content", "created_by", "created_at", "change_summary",
		}).AddRow(uuid.New(), uuid.New(), docID, 1, []byte(`invalid json`), uuid.New(), now, "v1"))
	mock.ExpectRollback()

	_, _, err := repo.GetVersionsForDocument(ctx, docID, 1, 10)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal version content")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepository_GetVersionByID_UnmarshalError(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	tenantID := uuid.New()
	ctx := tenant.WithContext(context.Background(), tenantID.String())

	versionID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(`SELECT id, tenant_id, document_id, version_number, content, created_by, created_at, change_summary FROM document_versions WHERE id = \$1`).
		WithArgs(versionID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "tenant_id", "document_id", "version_number", "content", "created_by", "created_at", "change_summary",
		}).AddRow(versionID, tenantID, uuid.New(), 1, []byte(`invalid json`), uuid.New(), now, "v1"))

	_, err := repo.GetVersionByID(ctx, versionID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal version content")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepository_CreateExportJob_BeginTxError(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	mock.ExpectBegin().WillReturnError(errors.New("connection refused"))

	err := repo.CreateExportJob(ctx, &model.ExportJob{ID: uuid.New()})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "begin tenant tx")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepository_UpdateExportJobStatus_BeginTxError(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	mock.ExpectBegin().WillReturnError(errors.New("connection refused"))

	err := repo.UpdateExportJobStatus(ctx, uuid.New(), model.ExportStatusFailed, nil, nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "begin tenant tx")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepository_GetExportJobByID_NotFound(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	jobID := uuid.New()
	mock.ExpectQuery(`SELECT id, tenant_id, document_id, format, status, file_path, file_size, error_message, created_by, created_at, completed_at FROM document_export_jobs WHERE id = \$1`).
		WithArgs(jobID).
		WillReturnError(sql.ErrNoRows)

	job, err := repo.GetExportJobByID(ctx, jobID)
	require.NoError(t, err)
	assert.Nil(t, job)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepository_GetExportJobByID_WrongTenant(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	repo := NewSQLRepository(db)
	tenantID := uuid.New()
	otherTenant := uuid.New()
	ctx := tenant.WithContext(context.Background(), tenantID.String())

	jobID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(`SELECT id, tenant_id, document_id, format, status, file_path, file_size, error_message, created_by, created_at, completed_at FROM document_export_jobs WHERE id = \$1`).
		WithArgs(jobID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "tenant_id", "document_id", "format", "status", "file_path", "file_size",
			"error_message", "created_by", "created_at", "completed_at",
		}).AddRow(jobID, otherTenant, uuid.New(), model.ExportFormatPDF, model.ExportStatusCompleted, nil, nil, nil, uuid.New(), now, nil))

	job, err := repo.GetExportJobByID(ctx, jobID)
	require.NoError(t, err)
	assert.Nil(t, job)
	assert.NoError(t, mock.ExpectationsWereMet())
}
