// Package repo provides a mock implementation for testing.
package repo

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"sovereign-ai-compliance/doc-service/model"
)

// MockRepository is a test double for Repository.
type MockRepository struct {
	CreateDocumentFunc         func(ctx context.Context, doc *model.GeneratedDocument) error
	GetDocumentByIDFunc        func(ctx context.Context, id uuid.UUID) (*model.GeneratedDocument, error)
	ListDocumentsFunc          func(ctx context.Context, aiSystemID *uuid.UUID, docType, status *string, page, pageSize int) ([]model.DocumentSummary, int, error)
	UpdateDocumentFunc         func(ctx context.Context, doc *model.GeneratedDocument) error
	UpdateDocumentStatusFunc   func(ctx context.Context, id uuid.UUID, status string) error
	CreateVersionFunc          func(ctx context.Context, version *model.DocumentVersion) error
	GetVersionsForDocumentFunc func(ctx context.Context, documentID uuid.UUID, page, pageSize int) ([]model.DocumentVersion, int, error)
	GetVersionByIDFunc         func(ctx context.Context, id uuid.UUID) (*model.DocumentVersion, error)
	CreateExportJobFunc        func(ctx context.Context, job *model.ExportJob) error
	GetExportJobByIDFunc       func(ctx context.Context, id uuid.UUID) (*model.ExportJob, error)
	UpdateExportJobStatusFunc  func(ctx context.Context, id uuid.UUID, status string, filePath *string, fileSize *int64, errorMessage *string) error
	DeleteDocumentFunc         func(ctx context.Context, id uuid.UUID) error
}

// DB is not implemented for the mock.
func (m *MockRepository) DB() *sql.DB { return nil }

// CreateDocument delegates to CreateDocumentFunc.
func (m *MockRepository) CreateDocument(ctx context.Context, doc *model.GeneratedDocument) error {
	if m.CreateDocumentFunc != nil {
		return m.CreateDocumentFunc(ctx, doc)
	}
	return nil
}

// GetDocumentByID delegates to GetDocumentByIDFunc.
func (m *MockRepository) GetDocumentByID(ctx context.Context, id uuid.UUID) (*model.GeneratedDocument, error) {
	if m.GetDocumentByIDFunc != nil {
		return m.GetDocumentByIDFunc(ctx, id)
	}
	return nil, nil
}

// ListDocuments delegates to ListDocumentsFunc.
func (m *MockRepository) ListDocuments(ctx context.Context, aiSystemID *uuid.UUID, docType, status *string, page, pageSize int) ([]model.DocumentSummary, int, error) {
	if m.ListDocumentsFunc != nil {
		return m.ListDocumentsFunc(ctx, aiSystemID, docType, status, page, pageSize)
	}
	return nil, 0, nil
}

// UpdateDocument delegates to UpdateDocumentFunc.
func (m *MockRepository) UpdateDocument(ctx context.Context, doc *model.GeneratedDocument) error {
	if m.UpdateDocumentFunc != nil {
		return m.UpdateDocumentFunc(ctx, doc)
	}
	return nil
}

// UpdateDocumentStatus delegates to UpdateDocumentStatusFunc.
func (m *MockRepository) UpdateDocumentStatus(ctx context.Context, id uuid.UUID, status string) error {
	if m.UpdateDocumentStatusFunc != nil {
		return m.UpdateDocumentStatusFunc(ctx, id, status)
	}
	return nil
}

// CreateVersion delegates to CreateVersionFunc.
func (m *MockRepository) CreateVersion(ctx context.Context, version *model.DocumentVersion) error {
	if m.CreateVersionFunc != nil {
		return m.CreateVersionFunc(ctx, version)
	}
	return nil
}

// GetVersionsForDocument delegates to GetVersionsForDocumentFunc.
func (m *MockRepository) GetVersionsForDocument(ctx context.Context, documentID uuid.UUID, page, pageSize int) ([]model.DocumentVersion, int, error) {
	if m.GetVersionsForDocumentFunc != nil {
		return m.GetVersionsForDocumentFunc(ctx, documentID, page, pageSize)
	}
	return nil, 0, nil
}

// GetVersionByID delegates to GetVersionByIDFunc.
func (m *MockRepository) GetVersionByID(ctx context.Context, id uuid.UUID) (*model.DocumentVersion, error) {
	if m.GetVersionByIDFunc != nil {
		return m.GetVersionByIDFunc(ctx, id)
	}
	return nil, nil
}

// CreateExportJob delegates to CreateExportJobFunc.
func (m *MockRepository) CreateExportJob(ctx context.Context, job *model.ExportJob) error {
	if m.CreateExportJobFunc != nil {
		return m.CreateExportJobFunc(ctx, job)
	}
	return nil
}

// GetExportJobByID delegates to GetExportJobByIDFunc.
func (m *MockRepository) GetExportJobByID(ctx context.Context, id uuid.UUID) (*model.ExportJob, error) {
	if m.GetExportJobByIDFunc != nil {
		return m.GetExportJobByIDFunc(ctx, id)
	}
	return nil, nil
}

// UpdateExportJobStatus delegates to UpdateExportJobStatusFunc.
func (m *MockRepository) UpdateExportJobStatus(ctx context.Context, id uuid.UUID, status string, filePath *string, fileSize *int64, errorMessage *string) error {
	if m.UpdateExportJobStatusFunc != nil {
		return m.UpdateExportJobStatusFunc(ctx, id, status, filePath, fileSize, errorMessage)
	}
	return nil
}

// DeleteDocument delegates to DeleteDocumentFunc.
func (m *MockRepository) DeleteDocument(ctx context.Context, id uuid.UUID) error {
	if m.DeleteDocumentFunc != nil {
		return m.DeleteDocumentFunc(ctx, id)
	}
	return nil
}
