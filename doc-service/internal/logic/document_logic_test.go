package logic

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	docserviceconfig "sovereign-ai-compliance/doc-service/internal/config"
	"sovereign-ai-compliance/doc-service/internal/generator"
	"sovereign-ai-compliance/doc-service/internal/types"
	"sovereign-ai-compliance/doc-service/model"
	"sovereign-ai-compliance/doc-service/repo"
	sharedconfig "sovereign-ai-compliance/shared/config"
	"sovereign-ai-compliance/shared/llm"
	"sovereign-ai-compliance/shared/tenant"
)

type mockLLMClient struct {
	content string
	err     error
}

func (m *mockLLMClient) Complete(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
	if m.err != nil {
		return llm.CompletionResponse{}, m.err
	}
	return llm.CompletionResponse{Content: m.content}, nil
}

func (m *mockLLMClient) Embed(ctx context.Context, req llm.EmbeddingRequest) (llm.EmbeddingResponse, error) {
	return llm.EmbeddingResponse{}, nil
}

func (m *mockLLMClient) Health(ctx context.Context) error {
	return nil
}

func (m *mockLLMClient) StreamComplete(ctx context.Context, req llm.CompletionRequest, onDelta func(token string)) (llm.StreamCompletionResponse, error) {
	resp, err := m.Complete(ctx, req)
	if err != nil {
		return llm.StreamCompletionResponse{}, err
	}
	return llm.StreamCompletionResponse{Content: resp.Content}, nil
}

func newTestDocumentLogic(mockRepo repo.Repository) *DocumentLogic {
	logger, _ := zap.NewDevelopment()
	return NewDocumentLogic(mockRepo, sharedconfig.LLMConfig{}, docserviceconfig.DefaultExportConfig(), logger, nil)
}

func TestDocumentLogic_GenerateDocument(t *testing.T) {
	mockRepo := &repo.MockRepository{}
	logic := newTestDocumentLogic(mockRepo)

	tenantID := uuid.New()
	ctx := tenant.WithContext(context.Background(), tenantID.String())

	var createdDoc *model.GeneratedDocument
	mockRepo.CreateDocumentFunc = func(ctx context.Context, doc *model.GeneratedDocument) error {
		createdDoc = doc
		return nil
	}

	req := types.GenerateDocumentRequest{
		AISystemID: uuid.New(),
		DocType:    model.DocTypeAnnexIV,
		Title:      "Test Title",
	}

	resp, err := logic.GenerateDocument(ctx, req)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, resp.DocumentID)
	assert.Equal(t, model.StatusGenerating, resp.Status)
	require.NotNil(t, createdDoc)
	assert.Equal(t, req.AISystemID, createdDoc.AISystemID)
	assert.Equal(t, req.DocType, createdDoc.DocType)
	assert.Equal(t, req.Title, createdDoc.Title)
	assert.Equal(t, 1, createdDoc.Version)
	assert.Equal(t, model.StatusGenerating, createdDoc.Status)
}

