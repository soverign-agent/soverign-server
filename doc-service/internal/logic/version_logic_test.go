package logic

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"sovereign-ai-compliance/doc-service/internal/types"
	"sovereign-ai-compliance/doc-service/model"
	"sovereign-ai-compliance/doc-service/repo"
	"sovereign-ai-compliance/shared/tenant"
)

func newTestVersionLogic(mockRepo repo.Repository) *VersionLogic {
	logger, _ := zap.NewDevelopment()
	return NewVersionLogic(mockRepo, logger)
}

func TestVersionLogic_ListVersions(t *testing.T) {
	docID := uuid.New()
	mockRepo := &repo.MockRepository{
		GetVersionsForDocumentFunc: func(ctx context.Context, documentID uuid.UUID, page, pageSize int) ([]model.DocumentVersion, int, error) {
			assert.Equal(t, docID, documentID)
			assert.Equal(t, 1, page)
			assert.Equal(t, 10, pageSize)
			return []model.DocumentVersion{
				{ID: uuid.New(), VersionNumber: 2, DocumentID: docID},
				{ID: uuid.New(), VersionNumber: 1, DocumentID: docID},
			}, 2, nil
		},
	}
	logic := newTestVersionLogic(mockRepo)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	resp, err := logic.ListVersions(ctx, types.ListVersionsRequest{
		DocumentID: docID,
		Page:       1,
		PageSize:   10,
	})
	require.NoError(t, err)
	assert.Len(t, resp.Items, 2)
	assert.Equal(t, 2, resp.Total)
	assert.Equal(t, 1, resp.Pages)
}

func TestVersionLogic_ListVersions_DefaultPagination(t *testing.T) {
	mockRepo := &repo.MockRepository{
		GetVersionsForDocumentFunc: func(ctx context.Context, documentID uuid.UUID, page, pageSize int) ([]model.DocumentVersion, int, error) {
			assert.Equal(t, 1, page)
			assert.Equal(t, 20, pageSize)
			return nil, 0, nil
		},
	}
	logic := newTestVersionLogic(mockRepo)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	_, err := logic.ListVersions(ctx, types.ListVersionsRequest{
		DocumentID: uuid.New(),
	})
	require.NoError(t, err)
}

func TestVersionLogic_ListVersions_RepoError(t *testing.T) {
	mockRepo := &repo.MockRepository{
		GetVersionsForDocumentFunc: func(ctx context.Context, documentID uuid.UUID, page, pageSize int) ([]model.DocumentVersion, int, error) {
			return nil, 0, errors.New("db error")
		},
	}
	logic := newTestVersionLogic(mockRepo)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	_, err := logic.ListVersions(ctx, types.ListVersionsRequest{
		DocumentID: uuid.New(),
		Page:       1,
		PageSize:   10,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "list versions")
}

func TestVersionLogic_RollbackToVersion(t *testing.T) {
	docID := uuid.New()
	tenantID := uuid.New()
	targetVersionNum := 1

	mockRepo := &repo.MockRepository{
		GetDocumentByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.GeneratedDocument, error) {
			return &model.GeneratedDocument{
				ID:       docID,
				TenantID: tenantID,
				Version:  3,
				Status:   model.StatusEditing,
				Content:  model.DocumentContent{Sections: []model.Section{{ID: "s1", Title: "Current"}}},
			}, nil
		},
		GetVersionsForDocumentFunc: func(ctx context.Context, documentID uuid.UUID, page, pageSize int) ([]model.DocumentVersion, int, error) {
			return []model.DocumentVersion{
				{ID: uuid.New(), VersionNumber: 2, DocumentID: docID, Content: model.DocumentContent{Sections: []model.Section{{ID: "s1", Title: "V2"}}}},
				{ID: uuid.New(), VersionNumber: targetVersionNum, DocumentID: docID, Content: model.DocumentContent{Sections: []model.Section{{ID: "s1", Title: "V1"}}}},
			}, 2, nil
		},
		CreateVersionFunc: func(ctx context.Context, version *model.DocumentVersion) error {
			return nil
		},
		UpdateDocumentFunc: func(ctx context.Context, doc *model.GeneratedDocument) error {
			assert.Equal(t, docID, doc.ID)
			assert.Equal(t, 4, doc.Version)
			assert.Equal(t, model.StatusEditing, doc.Status)
			return nil
		},
	}
	logic := newTestVersionLogic(mockRepo)
	ctx := tenant.WithContext(context.Background(), tenantID.String())

	resp, err := logic.RollbackToVersion(ctx, types.RollbackVersionRequest{
		DocumentID:    docID,
		VersionNumber: targetVersionNum,
	})
	require.NoError(t, err)
	assert.Equal(t, 4, resp.Version)
	assert.Equal(t, "V1", resp.Content.Sections[0].Title)
}

