// Package handler provides REST handlers for the document service.
package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/rest/httpx"
	"sovereign-ai-compliance/doc-service/internal/logic"
	"sovereign-ai-compliance/doc-service/internal/types"
)

// DocumentHandler handles document HTTP requests.
type DocumentHandler struct {
	documentLogic *logic.DocumentLogic
	versionLogic  *logic.VersionLogic
}

// NewDocumentHandler creates a new DocumentHandler.
func NewDocumentHandler(documentLogic *logic.DocumentLogic, versionLogic *logic.VersionLogic) *DocumentHandler {
	return &DocumentHandler{
		documentLogic: documentLogic,
		versionLogic:  versionLogic,
	}
}

// ListDocuments lists all documents for the current tenant.
func (h *DocumentHandler) ListDocuments(w http.ResponseWriter, r *http.Request) {
	var req types.ListDocumentsRequest
	if err := httpx.Parse(r, &req); err != nil {
		httpx.ErrorCtx(r.Context(), w, err)
		return
	}

	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 20
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	resp, err := h.documentLogic.ListDocuments(r.Context(), req)
	if err != nil {
		httpx.ErrorCtx(r.Context(), w, err)
		return
	}

	httpx.OkJson(w, resp)
}

// GenerateDocument triggers document generation.
func (h *DocumentHandler) GenerateDocument(w http.ResponseWriter, r *http.Request) {
	var req types.GenerateDocumentRequest
	if err := httpx.Parse(r, &req); err != nil {
		httpx.ErrorCtx(r.Context(), w, err)
		return
	}

	if req.AISystemID == uuid.Nil {
		httpx.ErrorCtx(r.Context(), w, fmt.Errorf("ai_system_id is required"))
		return
	}
	if req.DocType == "" {
		httpx.ErrorCtx(r.Context(), w, fmt.Errorf("doc_type is required"))
		return
	}

	resp, err := h.documentLogic.GenerateDocument(r.Context(), req)
	if err != nil {
		httpx.ErrorCtx(r.Context(), w, err)
		return
	}

	httpx.OkJson(w, resp)
}

// GetDocument gets a specific document.
func (h *DocumentHandler) GetDocument(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	docID, err := uuid.Parse(idStr)
	if err != nil {
		httpx.ErrorCtx(r.Context(), w, fmt.Errorf("invalid document ID: %w", err))
		return
	}

	req := types.GetDocumentRequest{DocumentID: docID}
	resp, err := h.documentLogic.GetDocument(r.Context(), req)
	if err != nil {
		httpx.ErrorCtx(r.Context(), w, err)
		return
	}
	if resp == nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "document not found"})
		return
	}

	httpx.OkJson(w, resp)
}

// UpdateDocument updates document content.
func (h *DocumentHandler) UpdateDocument(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	docID, err := uuid.Parse(idStr)
	if err != nil {
		httpx.ErrorCtx(r.Context(), w, fmt.Errorf("invalid document ID: %w", err))
		return
	}

	var req types.UpdateDocumentRequest
	if err := httpx.Parse(r, &req); err != nil {
		httpx.ErrorCtx(r.Context(), w, err)
		return
	}
	req.DocumentID = docID

	resp, err := h.documentLogic.UpdateDocument(r.Context(), req)
	if err != nil {
		httpx.ErrorCtx(r.Context(), w, err)
		return
	}

	httpx.OkJson(w, resp)
}

// ListVersions lists version history for a document.
func (h *DocumentHandler) ListVersions(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	docID, err := uuid.Parse(idStr)
	if err != nil {
		httpx.ErrorCtx(r.Context(), w, fmt.Errorf("invalid document ID: %w", err))
		return
	}

	var req types.ListVersionsRequest
	if err := httpx.Parse(r, &req); err != nil {
		httpx.ErrorCtx(r.Context(), w, err)
		return
	}
	req.DocumentID = docID

	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 20
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	resp, err := h.versionLogic.ListVersions(r.Context(), req)
	if err != nil {
		httpx.ErrorCtx(r.Context(), w, err)
		return
	}

	httpx.OkJson(w, resp)
}

// RollbackVersion rolls back to a specific version.
func (h *DocumentHandler) RollbackVersion(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	docID, err := uuid.Parse(idStr)
	if err != nil {
		httpx.ErrorCtx(r.Context(), w, fmt.Errorf("invalid document ID: %w", err))
		return
	}

	var req types.RollbackVersionRequest
	if err := httpx.Parse(r, &req); err != nil {
		httpx.ErrorCtx(r.Context(), w, err)
		return
	}
	req.DocumentID = docID

	resp, err := h.versionLogic.RollbackToVersion(r.Context(), req)
	if err != nil {
		httpx.ErrorCtx(r.Context(), w, err)
		return
	}

	httpx.OkJson(w, resp)
}
