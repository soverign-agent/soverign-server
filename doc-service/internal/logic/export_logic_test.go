package logic

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	docserviceconfig "sovereign-ai-compliance/doc-service/internal/config"
	"sovereign-ai-compliance/doc-service/internal/exporter"
	"sovereign-ai-compliance/doc-service/internal/types"
	"sovereign-ai-compliance/doc-service/model"
	"sovereign-ai-compliance/doc-service/repo"
	"sovereign-ai-compliance/shared/tenant"
)

func newTestExportLogic(mockRepo repo.Repository, export docserviceconfig.ExportConfig) *ExportLogic {
	logger, _ := zap.NewDevelopment()
	return NewExportLogic(mockRepo, export, logger)
}

func TestExportLogic_CreateExportJob(t *testing.T) {
	docID := uuid.New()
	tenantID := uuid.New()
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
	logic := newTestExportLogic(mockRepo, docserviceconfig.ExportConfig{
		OutputDir:     exportDir,
		DefaultFormat: "pdf",
	})
	ctx := tenant.WithContext(context.Background(), tenantID.String())

	resp, err := logic.CreateExportJob(ctx, types.CreateExportJobRequest{
		DocumentID: docID,
		Format:     "pdf",
	})
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, resp.JobID)
	assert.Equal(t, model.ExportStatusPending, resp.Status)
}

func TestExportLogic_CreateExportJob_DefaultFormat(t *testing.T) {
	docID := uuid.New()
	mockRepo := &repo.MockRepository{
		GetDocumentByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.GeneratedDocument, error) {
			return &model.GeneratedDocument{ID: docID}, nil
		},
		CreateExportJobFunc: func(ctx context.Context, job *model.ExportJob) error {
			assert.Equal(t, model.ExportFormatPDF, job.Format)
			return nil
		},
	}

	exportDir := t.TempDir()
	logic := newTestExportLogic(mockRepo, docserviceconfig.ExportConfig{
		OutputDir:     exportDir,
		DefaultFormat: "pdf",
	})
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	_, err := logic.CreateExportJob(ctx, types.CreateExportJobRequest{
		DocumentID: docID,
		Format:     "",
	})
	require.NoError(t, err)
}

func TestExportLogic_CreateExportJob_MissingTenant(t *testing.T) {
	mockRepo := &repo.MockRepository{}
	logic := newTestExportLogic(mockRepo, docserviceconfig.DefaultExportConfig())
	ctx := context.Background()

	_, err := logic.CreateExportJob(ctx, types.CreateExportJobRequest{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tenant context required")
}

func TestExportLogic_CreateExportJob_InvalidFormat(t *testing.T) {
	mockRepo := &repo.MockRepository{}
	logic := newTestExportLogic(mockRepo, docserviceconfig.DefaultExportConfig())
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	_, err := logic.CreateExportJob(ctx, types.CreateExportJobRequest{
		DocumentID: uuid.New(),
		Format:     "txt",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported export format")
}

func TestExportLogic_CreateExportJob_DocumentNotFound(t *testing.T) {
	mockRepo := &repo.MockRepository{
		GetDocumentByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.GeneratedDocument, error) {
			return nil, nil
		},
	}
	logic := newTestExportLogic(mockRepo, docserviceconfig.DefaultExportConfig())
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	_, err := logic.CreateExportJob(ctx, types.CreateExportJobRequest{
		DocumentID: uuid.New(),
		Format:     "pdf",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "document not found")
}

func TestExportLogic_CreateExportJob_RepoError(t *testing.T) {
	mockRepo := &repo.MockRepository{
		GetDocumentByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.GeneratedDocument, error) {
			return &model.GeneratedDocument{ID: id}, nil
		},
		CreateExportJobFunc: func(ctx context.Context, job *model.ExportJob) error {
			return errors.New("db error")
		},
	}
	logic := newTestExportLogic(mockRepo, docserviceconfig.DefaultExportConfig())
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	_, err := logic.CreateExportJob(ctx, types.CreateExportJobRequest{
		DocumentID: uuid.New(),
		Format:     "pdf",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "create export job")
}

func TestExportLogic_GetExportJob(t *testing.T) {
	jobID := uuid.New()
	mockRepo := &repo.MockRepository{
		GetExportJobByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.ExportJob, error) {
			return &model.ExportJob{
				ID:     jobID,
				Status: model.ExportStatusCompleted,
			}, nil
		},
	}
	logic := newTestExportLogic(mockRepo, docserviceconfig.DefaultExportConfig())
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	resp, err := logic.GetExportJob(ctx, types.GetExportJobRequest{JobID: jobID})
	require.NoError(t, err)
	assert.Equal(t, jobID, resp.ID)
	assert.Equal(t, model.ExportStatusCompleted, resp.Status)
}

func TestExportLogic_GetExportJob_NotFound(t *testing.T) {
	mockRepo := &repo.MockRepository{
		GetExportJobByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.ExportJob, error) {
			return nil, nil
		},
	}
	logic := newTestExportLogic(mockRepo, docserviceconfig.DefaultExportConfig())
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	_, err := logic.GetExportJob(ctx, types.GetExportJobRequest{JobID: uuid.New()})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "export job not found")
}

func TestExportLogic_GetExportJob_RepoError(t *testing.T) {
	mockRepo := &repo.MockRepository{
		GetExportJobByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.ExportJob, error) {
			return nil, errors.New("db error")
		},
	}
	logic := newTestExportLogic(mockRepo, docserviceconfig.DefaultExportConfig())
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	_, err := logic.GetExportJob(ctx, types.GetExportJobRequest{JobID: uuid.New()})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "get export job")
}