func TestDocumentLogic_GenerateDocument_MissingTenant(t *testing.T) {
	mockRepo := &repo.MockRepository{}
	logic := newTestDocumentLogic(mockRepo)
	ctx := context.Background()

	_, err := logic.GenerateDocument(ctx, types.GenerateDocumentRequest{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tenant context required")
}

func TestDocumentLogic_GenerateDocument_InvalidDocType(t *testing.T) {
	mockRepo := &repo.MockRepository{}
	logic := newTestDocumentLogic(mockRepo)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	_, err := logic.GenerateDocument(ctx, types.GenerateDocumentRequest{
		AISystemID: uuid.New(),
		DocType:    "invalid_type",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported document type")
}

func TestDocumentLogic_GenerateDocument_DefaultTitle(t *testing.T) {
	mockRepo := &repo.MockRepository{}
	logic := newTestDocumentLogic(mockRepo)

	tenantID := uuid.New()
	ctx := tenant.WithContext(context.Background(), tenantID.String())

	var createdDoc *model.GeneratedDocument
	mockRepo.CreateDocumentFunc = func(ctx context.Context, doc *model.GeneratedDocument) error {
		createdDoc = doc
		return nil
	}

	aiSystemID := uuid.New()
	_, err := logic.GenerateDocument(ctx, types.GenerateDocumentRequest{
		AISystemID: aiSystemID,
		DocType:    model.DocTypeAnnexIV,
	})
	require.NoError(t, err)
	require.NotNil(t, createdDoc)
	assert.Contains(t, createdDoc.Title, model.DocTypeAnnexIV)
	assert.Contains(t, createdDoc.Title, aiSystemID.String())
}

func TestDocumentLogic_GetDocument(t *testing.T) {
	tenantID := uuid.New()
	docID := uuid.New()
	mockRepo := &repo.MockRepository{
		GetDocumentByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.GeneratedDocument, error) {
			return &model.GeneratedDocument{
				ID:     docID,
				Title:  "Found",
				Status: model.StatusEditing,
			}, nil
		},
	}
	logic := newTestDocumentLogic(mockRepo)
	ctx := tenant.WithContext(context.Background(), tenantID.String())

	resp, err := logic.GetDocument(ctx, types.GetDocumentRequest{DocumentID: docID})
	require.NoError(t, err)
	assert.Equal(t, "Found", resp.Title)
}

func TestDocumentLogic_GetDocument_NotFound(t *testing.T) {
	mockRepo := &repo.MockRepository{
		GetDocumentByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.GeneratedDocument, error) {
			return nil, nil
		},
	}
	logic := newTestDocumentLogic(mockRepo)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	resp, err := logic.GetDocument(ctx, types.GetDocumentRequest{DocumentID: uuid.New()})
	require.NoError(t, err)
	assert.Nil(t, resp)
}

func TestDocumentLogic_ListDocuments(t *testing.T) {
	mockRepo := &repo.MockRepository{
		ListDocumentsFunc: func(ctx context.Context, aiSystemID *uuid.UUID, docType, status *string, page, pageSize int) ([]model.DocumentSummary, int, error) {
			return []model.DocumentSummary{
				{ID: uuid.New(), Title: "Doc 1"},
				{ID: uuid.New(), Title: "Doc 2"},
			}, 2, nil
		},
	}
	logic := newTestDocumentLogic(mockRepo)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	resp, err := logic.ListDocuments(ctx, types.ListDocumentsRequest{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Len(t, resp.Items, 2)
	assert.Equal(t, 2, resp.Total)
	assert.Equal(t, 1, resp.Page)
	assert.Equal(t, 1, resp.Pages)
}

func TestDocumentLogic_ListDocuments_DefaultPagination(t *testing.T) {
	mockRepo := &repo.MockRepository{
		ListDocumentsFunc: func(ctx context.Context, aiSystemID *uuid.UUID, docType, status *string, page, pageSize int) ([]model.DocumentSummary, int, error) {
			assert.Equal(t, 1, page)
			assert.Equal(t, 20, pageSize)
			return nil, 0, nil
		},
	}
	logic := newTestDocumentLogic(mockRepo)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	_, err := logic.ListDocuments(ctx, types.ListDocumentsRequest{})
	require.NoError(t, err)
}

func TestDocumentLogic_UpdateDocument(t *testing.T) {
	tenantID := uuid.New()
	docID := uuid.New()
	mockRepo := &repo.MockRepository{
		GetDocumentByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.GeneratedDocument, error) {
			return &model.GeneratedDocument{
				ID:      docID,
				Title:   "Original",
				Status:  model.StatusEditing,
				Version: 1,
				Content: model.DocumentContent{Sections: []model.Section{{ID: "s1", Title: "Old"}}},
			}, nil
		},
		CreateVersionFunc: func(ctx context.Context, version *model.DocumentVersion) error {
			return nil
		},
		UpdateDocumentFunc: func(ctx context.Context, doc *model.GeneratedDocument) error {
			assert.Equal(t, docID, doc.ID)
			assert.Equal(t, 2, doc.Version)
			assert.Equal(t, model.StatusEditing, doc.Status)
			return nil
		},
	}
	logic := newTestDocumentLogic(mockRepo)
	ctx := tenant.WithContext(context.Background(), tenantID.String())

	resp, err := logic.UpdateDocument(ctx, types.UpdateDocumentRequest{
		DocumentID: docID,
		Content:    model.DocumentContent{Sections: []model.Section{{ID: "s1", Title: "New"}}},
	})
	require.NoError(t, err)
	assert.Equal(t, 2, resp.Version)
}

func TestDocumentLogic_UpdateDocument_NotFound(t *testing.T) {
	mockRepo := &repo.MockRepository{
		GetDocumentByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.GeneratedDocument, error) {
			return nil, nil
		},
	}
	logic := newTestDocumentLogic(mockRepo)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	_, err := logic.UpdateDocument(ctx, types.UpdateDocumentRequest{DocumentID: uuid.New()})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "document not found")
}

func TestDocumentLogic_UpdateDocument_InvalidStatus(t *testing.T) {
	mockRepo := &repo.MockRepository{
		GetDocumentByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.GeneratedDocument, error) {
			return &model.GeneratedDocument{
				ID:     id,
				Status: model.StatusPublished,
			}, nil
		},
	}
	logic := newTestDocumentLogic(mockRepo)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	_, err := logic.UpdateDocument(ctx, types.UpdateDocumentRequest{DocumentID: uuid.New()})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot edit document with status")
}

