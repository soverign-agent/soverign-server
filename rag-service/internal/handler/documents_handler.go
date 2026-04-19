// Package handler provides HTTP handlers for rag-service.
package handler

import (
	"bytes"
	"io"
	"net/http"

	"sovereign-ai-compliance/rag-service/internal/logic"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// DocumentsHandler handles document-related HTTP requests.
type DocumentsHandler struct {
	logic *logic.DocumentsLogic
}

// NewDocumentsHandler creates a new DocumentsHandler.
func NewDocumentsHandler(logic *logic.DocumentsLogic) *DocumentsHandler {
	return &DocumentsHandler{
		logic: logic,
	}
}

// Upload handles document upload.
func (h *DocumentsHandler) Upload(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(10 << 20) // 10MB max
	if err != nil {
		httpx.Error(w, err)
		return
	}

	name := r.FormValue("name")
	description := r.FormValue("description")

	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		httpx.Error(w, err)
		return
	}
	defer file.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, file); err != nil {
		httpx.Error(w, err)
		return
	}

	req := logic.UploadDocumentRequest{
		Name:        name,
		Description: description,
		File:        fileHeader,
	}

	resp, err := h.logic.UploadDocument(r.Context(), req, buf.Bytes())
	if err != nil {
		httpx.Error(w, err)
		return
	}

	httpx.OkJson(w, resp)
}

// List lists documents with pagination.
func (h *DocumentsHandler) List(w http.ResponseWriter, r *http.Request) {
	var req logic.ListDocumentsRequest
	if err := httpx.Parse(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}

	resp, err := h.logic.ListDocuments(r.Context(), req)
	if err != nil {
		httpx.Error(w, err)
		return
	}

	httpx.OkJson(w, resp)
}

// Delete deletes a document.
func (h *DocumentsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Path[len("/api/v1/documents/"):]
	id, err := uuid.Parse(idStr)
	if err != nil {
		httpx.Error(w, err)
		return
	}

	req := logic.DeleteDocumentRequest{
		DocumentID: id,
	}

	if err := h.logic.DeleteDocument(r.Context(), req); err != nil {
		httpx.Error(w, err)
		return
	}

	httpx.OkJson(w, map[string]bool{"success": true})
}

// Reprocess reprocesses an existing document.
func (h *DocumentsHandler) Reprocess(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Path[len("/api/v1/documents/") : len(r.URL.Path)-len("/reprocess")]
	id, err := uuid.Parse(idStr)
	if err != nil {
		httpx.Error(w, err)
		return
	}

	// Read file content from request body
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r.Body); err != nil {
		httpx.Error(w, err)
		return
	}
	defer r.Body.Close()

	req := logic.ReprocessDocumentRequest{
		DocumentID:  id,
		FileContent: buf.Bytes(),
	}

	if err := h.logic.ReprocessDocument(r.Context(), req); err != nil {
		httpx.Error(w, err)
		return
	}

	httpx.OkJson(w, map[string]bool{"success": true})
}
