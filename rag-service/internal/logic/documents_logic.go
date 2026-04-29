// Package logic contains the business logic for rag-service.
package logic

import (
	"context"
	"errors"
	"mime/multipart"
	"strings"

	"sovereign-ai-compliance/rag-service/internal/config"
	"sovereign-ai-compliance/rag-service/model"
	"sovereign-ai-compliance/rag-service/processing"
	"sovereign-ai-compliance/rag-service/repo"
	"sovereign-ai-compliance/shared/llm"
	"sovereign-ai-compliance/shared/tenant"

	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"
	"go.uber.org/zap"
)

// DocumentsLogic handles document management business logic.
type DocumentsLogic struct {
	config    config.Config
	repo      *repo.SQLRepository
	processor *processing.Processor
	llmClient llm.Client
	logger    *zap.Logger
}

// NewDocumentsLogic creates a new DocumentsLogic.
func NewDocumentsLogic(
	cfg config.Config,
	repo *repo.SQLRepository,
	processor *processing.Processor,
	llmClient llm.Client,
	logger *zap.Logger,
) *DocumentsLogic {
	return &DocumentsLogic{
		config:    cfg,
		repo:      repo,
		processor: processor,
		llmClient: llmClient,
		logger:    logger,
	}
}

// UploadDocumentRequest is the request for uploading a document.
type UploadDocumentRequest struct {
	Name        string                `form:"name"`
	Description string                `form:"description"`
	File        *multipart.FileHeader `form:"file"`
}

// UploadDocumentResponse is the response for uploading a document.
type UploadDocumentResponse struct {
	DocumentID uuid.UUID `json:"document_id"`
	Status     string    `json:"status"`
}

// UploadDocument handles document upload and queues it for processing.
// Processing happens synchronously in-line for now.
func (l *DocumentsLogic) UploadDocument(ctx context.Context, req UploadDocumentRequest, fileContent []byte) (*UploadDocumentResponse, error) {
	tenantIDStr, ok := tenant.FromContext(ctx)
	if !ok {
		return nil, ErrTenantContextRequired
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		return nil, err
	}

	fileExt := strings.ToLower(strings.TrimPrefix(req.File.Filename, strings.TrimSuffix(req.File.Filename, ".")))
	if fileExt == req.File.Filename {
		fileExt = "txt"
	}

	doc := &model.Document{
		ID:          uuid.New(),
		TenantID:    tenantID,
		Name:        req.Name,
		Description: req.Description,
		FileType:    fileExt,
		FileSize:    req.File.Size,
		Status:      model.DocumentStatusProcessing,
	}

	if err := l.repo.CreateDocument(ctx, doc); err != nil {
		return nil, err
	}

	l.logger.Info("document uploaded",
		zap.String("document_id", doc.ID.String()),
		zap.String("tenant_id", tenantIDStr),
		zap.Int64("file_size", doc.FileSize))

	// Process document asynchronously in a goroutine.
	// Use a background context because the request context will be cancelled
	// once the HTTP response is sent.
	bgCtx := tenant.WithContext(context.Background(), tenantIDStr)
	go l.processDocumentAsync(bgCtx, doc.ID, fileContent, fileExt)

	return &UploadDocumentResponse{
		DocumentID: doc.ID,
		Status:     doc.Status,
	}, nil
}