func TestExportLogic_DownloadExport(t *testing.T) {
	jobID := uuid.New()
	exportDir := t.TempDir()
	testFile := filepath.Join(exportDir, "test.pdf")
	require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0644))

	mockRepo := &repo.MockRepository{
		GetExportJobByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.ExportJob, error) {
			return &model.ExportJob{
				ID:       jobID,
				Status:   model.ExportStatusCompleted,
				FilePath: &testFile,
			}, nil
		},
	}
	logic := newTestExportLogic(mockRepo, docserviceconfig.ExportConfig{OutputDir: exportDir})
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	path, job, err := logic.DownloadExport(ctx, jobID)
	require.NoError(t, err)
	assert.Equal(t, testFile, path)
	assert.Equal(t, model.ExportStatusCompleted, job.Status)
}

func TestExportLogic_DownloadExport_NotReady(t *testing.T) {
	jobID := uuid.New()
	mockRepo := &repo.MockRepository{
		GetExportJobByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.ExportJob, error) {
			return &model.ExportJob{
				ID:     jobID,
				Status: model.ExportStatusProcessing,
			}, nil
		},
	}
	logic := newTestExportLogic(mockRepo, docserviceconfig.DefaultExportConfig())
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	_, job, err := logic.DownloadExport(ctx, jobID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "export not ready")
	assert.Equal(t, model.ExportStatusProcessing, job.Status)
}

func TestExportLogic_DownloadExport_NoFilePath(t *testing.T) {
	jobID := uuid.New()
	mockRepo := &repo.MockRepository{
		GetExportJobByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.ExportJob, error) {
			return &model.ExportJob{
				ID:     jobID,
				Status: model.ExportStatusCompleted,
			}, nil
		},
	}
	logic := newTestExportLogic(mockRepo, docserviceconfig.DefaultExportConfig())
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	_, job, err := logic.DownloadExport(ctx, jobID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "export file path not set")
	assert.Equal(t, model.ExportStatusCompleted, job.Status)
}

func TestExportLogic_DownloadExport_FileNotFound(t *testing.T) {
	jobID := uuid.New()
	missingPath := "/nonexistent/path/test.pdf"
	mockRepo := &repo.MockRepository{
		GetExportJobByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.ExportJob, error) {
			return &model.ExportJob{
				ID:       jobID,
				Status:   model.ExportStatusCompleted,
				FilePath: &missingPath,
			}, nil
		},
	}
	logic := newTestExportLogic(mockRepo, docserviceconfig.DefaultExportConfig())
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	_, job, err := logic.DownloadExport(ctx, jobID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "export file not found")
	assert.Equal(t, model.ExportStatusCompleted, job.Status)
}

