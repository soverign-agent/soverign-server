package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
)

func newTestExportHandler(mockRepo repo.Repository, exportDir string) *ExportHandler {
	logger, _ := zap.NewDevelopment()
	exportLogic := logic.NewExportLogic(mockRepo, docserviceconfig.ExportConfig{OutputDir: exportDir}, logger)
	return NewExportHandler(exportLogic)
}

func TestExportHandler_CreateExportJob(t *testing.T) {
	docID := uuid.New()
	mockRepo := &repo.MockRepository{
		GetDocumentByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.GeneratedDocument, error) {
			return &model.GeneratedDocument{
				ID:      docID,
				Title:   "Test Doc",
				Version: 1,
			}, nil
		},
		CreateExportJobFunc: func(ctx context.Context, job *model.ExportJob) error {
			return nil
		},
	}

	exportDir := t.TempDir()
	logger, _ := zap.NewDevelopment()
	exportLogic := logic.NewExportLogic(mockRepo, docserviceconfig.ExportConfig{OutputDir: exportDir}, logger)
	h := NewExportHandler(exportLogic)

	body, _ := json.Marshal(types.CreateExportJobRequest{Format: "pdf"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/generated-documents/"+docID.String()+"/export", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addTenantContext(req, uuid.New().String())
	req.SetPathValue("id", docID.String())
	rr := httptest.NewRecorder()

	h.CreateExportJob(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp types.CreateExportJobResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.NotEqual(t, uuid.Nil, resp.JobID)
	assert.Equal(t, model.ExportStatusPending, resp.Status)
}

func TestExportHandler_CreateExportJob_InvalidID(t *testing.T) {
	mockRepo := &repo.MockRepository{}
	exportDir := t.TempDir()
	logger, _ := zap.NewDevelopment()
	exportLogic := logic.NewExportLogic(mockRepo, docserviceconfig.ExportConfig{OutputDir: exportDir}, logger)
	h := NewExportHandler(exportLogic)

	body, _ := json.Marshal(types.CreateExportJobRequest{Format: "pdf"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/generated-documents/invalid/export", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addTenantContext(req, uuid.New().String())
	req.SetPathValue("id", "invalid")
	rr := httptest.NewRecorder()

	h.CreateExportJob(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestExportHandler_CreateExportJob_InvalidBody(t *testing.T) {
	mockRepo := &repo.MockRepository{}
	exportDir := t.TempDir()
	logger, _ := zap.NewDevelopment()
	exportLogic := logic.NewExportLogic(mockRepo, docserviceconfig.ExportConfig{OutputDir: exportDir}, logger)
	h := NewExportHandler(exportLogic)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/generated-documents/"+uuid.New().String()+"/export", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	req = addTenantContext(req, uuid.New().String())
	req.SetPathValue("id", uuid.New().String())
	rr := httptest.NewRecorder()

	h.CreateExportJob(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestExportHandler_GetExportJobStatus(t *testing.T) {
	jobID := uuid.New()
	mockRepo := &repo.MockRepository{
		GetExportJobByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.ExportJob, error) {
			return &model.ExportJob{
				ID:     id,
				Status: model.ExportStatusCompleted,
			}, nil
		},
	}
	exportDir := t.TempDir()
	logger, _ := zap.NewDevelopment()
	exportLogic := logic.NewExportLogic(mockRepo, docserviceconfig.ExportConfig{OutputDir: exportDir}, logger)
	h := NewExportHandler(exportLogic)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/export-jobs/"+jobID.String(), nil)
	req = addTenantContext(req, uuid.New().String())
	req.SetPathValue("id", jobID.String())
	rr := httptest.NewRecorder()

	h.GetExportJobStatus(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp types.GetExportJobResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, model.ExportStatusCompleted, resp.Status)
}

func TestExportHandler_GetExportJobStatus_InvalidID(t *testing.T) {
	mockRepo := &repo.MockRepository{}
	exportDir := t.TempDir()
	logger, _ := zap.NewDevelopment()
	exportLogic := logic.NewExportLogic(mockRepo, docserviceconfig.ExportConfig{OutputDir: exportDir}, logger)
	h := NewExportHandler(exportLogic)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/export-jobs/invalid", nil)
	req = addTenantContext(req, uuid.New().String())
	req.SetPathValue("id", "invalid")
	rr := httptest.NewRecorder()

	h.GetExportJobStatus(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestExportHandler_GetExportJobStatus_NotFound(t *testing.T) {
	mockRepo := &repo.MockRepository{
		GetExportJobByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.ExportJob, error) {
			return nil, nil
		},
	}
	exportDir := t.TempDir()
	logger, _ := zap.NewDevelopment()
	exportLogic := logic.NewExportLogic(mockRepo, docserviceconfig.ExportConfig{OutputDir: exportDir}, logger)
	h := NewExportHandler(exportLogic)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/export-jobs/"+uuid.New().String(), nil)
	req = addTenantContext(req, uuid.New().String())
	req.SetPathValue("id", uuid.New().String())
	rr := httptest.NewRecorder()

	h.GetExportJobStatus(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestExportHandler_DownloadExport(t *testing.T) {
	jobID := uuid.New()
	exportDir := t.TempDir()
	testFile := filepath.Join(exportDir, "test.pdf")
	require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0644))

	mockRepo := &repo.MockRepository{
		GetExportJobByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.ExportJob, error) {
			return &model.ExportJob{
				ID:       id,
				Status:   model.ExportStatusCompleted,
				FilePath: &testFile,
				Format:   model.ExportFormatPDF,
			}, nil
		},
	}
	logger, _ := zap.NewDevelopment()
	exportLogic := logic.NewExportLogic(mockRepo, docserviceconfig.ExportConfig{OutputDir: exportDir}, logger)
	h := NewExportHandler(exportLogic)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/export-jobs/"+jobID.String()+"/download", nil)
	req = addTenantContext(req, uuid.New().String())
	req.SetPathValue("id", jobID.String())
	rr := httptest.NewRecorder()

	h.DownloadExport(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/pdf", rr.Header().Get("Content-Type"))
	assert.Contains(t, rr.Header().Get("Content-Disposition"), "test.pdf")
	assert.Equal(t, "test content", rr.Body.String())
}

func TestExportHandler_DownloadExport_InvalidID(t *testing.T) {
	mockRepo := &repo.MockRepository{}
	exportDir := t.TempDir()
	logger, _ := zap.NewDevelopment()
	exportLogic := logic.NewExportLogic(mockRepo, docserviceconfig.ExportConfig{OutputDir: exportDir}, logger)
	h := NewExportHandler(exportLogic)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/export-jobs/invalid/download", nil)
	req = addTenantContext(req, uuid.New().String())
	req.SetPathValue("id", "invalid")
	rr := httptest.NewRecorder()

	h.DownloadExport(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestExportHandler_DownloadExport_NotReady(t *testing.T) {
	jobID := uuid.New()
	mockRepo := &repo.MockRepository{
		GetExportJobByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.ExportJob, error) {
			return &model.ExportJob{
				ID:     id,
				Status: model.ExportStatusProcessing,
			}, nil
		},
	}
	exportDir := t.TempDir()
	logger, _ := zap.NewDevelopment()
	exportLogic := logic.NewExportLogic(mockRepo, docserviceconfig.ExportConfig{OutputDir: exportDir}, logger)
	h := NewExportHandler(exportLogic)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/export-jobs/"+jobID.String()+"/download", nil)
	req = addTenantContext(req, uuid.New().String())
	req.SetPathValue("id", jobID.String())
	rr := httptest.NewRecorder()

	h.DownloadExport(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}
