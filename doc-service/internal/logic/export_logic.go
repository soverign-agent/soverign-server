// Package logic provides business logic for the document service.
package logic

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	docserviceconfig "sovereign-ai-compliance/doc-service/internal/config"
	"sovereign-ai-compliance/doc-service/internal/exporter"
	"sovereign-ai-compliance/doc-service/internal/types"
	"sovereign-ai-compliance/doc-service/model"
	"sovereign-ai-compliance/doc-service/repo"
	"sovereign-ai-compliance/shared/tenant"
)

// ExportLogic handles document export operations.
type ExportLogic struct {
	repo    repo.Repository
	export  docserviceconfig.ExportConfig
	logger  *zap.Logger
	pdfExp  *exporter.PDFExporter
	docxExp *exporter.DOCXExporter
}

// NewExportLogic creates a new ExportLogic.
func NewExportLogic(repo repo.Repository, export docserviceconfig.ExportConfig, logger *zap.Logger) *ExportLogic {
	// Ensure output directory exists
	if export.OutputDir == "" {
		export.OutputDir = "./exports"
	}
	_ = os.MkdirAll(export.OutputDir, 0755)

	return &ExportLogic{
		repo:    repo,
		export:  export,
		logger:  logger,
		pdfExp:  exporter.NewPDFExporter(),
		docxExp: exporter.NewDOCXExporter(),
	}
}

// CreateExportJob creates a new export job and triggers async processing.
func (l *ExportLogic) CreateExportJob(ctx context.Context, req types.CreateExportJobRequest) (*types.CreateExportJobResponse, error) {
	tenantID, ok := tenant.FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("tenant context required")
	}

	tenantUUID, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, fmt.Errorf("invalid tenant ID: %w", err)
	}

	// Validate format
	format := req.Format
	if format == "" {
		format = l.export.DefaultFormat
	}
	if format != model.ExportFormatPDF && format != model.ExportFormatDOCX {
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}

	// Verify document exists
	doc, err := l.repo.GetDocumentByID(ctx, req.DocumentID)
	if err != nil {
		return nil, fmt.Errorf("get document: %w", err)
	}
	if doc == nil {
		return nil, fmt.Errorf("document not found")
	}

	job := &model.ExportJob{
		ID:         uuid.New(),
		TenantID:   tenantUUID,
		DocumentID: req.DocumentID,
		Format:     format,
		Status:     model.ExportStatusPending,
		CreatedBy:  tenantUUID,
	}

	if err := l.repo.CreateExportJob(ctx, job); err != nil {
		return nil, fmt.Errorf("create export job: %w", err)
	}

	// Kick off async export
	go l.runExport(job, doc)

	return &types.CreateExportJobResponse{
		JobID:  job.ID,
		Status: job.Status,
	}, nil
}

// GetExportJob retrieves an export job by ID.
func (l *ExportLogic) GetExportJob(ctx context.Context, req types.GetExportJobRequest) (*types.GetExportJobResponse, error) {
	job, err := l.repo.GetExportJobByID(ctx, req.JobID)
	if err != nil {
		return nil, fmt.Errorf("get export job: %w", err)
	}
	if job == nil {
		return nil, fmt.Errorf("export job not found")
	}

	return &types.GetExportJobResponse{ExportJob: *job}, nil
}

// DownloadExport validates tenant access and returns the file path for download.
func (l *ExportLogic) DownloadExport(ctx context.Context, jobID uuid.UUID) (string, *model.ExportJob, error) {
	job, err := l.repo.GetExportJobByID(ctx, jobID)
	if err != nil {
		return "", nil, fmt.Errorf("get export job: %w", err)
	}
	if job == nil {
		return "", nil, fmt.Errorf("export job not found")
	}

	if job.Status != model.ExportStatusCompleted {
		return "", job, fmt.Errorf("export not ready: status is %s", job.Status)
	}

	if job.FilePath == nil || *job.FilePath == "" {
		return "", job, fmt.Errorf("export file path not set")
	}

	// Verify file exists
	if _, err := os.Stat(*job.FilePath); err != nil {
		return "", job, fmt.Errorf("export file not found: %w", err)
	}

	return *job.FilePath, job, nil
}

// runExport performs the actual export in a background goroutine.
func (l *ExportLogic) runExport(job *model.ExportJob, doc *model.GeneratedDocument) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Propagate tenant context for RLS
	ctx = tenant.WithContext(ctx, job.TenantID.String())

	l.logger.Info("starting export",
		zap.String("job_id", job.ID.String()),
		zap.String("document_id", doc.ID.String()),
		zap.String("format", job.Format),
	)

	// Update status to processing
	if err := l.repo.UpdateExportJobStatus(ctx, job.ID, model.ExportStatusProcessing, nil, nil, nil); err != nil {
		l.logger.Error("failed to update export status to processing", zap.Error(err))
	}

	// Build output path
	ext := job.Format
	filename := fmt.Sprintf("%s_v%d_%s.%s", doc.ID.String(), doc.Version, job.ID.String(), ext)
	outputPath := filepath.Join(l.export.OutputDir, filename)

	// Ensure output directory exists
	if err := os.MkdirAll(l.export.OutputDir, 0755); err != nil {
		l.failExport(ctx, job.ID, fmt.Sprintf("create output dir: %v", err))
		return
	}

	// Perform export based on format
	var err error
	switch job.Format {
	case model.ExportFormatPDF:
		err = l.pdfExp.Export(doc, outputPath)
	case model.ExportFormatDOCX:
		err = l.docxExp.Export(doc, outputPath)
	default:
		err = fmt.Errorf("unsupported format: %s", job.Format)
	}

	if err != nil {
		l.failExport(ctx, job.ID, fmt.Sprintf("export: %v", err))
		return
	}

	// Get file size
	info, err := os.Stat(outputPath)
	if err != nil {
		l.failExport(ctx, job.ID, fmt.Sprintf("stat output file: %v", err))
		return
	}

	size := info.Size()
	if l.export.MaxFileSizeMB > 0 && size > int64(l.export.MaxFileSizeMB)*1024*1024 {
		_ = os.Remove(outputPath)
		l.failExport(ctx, job.ID, "export file exceeds maximum size")
		return
	}

	// Update status to completed
	if err := l.repo.UpdateExportJobStatus(ctx, job.ID, model.ExportStatusCompleted, &outputPath, &size, nil); err != nil {
		l.logger.Error("failed to update export status to completed", zap.Error(err))
	}

	l.logger.Info("export completed",
		zap.String("job_id", job.ID.String()),
		zap.String("path", outputPath),
		zap.Int64("size", size),
	)
}

// failExport updates the export job status to failed.
func (l *ExportLogic) failExport(ctx context.Context, jobID uuid.UUID, reason string) {
	if err := l.repo.UpdateExportJobStatus(ctx, jobID, model.ExportStatusFailed, nil, nil, &reason); err != nil {
		l.logger.Error("failed to update export status to failed", zap.Error(err))
	}
	l.logger.Warn("export failed",
		zap.String("job_id", jobID.String()),
		zap.String("reason", reason),
	)
}