func TestDocumentLogic_UpdateDocumentStatus(t *testing.T) {
	docID := uuid.New()
	mockRepo := &repo.MockRepository{
		GetDocumentByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.GeneratedDocument, error) {
			return &model.GeneratedDocument{
				ID:     docID,
				Status: model.StatusEditing,
			}, nil
		},
		UpdateDocumentStatusFunc: func(ctx context.Context, id uuid.UUID, status string) error {
			assert.Equal(t, docID, id)
			assert.Equal(t, model.StatusApproved, status)
			return nil
		},
	}
	logic := newTestDocumentLogic(mockRepo)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	resp, err := logic.UpdateDocumentStatus(ctx, types.UpdateDocumentStatusRequest{
		DocumentID: docID,
		Status:     model.StatusApproved,
	})
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, model.StatusApproved, resp.Status)
}

func TestDocumentLogic_UpdateDocumentStatus_InvalidStatus(t *testing.T) {
	mockRepo := &repo.MockRepository{}
	logic := newTestDocumentLogic(mockRepo)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	_, err := logic.UpdateDocumentStatus(ctx, types.UpdateDocumentStatusRequest{
		DocumentID: uuid.New(),
		Status:     "invalid",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid status")
}

func TestDocumentLogic_UpdateDocumentStatus_InvalidTransition(t *testing.T) {
	mockRepo := &repo.MockRepository{
		GetDocumentByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.GeneratedDocument, error) {
			return &model.GeneratedDocument{
				ID:     id,
				Status: model.StatusGenerating,
			}, nil
		},
	}
	logic := newTestDocumentLogic(mockRepo)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	_, err := logic.UpdateDocumentStatus(ctx, types.UpdateDocumentStatusRequest{
		DocumentID: uuid.New(),
		Status:     model.StatusPublished,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid status transition")
}

func TestDocumentLogic_UpdateDocumentStatus_NotFound(t *testing.T) {
	mockRepo := &repo.MockRepository{
		GetDocumentByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.GeneratedDocument, error) {
			return nil, nil
		},
	}
	logic := newTestDocumentLogic(mockRepo)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	_, err := logic.UpdateDocumentStatus(ctx, types.UpdateDocumentStatusRequest{
		DocumentID: uuid.New(),
		Status:     model.StatusApproved,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "document not found")
}

func TestDocumentLogic_UpdateDocumentStatus_RepoError(t *testing.T) {
	mockRepo := &repo.MockRepository{
		GetDocumentByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.GeneratedDocument, error) {
			return &model.GeneratedDocument{ID: id, Status: model.StatusEditing}, nil
		},
		UpdateDocumentStatusFunc: func(ctx context.Context, id uuid.UUID, status string) error {
			return errors.New("db error")
		},
	}
	logic := newTestDocumentLogic(mockRepo)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	_, err := logic.UpdateDocumentStatus(ctx, types.UpdateDocumentStatusRequest{
		DocumentID: uuid.New(),
		Status:     model.StatusApproved,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "update status")
}

func TestIsValidStatusTransition(t *testing.T) {
	tests := []struct {
		from   string
		to     string
		expect bool
	}{
		{model.StatusGenerating, model.StatusEditing, true},
		{model.StatusGenerating, model.StatusFailed, true},
		{model.StatusGenerating, model.StatusApproved, false},
		{model.StatusEditing, model.StatusApproved, true},
		{model.StatusEditing, model.StatusFailed, true},
		{model.StatusEditing, model.StatusGenerating, true},
		{model.StatusEditing, model.StatusPublished, false},
		{model.StatusApproved, model.StatusPublished, true},
		{model.StatusApproved, model.StatusEditing, true},
		{model.StatusApproved, model.StatusFailed, false},
		{model.StatusPublished, model.StatusEditing, true},
		{model.StatusPublished, model.StatusApproved, false},
		{model.StatusFailed, model.StatusGenerating, true},
		{model.StatusFailed, model.StatusEditing, true},
		{model.StatusFailed, model.StatusApproved, false},
		{"unknown", model.StatusEditing, false},
	}

	for _, tt := range tests {
		result := isValidStatusTransition(tt.from, tt.to)
		assert.Equal(t, tt.expect, result, "transition %s -> %s", tt.from, tt.to)
	}
}

func TestDocumentLogic_runGeneration(t *testing.T) {
	tenantID := uuid.New()
	docID := uuid.New()
	doc := &model.GeneratedDocument{
		ID:        docID,
		TenantID:  tenantID,
		DocType:   model.DocTypeAnnexIV,
		Status:    model.StatusGenerating,
		Version:   1,
		CreatedBy: tenantID,
	}

	mockRepo := &repo.MockRepository{
		UpdateDocumentFunc: func(ctx context.Context, d *model.GeneratedDocument) error {
			assert.Equal(t, docID, d.ID)
			assert.Equal(t, model.StatusEditing, d.Status)
			return nil
		},
		CreateVersionFunc: func(ctx context.Context, version *model.DocumentVersion) error {
			assert.Equal(t, docID, version.DocumentID)
			return nil
		},
	}

	logger, _ := zap.NewDevelopment()
	mockLLM := &mockLLMClient{content: "Generated section content"}
	gen := generator.NewDocumentGenerator(mockLLM, logger)

	logic := &DocumentLogic{
		repo:      mockRepo,
		gen:       gen,
		assembler: generator.NewDataAssembler(nil),
		logger:    logger,
	}

	logic.runGeneration(doc, nil)

	assert.Equal(t, model.StatusEditing, doc.Status)
	assert.Greater(t, len(doc.Content.Sections), 0)
}

func TestDocumentLogic_runGeneration_NilLLM(t *testing.T) {
	tenantID := uuid.New()
	docID := uuid.New()
	doc := &model.GeneratedDocument{
		ID:        docID,
		TenantID:  tenantID,
		DocType:   model.DocTypeAnnexIV,
		Status:    model.StatusGenerating,
		Version:   1,
		CreatedBy: tenantID,
	}

	var failedDocID uuid.UUID
	mockRepo := &repo.MockRepository{
		UpdateDocumentStatusFunc: func(ctx context.Context, id uuid.UUID, status string) error {
			failedDocID = id
			return nil
		},
	}

	logger, _ := zap.NewDevelopment()
	gen := generator.NewDocumentGenerator(nil, logger)

	logic := &DocumentLogic{
		repo:      mockRepo,
		gen:       gen,
		assembler: generator.NewDataAssembler(nil),
		logger:    logger,
	}

	logic.runGeneration(doc, nil)

	assert.Equal(t, docID, failedDocID)
}

func TestDocumentLogic_runGeneration_UpdateError(t *testing.T) {
	tenantID := uuid.New()
	docID := uuid.New()
	doc := &model.GeneratedDocument{
		ID:        docID,
		TenantID:  tenantID,
		DocType:   model.DocTypeAnnexIV,
		Status:    model.StatusGenerating,
		Version:   1,
		CreatedBy: tenantID,
	}

	var failedDocID uuid.UUID
	mockRepo := &repo.MockRepository{
		UpdateDocumentFunc: func(ctx context.Context, d *model.GeneratedDocument) error {
			return errors.New("update failed")
		},
		UpdateDocumentStatusFunc: func(ctx context.Context, id uuid.UUID, status string) error {
			failedDocID = id
			return nil
		},
	}

	logger, _ := zap.NewDevelopment()
	mockLLM := &mockLLMClient{content: "Generated section content"}
	gen := generator.NewDocumentGenerator(mockLLM, logger)

	logic := &DocumentLogic{
		repo:      mockRepo,
		gen:       gen,
		assembler: generator.NewDataAssembler(nil),
		logger:    logger,
	}

	logic.runGeneration(doc, nil)

	assert.Equal(t, docID, failedDocID)
}

func TestDocumentLogic_runGeneration_CreateVersionError(t *testing.T) {
	tenantID := uuid.New()
	docID := uuid.New()
	doc := &model.GeneratedDocument{
		ID:        docID,
		TenantID:  tenantID,
		DocType:   model.DocTypeAnnexIV,
		Status:    model.StatusGenerating,
		Version:   1,
		CreatedBy: tenantID,
	}

	mockRepo := &repo.MockRepository{
		UpdateDocumentFunc: func(ctx context.Context, d *model.GeneratedDocument) error {
			return nil
		},
		CreateVersionFunc: func(ctx context.Context, version *model.DocumentVersion) error {
			return errors.New("version create failed")
		},
	}

	logger, _ := zap.NewDevelopment()
	mockLLM := &mockLLMClient{content: "Generated section content"}
	gen := generator.NewDocumentGenerator(mockLLM, logger)

	logic := &DocumentLogic{
		repo:      mockRepo,
		gen:       gen,
		assembler: generator.NewDataAssembler(nil),
		logger:    logger,
	}

	logic.runGeneration(doc, nil)

	assert.Equal(t, model.StatusEditing, doc.Status)
}

func TestDocumentLogic_DeleteDocument(t *testing.T) {
	tenantID := uuid.New()
	docID := uuid.New()
	mockRepo := &repo.MockRepository{
		GetDocumentByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.GeneratedDocument, error) {
			return &model.GeneratedDocument{
				ID:       docID,
				TenantID: tenantID,
				Status:   model.StatusEditing,
			}, nil
		},
		DeleteDocumentFunc: func(ctx context.Context, id uuid.UUID) error {
			assert.Equal(t, docID, id)
			return nil
		},
	}
	logic := newTestDocumentLogic(mockRepo)
	ctx := tenant.WithContext(context.Background(), tenantID.String())

	resp, err := logic.DeleteDocument(ctx, types.DeleteDocumentRequest{DocumentID: docID})
	require.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestDocumentLogic_DeleteDocument_NotFound(t *testing.T) {
	mockRepo := &repo.MockRepository{
		GetDocumentByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.GeneratedDocument, error) {
			return nil, nil
		},
	}
	logic := newTestDocumentLogic(mockRepo)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	_, err := logic.DeleteDocument(ctx, types.DeleteDocumentRequest{DocumentID: uuid.New()})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "document not found")
}

func TestDocumentLogic_DeleteDocument_GetError(t *testing.T) {
	mockRepo := &repo.MockRepository{
		GetDocumentByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.GeneratedDocument, error) {
			return nil, errors.New("db connection lost")
		},
	}
	logic := newTestDocumentLogic(mockRepo)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	_, err := logic.DeleteDocument(ctx, types.DeleteDocumentRequest{DocumentID: uuid.New()})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "get document")
}