func TestExportLogic_DownloadExport_JobNotFound(t *testing.T) {
	mockRepo := &repo.MockRepository{
		GetExportJobByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.ExportJob, error) {
			return nil, nil
		},
	}
	logic := newTestExportLogic(mockRepo, docserviceconfig.DefaultExportConfig())
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	_, _, err := logic.DownloadExport(ctx, uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "export job not found")
}

func TestExportLogic_runExport(t *testing.T) {
	tenantID := uuid.New()
	jobID := uuid.New()
	docID := uuid.New()

	exportDir := t.TempDir()

	job := &model.ExportJob{
		ID:         jobID,
		TenantID:   tenantID,
		DocumentID: docID,
		Format:     model.ExportFormatPDF,
	}

	doc := &model.GeneratedDocument{
		ID:      docID,
		Title:   "Test Document",
		Version: 1,
		Content: model.DocumentContent{
			Sections: []model.Section{{ID: "s1", Title: "Section", Content: "Content"}},
		},
	}

	statusChanges := []string{}
	mockRepo := &repo.MockRepository{
		UpdateExportJobStatusFunc: func(ctx context.Context, id uuid.UUID, status string, filePath *string, fileSize *int64, errorMessage *string) error {
			statusChanges = append(statusChanges, status)
			return nil
		},
	}

	logger, _ := zap.NewDevelopment()
	logic := &ExportLogic{
		repo:   mockRepo,
		export: docserviceconfig.ExportConfig{OutputDir: exportDir, DefaultFormat: "pdf"},
		logger: logger,
		pdfExp: exporter.NewPDFExporter(),
	}

	logic.runExport(job, doc)

	assert.Contains(t, statusChanges, model.ExportStatusProcessing)
	assert.Contains(t, statusChanges, model.ExportStatusCompleted)
}

func TestExportLogic_runExport_MkdirError(t *testing.T) {
	tenantID := uuid.New()
	jobID := uuid.New()
	docID := uuid.New()

	job := &model.ExportJob{
		ID:         jobID,
		TenantID:   tenantID,
		DocumentID: docID,
		Format:     model.ExportFormatPDF,
	}

	doc := &model.GeneratedDocument{
		ID:      docID,
		Title:   "Test",
		Version: 1,
		Content: model.DocumentContent{},
	}

	var failedJobID uuid.UUID
	mockRepo := &repo.MockRepository{
		UpdateExportJobStatusFunc: func(ctx context.Context, id uuid.UUID, status string, filePath *string, fileSize *int64, errorMessage *string) error {
			if status == model.ExportStatusFailed {
				failedJobID = id
			}
			return nil
		},
	}

	logger, _ := zap.NewDevelopment()
	logic := &ExportLogic{
		repo:   mockRepo,
		export: docserviceconfig.ExportConfig{OutputDir: "/dev/null/invalid/subdir"},
		logger: logger,
		pdfExp: exporter.NewPDFExporter(),
	}

	logic.runExport(job, doc)

	assert.Equal(t, jobID, failedJobID)
}

func TestExportLogic_failExport_Success(t *testing.T) {
	jobID := uuid.New()
	mockRepo := &repo.MockRepository{
		UpdateExportJobStatusFunc: func(ctx context.Context, id uuid.UUID, status string, filePath *string, fileSize *int64, errorMessage *string) error {
			assert.Equal(t, model.ExportStatusFailed, status)
			assert.NotNil(t, errorMessage)
			assert.Contains(t, *errorMessage, "test reason")
			return nil
		},
	}

	logger, _ := zap.NewDevelopment()
	logic := &ExportLogic{repo: mockRepo, logger: logger}
	ctx := tenant.WithContext(context.Background(), uuid.New().String())

	logic.failExport(ctx, jobID, "test reason")
}

func TestNewExportLogic_EmptyOutputDir(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	logic := NewExportLogic(&repo.MockRepository{}, docserviceconfig.ExportConfig{}, logger)
	assert.Equal(t, "./exports", logic.export.OutputDir)
}
