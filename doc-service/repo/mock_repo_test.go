package repo

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sovereign-ai-compliance/doc-service/model"
)

func TestMockRepository_CreateDocument(t *testing.T) {
	called := false
	m := &MockRepository{
		CreateDocumentFunc: func(ctx context.Context, doc *model.GeneratedDocument) error {
			called = true
			return nil
		},
	}
	err := m.CreateDocument(context.Background(), &model.GeneratedDocument{})
	require.NoError(t, err)
	assert.True(t, called)
}

func TestMockRepository_GetDocumentByID(t *testing.T) {
	m := &MockRepository{
		GetDocumentByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.GeneratedDocument, error) {
			return &model.GeneratedDocument{ID: id}, nil
		},
	}
	doc, err := m.GetDocumentByID(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.NotNil(t, doc)
}

func TestMockRepository_ListDocuments(t *testing.T) {
	m := &MockRepository{
		ListDocumentsFunc: func(ctx context.Context, aiSystemID *uuid.UUID, docType, status *string, page, pageSize int) ([]model.DocumentSummary, int, error) {
			return []model.DocumentSummary{{ID: uuid.New()}}, 1, nil
		},
	}
	items, total, err := m.ListDocuments(context.Background(), nil, nil, nil, 1, 10)
	require.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, 1, total)
}

func TestMockRepository_UpdateDocument(t *testing.T) {
	called := false
	m := &MockRepository{
		UpdateDocumentFunc: func(ctx context.Context, doc *model.GeneratedDocument) error {
			called = true
			return nil
		},
	}
	err := m.UpdateDocument(context.Background(), &model.GeneratedDocument{})
	require.NoError(t, err)
	assert.True(t, called)
}

func TestMockRepository_UpdateDocumentStatus(t *testing.T) {
	called := false
	m := &MockRepository{
		UpdateDocumentStatusFunc: func(ctx context.Context, id uuid.UUID, status string) error {
			called = true
			return nil
		},
	}
	err := m.UpdateDocumentStatus(context.Background(), uuid.New(), "editing")
	require.NoError(t, err)
	assert.True(t, called)
}

func TestMockRepository_CreateVersion(t *testing.T) {
	called := false
	m := &MockRepository{
		CreateVersionFunc: func(ctx context.Context, version *model.DocumentVersion) error {
			called = true
			return nil
		},
	}
	err := m.CreateVersion(context.Background(), &model.DocumentVersion{})
	require.NoError(t, err)
	assert.True(t, called)
}

func TestMockRepository_GetVersionsForDocument(t *testing.T) {
	m := &MockRepository{
		GetVersionsForDocumentFunc: func(ctx context.Context, documentID uuid.UUID, page, pageSize int) ([]model.DocumentVersion, int, error) {
			return []model.DocumentVersion{{ID: uuid.New()}}, 1, nil
		},
	}
	items, total, err := m.GetVersionsForDocument(context.Background(), uuid.New(), 1, 10)
	require.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, 1, total)
}

func TestMockRepository_GetVersionByID(t *testing.T) {
	m := &MockRepository{
		GetVersionByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.DocumentVersion, error) {
			return &model.DocumentVersion{ID: id}, nil
		},
	}
	v, err := m.GetVersionByID(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.NotNil(t, v)
}

func TestMockRepository_CreateExportJob(t *testing.T) {
	called := false
	m := &MockRepository{
		CreateExportJobFunc: func(ctx context.Context, job *model.ExportJob) error {
			called = true
			return nil
		},
	}
	err := m.CreateExportJob(context.Background(), &model.ExportJob{})
	require.NoError(t, err)
	assert.True(t, called)
}

func TestMockRepository_GetExportJobByID(t *testing.T) {
	m := &MockRepository{
		GetExportJobByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.ExportJob, error) {
			return &model.ExportJob{ID: id}, nil
		},
	}
	job, err := m.GetExportJobByID(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.NotNil(t, job)
}

func TestMockRepository_UpdateExportJobStatus(t *testing.T) {
	called := false
	m := &MockRepository{
		UpdateExportJobStatusFunc: func(ctx context.Context, id uuid.UUID, status string, filePath *string, fileSize *int64, errorMessage *string) error {
			called = true
			return nil
		},
	}
	err := m.UpdateExportJobStatus(context.Background(), uuid.New(), "completed", nil, nil, nil)
	require.NoError(t, err)
	assert.True(t, called)
}

func TestMockRepository_DB(t *testing.T) {
	m := &MockRepository{}
	assert.Nil(t, m.DB())
}