func TestDocumentLogic_DeleteDocument_RepoError(t *testing.T) {
	docID := uuid.New()
	mockRepo := &repo.MockRepository{
		GetDocumentByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.GeneratedDocument, error) {
			return &model.GeneratedDocument{
				ID:     docID,
				Status: model.StatusEditing,
			}, nil
		},
		DeleteDocumentFunc: func(ctx context.Context, id uuid.UUID) error {
			return errors.New("delete failed")
		},
	}
	logic := newTestDocumentLogic(mockRepo)
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	_, err := logic.DeleteDocument(ctx, types.DeleteDocumentRequest{DocumentID: docID})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "delete document")
}

func TestDocumentLogic_DeleteDocument_MissingTenant(t *testing.T) {
	mockRepo := &repo.MockRepository{
		GetDocumentByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.GeneratedDocument, error) {
			return nil, errors.New("tenant context required")
		},
	}
	logic := newTestDocumentLogic(mockRepo)
	ctx := context.Background()

	_, err := logic.DeleteDocument(ctx, types.DeleteDocumentRequest{DocumentID: uuid.New()})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "get document")
}

func TestDocumentLogic_failGeneration_Success(t *testing.T) {
	docID := uuid.New()
	mockRepo := &repo.MockRepository{
		UpdateDocumentStatusFunc: func(ctx context.Context, id uuid.UUID, status string) error {
			assert.Equal(t, model.StatusFailed, status)
			return nil
		},
	}

	logger, _ := zap.NewDevelopment()
	logic := &DocumentLogic{repo: mockRepo, logger: logger}
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	logic.failGeneration(ctx, docID, "test failure reason")
}
