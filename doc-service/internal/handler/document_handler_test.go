package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	docserviceconfig "sovereign-ai-compliance/doc-service/internal/config"
	"sovereign-ai-compliance/doc-service/internal/logic"
	"sovereign-ai-compliance/doc-service/internal/types"
	"sovereign-ai-compliance/doc-service/model"
	"sovereign-ai-compliance/doc-service/repo"
	sharedconfig "sovereign-ai-compliance/shared/config"
	"sovereign-ai-compliance/shared/tenant"
)

func newTestDocumentHandler(mockRepo repo.Repository) *DocumentHandler {
	logger, _ := zap.NewDevelopment()
	docLogic := logic.NewDocumentLogic(mockRepo, sharedconfig.LLMConfig{}, docserviceconfig.DefaultExportConfig(), logger)
	versionLogic := logic.NewVersionLogic(mockRepo, logger)
	return NewDocumentHandler(docLogic, versionLogic)
}

func addTenantContext(req *http.Request, tenantID string) *http.Request {
	return req.WithContext(tenant.WithContext(req.Context(), tenantID))
}

func TestDocumentHandler_ListDocuments(t *testing.T) {
	mockRepo := &repo.MockRepository{
		ListDocumentsFunc: func(ctx context.Context, aiSystemID *uuid.UUID, docType, status *string, page, pageSize int) ([]model.DocumentSummary, int, error) {
			return []model.DocumentSummary{
				{ID: uuid.New(), Title: "Doc 1"},
			}, 1, nil
		},
	}
	h := newTestDocumentHandler(mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/generated-documents?page=1&page_size=10", nil)
	req = addTenantContext(req, uuid.New().String())
	rr := httptest.NewRecorder()

	h.ListDocuments(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp types.ListDocumentsResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Len(t, resp.Items, 1)
	assert.Equal(t, 1, resp.Total)
}

func TestDocumentHandler_ListDocuments_InvalidQuery(t *testing.T) {
	mockRepo := &repo.MockRepository{}
	h := newTestDocumentHandler(mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/generated-documents?page=invalid", nil)
	req = addTenantContext(req, uuid.New().String())
	rr := httptest.NewRecorder()

	h.ListDocuments(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestDocumentHandler_GenerateDocument(t *testing.T) {
	mockRepo := &repo.MockRepository{
		CreateDocumentFunc: func(ctx context.Context, doc *model.GeneratedDocument) error {
			return nil
		},
	}
	h := newTestDocumentHandler(mockRepo)

	body, _ := json.Marshal(types.GenerateDocumentRequest{
		AISystemID: uuid.New(),
		DocType:    model.DocTypeAnnexIV,
		Title:      "Test",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/generated-documents", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addTenantContext(req, uuid.New().String())
	rr := httptest.NewRecorder()

	h.GenerateDocument(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp types.GenerateDocumentResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.NotEqual(t, uuid.Nil, resp.DocumentID)
	assert.Equal(t, model.StatusGenerating, resp.Status)
}

func TestDocumentHandler_GenerateDocument_InvalidBody(t *testing.T) {
	mockRepo := &repo.MockRepository{}
	h := newTestDocumentHandler(mockRepo)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/generated-documents", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	req = addTenantContext(req, uuid.New().String())
	rr := httptest.NewRecorder()

	h.GenerateDocument(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestDocumentHandler_GenerateDocument_MissingAISystemID(t *testing.T) {
	mockRepo := &repo.MockRepository{}
	h := newTestDocumentHandler(mockRepo)

	body, _ := json.Marshal(types.GenerateDocumentRequest{
		DocType: model.DocTypeAnnexIV,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/generated-documents", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addTenantContext(req, uuid.New().String())
	rr := httptest.NewRecorder()

	h.GenerateDocument(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestDocumentHandler_GenerateDocument_MissingDocType(t *testing.T) {
	mockRepo := &repo.MockRepository{}
	h := newTestDocumentHandler(mockRepo)

	body, _ := json.Marshal(types.GenerateDocumentRequest{
		AISystemID: uuid.New(),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/generated-documents", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addTenantContext(req, uuid.New().String())
	rr := httptest.NewRecorder()

	h.GenerateDocument(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestDocumentHandler_GetDocument(t *testing.T) {
	docID := uuid.New()
	mockRepo := &repo.MockRepository{
		GetDocumentByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.GeneratedDocument, error) {
			return &model.GeneratedDocument{
				ID:     id,
				Title:  "Found",
				Status: model.StatusEditing,
			}, nil
		},
	}
	h := newTestDocumentHandler(mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/generated-documents/"+docID.String(), nil)
	req = addTenantContext(req, uuid.New().String())
	req.SetPathValue("id", docID.String())
	rr := httptest.NewRecorder()

	h.GetDocument(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp types.GetDocumentResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "Found", resp.Title)
}

func TestDocumentHandler_GetDocument_InvalidID(t *testing.T) {
	mockRepo := &repo.MockRepository{}
	h := newTestDocumentHandler(mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/generated-documents/invalid", nil)
	req = addTenantContext(req, uuid.New().String())
	req.SetPathValue("id", "invalid")
	rr := httptest.NewRecorder()

	h.GetDocument(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestDocumentHandler_GetDocument_NotFound(t *testing.T) {
	mockRepo := &repo.MockRepository{
		GetDocumentByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.GeneratedDocument, error) {
			return nil, nil
		},
	}
	h := newTestDocumentHandler(mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/generated-documents/"+uuid.New().String(), nil)
	req = addTenantContext(req, uuid.New().String())
	req.SetPathValue("id", uuid.New().String())
	rr := httptest.NewRecorder()

	h.GetDocument(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestDocumentHandler_UpdateDocument(t *testing.T) {
	docID := uuid.New()
	mockRepo := &repo.MockRepository{
		GetDocumentByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.GeneratedDocument, error) {
			return &model.GeneratedDocument{
				ID:      id,
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
			return nil
		},
	}
	h := newTestDocumentHandler(mockRepo)

	body, _ := json.Marshal(types.UpdateDocumentRequest{
		Content: model.DocumentContent{Sections: []model.Section{{ID: "s1", Title: "New"}}},
	})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/generated-documents/"+docID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addTenantContext(req, uuid.New().String())
	req.SetPathValue("id", docID.String())
	rr := httptest.NewRecorder()

	h.UpdateDocument(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp types.UpdateDocumentResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, 2, resp.Version)
}

func TestDocumentHandler_UpdateDocument_InvalidBody(t *testing.T) {
	mockRepo := &repo.MockRepository{}
	h := newTestDocumentHandler(mockRepo)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/generated-documents/"+uuid.New().String(), bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	req = addTenantContext(req, uuid.New().String())
	req.SetPathValue("id", uuid.New().String())
	rr := httptest.NewRecorder()

	h.UpdateDocument(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestDocumentHandler_UpdateDocument_InvalidID(t *testing.T) {
	mockRepo := &repo.MockRepository{}
	h := newTestDocumentHandler(mockRepo)

	body, _ := json.Marshal(types.UpdateDocumentRequest{})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/generated-documents/invalid", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addTenantContext(req, uuid.New().String())
	req.SetPathValue("id", "invalid")
	rr := httptest.NewRecorder()

	h.UpdateDocument(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestDocumentHandler_ListVersions(t *testing.T) {
	docID := uuid.New()
	mockRepo := &repo.MockRepository{
		GetVersionsForDocumentFunc: func(ctx context.Context, documentID uuid.UUID, page, pageSize int) ([]model.DocumentVersion, int, error) {
			return []model.DocumentVersion{
				{ID: uuid.New(), VersionNumber: 1, DocumentID: docID},
			}, 1, nil
		},
	}
	h := newTestDocumentHandler(mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/generated-documents/"+docID.String()+"/versions", nil)
	req = addTenantContext(req, uuid.New().String())
	req.SetPathValue("id", docID.String())
	rr := httptest.NewRecorder()

	h.ListVersions(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp types.ListVersionsResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Len(t, resp.Items, 1)
}

func TestDocumentHandler_ListVersions_InvalidID(t *testing.T) {
	mockRepo := &repo.MockRepository{}
	h := newTestDocumentHandler(mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/generated-documents/invalid/versions", nil)
	req = addTenantContext(req, uuid.New().String())
	req.SetPathValue("id", "invalid")
	rr := httptest.NewRecorder()

	h.ListVersions(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestDocumentHandler_RollbackVersion(t *testing.T) {
	docID := uuid.New()
	mockRepo := &repo.MockRepository{
		GetDocumentByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.GeneratedDocument, error) {
			return &model.GeneratedDocument{
				ID:      id,
				Version: 3,
				Status:  model.StatusEditing,
				Content: model.DocumentContent{Sections: []model.Section{{ID: "s1", Title: "Current"}}},
			}, nil
		},
		GetVersionsForDocumentFunc: func(ctx context.Context, documentID uuid.UUID, page, pageSize int) ([]model.DocumentVersion, int, error) {
			return []model.DocumentVersion{
				{ID: uuid.New(), VersionNumber: 2, DocumentID: docID, Content: model.DocumentContent{Sections: []model.Section{{ID: "s1", Title: "V2"}}}},
				{ID: uuid.New(), VersionNumber: 1, DocumentID: docID, Content: model.DocumentContent{Sections: []model.Section{{ID: "s1", Title: "V1"}}}},
			}, 2, nil
		},
		CreateVersionFunc: func(ctx context.Context, version *model.DocumentVersion) error {
			return nil
		},
		UpdateDocumentFunc: func(ctx context.Context, doc *model.GeneratedDocument) error {
			return nil
		},
	}
	h := newTestDocumentHandler(mockRepo)

	body, _ := json.Marshal(types.RollbackVersionRequest{VersionNumber: 1})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/generated-documents/"+docID.String()+"/rollback", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addTenantContext(req, uuid.New().String())
	req.SetPathValue("id", docID.String())
	rr := httptest.NewRecorder()

	h.RollbackVersion(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp types.RollbackVersionResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "V1", resp.Content.Sections[0].Title)
}

func TestDocumentHandler_RollbackVersion_InvalidBody(t *testing.T) {
	mockRepo := &repo.MockRepository{}
	h := newTestDocumentHandler(mockRepo)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/generated-documents/"+uuid.New().String()+"/rollback", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	req = addTenantContext(req, uuid.New().String())
	req.SetPathValue("id", uuid.New().String())
	rr := httptest.NewRecorder()

	h.RollbackVersion(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestDocumentHandler_RollbackVersion_InvalidID(t *testing.T) {
	mockRepo := &repo.MockRepository{}
	h := newTestDocumentHandler(mockRepo)

	body, _ := json.Marshal(types.RollbackVersionRequest{VersionNumber: 1})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/generated-documents/invalid/rollback", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addTenantContext(req, uuid.New().String())
	req.SetPathValue("id", "invalid")
	rr := httptest.NewRecorder()

	h.RollbackVersion(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestDocumentHandler_ListDocuments_RepoError(t *testing.T) {
	mockRepo := &repo.MockRepository{
		ListDocumentsFunc: func(ctx context.Context, aiSystemID *uuid.UUID, docType, status *string, page, pageSize int) ([]model.DocumentSummary, int, error) {
			return nil, 0, errors.New("db error")
		},
	}
	h := newTestDocumentHandler(mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/generated-documents", nil)
	req = addTenantContext(req, uuid.New().String())
	rr := httptest.NewRecorder()

	h.ListDocuments(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}
