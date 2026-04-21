// Package handler provides REST handlers for the document service.
package handler

import (
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/rest/httpx"
	"sovereign-ai-compliance/doc-service/internal/logic"
	"sovereign-ai-compliance/doc-service/internal/types"
)

// ExportHandler handles export HTTP requests.
type ExportHandler struct {
	logic *logic.ExportLogic
}

// NewExportHandler creates a new ExportHandler.
func NewExportHandler(logic *logic.ExportLogic) *ExportHandler {
	return &ExportHandler{logic: logic}
}

// CreateExportJob creates a new export job.
func (h *ExportHandler) CreateExportJob(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	docID, err := uuid.Parse(idStr)
	if err != nil {
		httpx.ErrorCtx(r.Context(), w, fmt.Errorf("invalid document ID: %w", err))
		return
	}

	var req types.CreateExportJobRequest
	if err := httpx.Parse(r, &req); err != nil {
		httpx.ErrorCtx(r.Context(), w, err)
		return
	}
	req.DocumentID = docID

	resp, err := h.logic.CreateExportJob(r.Context(), req)
	if err != nil {
		httpx.ErrorCtx(r.Context(), w, err)
		return
	}

	httpx.OkJson(w, resp)
}

// GetExportJobStatus gets the status of an export job.
func (h *ExportHandler) GetExportJobStatus(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	jobID, err := uuid.Parse(idStr)
	if err != nil {
		httpx.ErrorCtx(r.Context(), w, fmt.Errorf("invalid job ID: %w", err))
		return
	}

	req := types.GetExportJobRequest{JobID: jobID}
	resp, err := h.logic.GetExportJob(r.Context(), req)
	if err != nil {
		httpx.ErrorCtx(r.Context(), w, err)
		return
	}

	httpx.OkJson(w, resp)
}

// DownloadExport serves the exported file.
func (h *ExportHandler) DownloadExport(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	jobID, err := uuid.Parse(idStr)
	if err != nil {
		httpx.ErrorCtx(r.Context(), w, fmt.Errorf("invalid job ID: %w", err))
		return
	}

	filePath, job, err := h.logic.DownloadExport(r.Context(), jobID)
	if err != nil {
		httpx.ErrorCtx(r.Context(), w, err)
		return
	}

	// Set content type based on format
	contentType := "application/octet-stream"
	if job.Format == "pdf" {
		contentType = "application/pdf"
	} else if job.Format == "docx" {
		contentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(filePath)))
	http.ServeFile(w, r, filePath)
}