func TestVersionLogic_RollbackToVersion_DocumentNotFound(t *testing.T) {
	mockRepo := &repo.MockRepository{
		GetDocumentByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.GeneratedDocument, error) {
			return nil, nil
		},
	}
	logic := newTestVersionLogic(mockRepo)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	_, err := logic.RollbackToVersion(ctx, types.RollbackVersionRequest{
		DocumentID:    uuid.New(),
		VersionNumber: 1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "document not found")
}

func TestVersionLogic_RollbackToVersion_VersionNotFound(t *testing.T) {
	docID := uuid.New()
	mockRepo := &repo.MockRepository{
		GetDocumentByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.GeneratedDocument, error) {
			return &model.GeneratedDocument{ID: docID, Version: 2}, nil
		},
		GetVersionsForDocumentFunc: func(ctx context.Context, documentID uuid.UUID, page, pageSize int) ([]model.DocumentVersion, int, error) {
			return []model.DocumentVersion{
				{VersionNumber: 2, DocumentID: docID},
			}, 1, nil
		},
	}
	logic := newTestVersionLogic(mockRepo)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	_, err := logic.RollbackToVersion(ctx, types.RollbackVersionRequest{
		DocumentID:    docID,
		VersionNumber: 99,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "version 99 not found")
}

func TestVersionLogic_RollbackToVersion_UpdateError(t *testing.T) {
	docID := uuid.New()
	mockRepo := &repo.MockRepository{
		GetDocumentByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.GeneratedDocument, error) {
			return &model.GeneratedDocument{ID: docID, Version: 2, TenantID: uuid.New()}, nil
		},
		GetVersionsForDocumentFunc: func(ctx context.Context, documentID uuid.UUID, page, pageSize int) ([]model.DocumentVersion, int, error) {
			return []model.DocumentVersion{
				{VersionNumber: 1, DocumentID: docID, Content: model.DocumentContent{}},
			}, 1, nil
		},
		CreateVersionFunc: func(ctx context.Context, version *model.DocumentVersion) error {
			return nil
		},
		UpdateDocumentFunc: func(ctx context.Context, doc *model.GeneratedDocument) error {
			return errors.New("update failed")
		},
	}
	logic := newTestVersionLogic(mockRepo)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	_, err := logic.RollbackToVersion(ctx, types.RollbackVersionRequest{
		DocumentID:    docID,
		VersionNumber: 1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "update document after rollback")
}

func TestVersionLogic_RollbackToVersion_RepoError(t *testing.T) {
	docID := uuid.New()
	mockRepo := &repo.MockRepository{
		GetDocumentByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.GeneratedDocument, error) {
			return &model.GeneratedDocument{ID: docID, Version: 2}, nil
		},
		GetVersionsForDocumentFunc: func(ctx context.Context, documentID uuid.UUID, page, pageSize int) ([]model.DocumentVersion, int, error) {
			return nil, 0, errors.New("db error")
		},
	}
	logic := newTestVersionLogic(mockRepo)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	_, err := logic.RollbackToVersion(ctx, types.RollbackVersionRequest{
		DocumentID:    docID,
		VersionNumber: 1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "list versions")
}