func (l *DocumentsLogic) processDocumentAsync(ctx context.Context, documentID uuid.UUID, content []byte, fileType string) {
	doc, err := l.repo.GetDocumentByID(ctx, documentID)
	if err != nil {
		l.logger.Error("failed to get document for processing",
			zap.String("document_id", documentID.String()),
			zap.Error(err))
		return
	}
	if doc == nil {
		l.logger.Error("document not found for processing",
			zap.String("document_id", documentID.String()))
		return
	}

	// Process the document
	chunks, err := l.processor.Process(content, fileType)
	if err != nil {
		errorMsg := err.Error()
		_ = l.repo.UpdateDocumentStatus(ctx, documentID, model.DocumentStatusFailed, &errorMsg)
		l.logger.Error("failed to process document",
			zap.String("document_id", documentID.String()),
			zap.Error(err))
		return
	}

	// Report progress after text extraction (first 10%)
	if err := l.repo.UpdateDocumentProgress(ctx, documentID, 10); err != nil {
		l.logger.Warn("failed to update document progress after extraction",
			zap.String("document_id", documentID.String()),
			zap.Error(err))
	}

	// Generate embeddings for each chunk
	tenantIDStr, _ := tenant.FromContext(ctx)
	tenantID, _ := uuid.Parse(tenantIDStr)

	totalChunks := len(chunks)
	if totalChunks == 0 {
		_ = l.repo.UpdateDocumentProgress(ctx, documentID, 100)
		errMsg := ""
		_ = l.repo.UpdateDocumentStatus(ctx, documentID, model.DocumentStatusCompleted, &errMsg)
		l.logger.Info("document processing completed (no chunks)",
			zap.String("document_id", documentID.String()))
		return
	}

	for i, chunk := range chunks {
		// Report progress after text extraction (first 10%) and per-chunk embedding (remaining 90%)
		progress := 10 + int(float64(i+1)/float64(totalChunks)*90)
		if err := l.repo.UpdateDocumentProgress(ctx, documentID, progress); err != nil {
			l.logger.Warn("failed to update document progress",
				zap.String("document_id", documentID.String()),
				zap.Error(err))
		}

		// Check for duplicate
		exists, err := l.repo.ExistsChecksum(ctx, chunk.Checksum)
		if err != nil {
			l.logger.Warn("failed to check duplicate chunk",
				zap.String("document_id", documentID.String()),
				zap.Error(err))
			continue
		}
		if exists {
			continue
		}

		// Generate embedding
		embResp, err := l.llmClient.Embed(ctx, llm.EmbeddingRequest{
			Model: l.config.LLM.EmbeddingModel,
			Input: chunk.Text,
		})
		if err != nil {
			l.logger.Warn("failed to generate embedding",
				zap.String("document_id", documentID.String()),
				zap.Error(err))
			continue
		}

		// Convert to pgvector
		embedding := pgvector.NewVector(embResp.Embedding)

		emb := model.Embedding{
			ID:         uuid.New(),
			TenantID:   tenantID,
			DocumentID: documentID,
			ChunkIndex: chunk.Index,
			Text:       chunk.Text,
			Embedding:  embedding,
			Checksum:   chunk.Checksum,
		}

		if err := l.repo.InsertEmbedding(ctx, &emb); err != nil {
			l.logger.Warn("failed to insert embedding",
				zap.String("document_id", documentID.String()),
				zap.Error(err))
		}
	}

	// Mark as completed
	_ = l.repo.UpdateDocumentProgress(ctx, documentID, 100)
	errMsg := ""
	_ = l.repo.UpdateDocumentStatus(ctx, documentID, model.DocumentStatusCompleted, &errMsg)
	l.logger.Info("document processing completed",
		zap.String("document_id", documentID.String()),
		zap.Int("chunks", len(chunks)))
}

// ListDocumentsRequest lists documents with pagination.
type ListDocumentsRequest struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

// ListDocumentsResponse lists documents with pagination.
type ListDocumentsResponse struct {
	Documents []model.Document `json:"documents"`
	Total     int              `json:"total"`
	Page      int              `json:"page"`
	PageSize  int              `json:"page_size"`
}

// ListDocuments lists documents for the current tenant.
func (l *DocumentsLogic) ListDocuments(ctx context.Context, req ListDocumentsRequest) (*ListDocumentsResponse, error) {
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > 100 {
		req.PageSize = 10
	}

	documents, total, err := l.repo.ListDocuments(ctx, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}

	return &ListDocumentsResponse{
		Documents: documents,
		Total:     total,
		Page:      req.Page,
		PageSize:  req.PageSize,
	}, nil
}

// DeleteDocumentRequest deletes a document.
type DeleteDocumentRequest struct {
	DocumentID uuid.UUID `json:"document_id"`
}

// DeleteDocument deletes a document and all its embeddings.
func (l *DocumentsLogic) DeleteDocument(ctx context.Context, req DeleteDocumentRequest) error {
	_, ok := tenant.FromContext(ctx)
	if !ok {
		return ErrTenantContextRequired
	}

	return l.repo.DeleteDocument(ctx, req.DocumentID)
}

// ReprocessDocumentRequest reprocesses a document.
type ReprocessDocumentRequest struct {
	DocumentID  uuid.UUID `json:"document_id"`
	FileContent []byte    `json:"-"` // File content if re-uploaded
	FileType    string    `json:"-"`
}

// ReprocessDocument reprocesses an existing document.
func (l *DocumentsLogic) ReprocessDocument(ctx context.Context, req ReprocessDocumentRequest) error {
	doc, err := l.repo.GetDocumentByID(ctx, req.DocumentID)
	if err != nil {
		return err
	}
	if doc == nil {
		return ErrDocumentNotFound
	}

	// Delete existing embeddings
	if err := l.repo.DeleteEmbeddingsByDocument(ctx, req.DocumentID); err != nil {
		l.logger.Warn("failed to delete old embeddings",
			zap.String("document_id", req.DocumentID.String()),
			zap.Error(err))
	}

	// Update status to processing
	if err := l.repo.UpdateDocumentStatus(ctx, req.DocumentID, model.DocumentStatusProcessing, nil); err != nil {
		return err
	}

	// Process async with a background context because the request context
	// will be cancelled once the HTTP response is sent.
	bgCtx := tenant.WithContext(context.Background(), tenant.MustFromContext(ctx))
	go l.processDocumentAsync(bgCtx, req.DocumentID, req.FileContent, doc.FileType)

	return nil
}

// Errors
var (
	ErrTenantContextRequired = errors.New("tenant context required")
	ErrDocumentNotFound      = errors.New("document not found")
)
