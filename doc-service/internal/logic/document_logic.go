// Package logic provides business logic for the document service.
package logic

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
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

// DocumentLogic handles document generation and management.
type DocumentLogic struct {
	repo      repo.Repository
	llmCfg    sharedconfig.LLMConfig
	export    docserviceconfig.ExportConfig
	logger    *zap.Logger
	gen       *generator.DocumentGenerator
	assembler *generator.DataAssembler
}

// NewDocumentLogic creates a new DocumentLogic.
func NewDocumentLogic(repo repo.Repository, llmCfg sharedconfig.LLMConfig, export docserviceconfig.ExportConfig, logger *zap.Logger, clients *generator.DownstreamClients) *DocumentLogic {
	// Create LLM client for document generation
	llmClient, err := llm.NewClient(llm.Config{
		Provider:    llm.Provider(llmCfg.Provider),
		APIKey:      llmCfg.APIKey,
		BaseURL:     llmCfg.BaseURL,
		Model:       llmCfg.Model,
		Timeout:     int(llmCfg.Timeout.Seconds()),
		MaxTokens:   llmCfg.MaxTokens,
		Temperature: llmCfg.Temperature,
	}, logger)
	if err != nil {
		logger.Warn("failed to create LLM client, document generation will fail", zap.Error(err))
		llmClient = nil
	}

	return &DocumentLogic{
		repo:      repo,
		llmCfg:    llmCfg,
		export:    export,
		logger:    logger,
		gen:       generator.NewDocumentGenerator(llmClient, logger),
		assembler: generator.NewDataAssembler(clients),
	}
}

// GenerateDocument creates a new document and triggers async generation.
func (l *DocumentLogic) GenerateDocument(ctx context.Context, req types.GenerateDocumentRequest) (*types.GenerateDocumentResponse, error) {
	tenantID, ok := tenant.FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("tenant context required")
	}

	tenantUUID, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, fmt.Errorf("invalid tenant ID: %w", err)
	}

	// Validate doc type
	if req.DocType != model.DocTypeAnnexIV && req.DocType != model.DocTypeRiskReport && req.DocType != model.DocTypeCompliance {
		return nil, fmt.Errorf("unsupported document type: %s", req.DocType)
	}

	doc := &model.GeneratedDocument{
		ID:         uuid.New(),
		TenantID:   tenantUUID,
		AISystemID: req.AISystemID,
		DocType:    req.DocType,
		Title:      req.Title,
		Content: model.DocumentContent{
			Sections: []model.Section{},
			Metadata: map[string]string{},
		},
		Version:   1,
		Status:    model.StatusGenerating,
		CreatedBy: tenantUUID, // TODO: Use actual user ID from auth context
	}

	if doc.Title == "" {
		doc.Title = fmt.Sprintf("%s Documentation - %s", req.DocType, req.AISystemID.String())
	}

	if err := l.repo.CreateDocument(ctx, doc); err != nil {
		return nil, fmt.Errorf("create document: %w", err)
	}

	// Kick off async generation
	go l.runGeneration(doc, req.AuditJobID)

	return &types.GenerateDocumentResponse{
		DocumentID: doc.ID,
		Status:     doc.Status,
	}, nil
}

// runGeneration performs document generation asynchronously.
func (l *DocumentLogic) runGeneration(doc *model.GeneratedDocument, auditJobID *uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Propagate tenant context for RLS
	ctx = tenant.WithContext(ctx, doc.TenantID.String())

	l.logger.Info("starting document generation",
		zap.String("document_id", doc.ID.String()),
		zap.String("doc_type", doc.DocType),
	)

	// Assemble data from external services
	var auditJobIDStr *string
	if auditJobID != nil {
		s := auditJobID.String()
		auditJobIDStr = &s
	}

	data, err := l.assembler.Assemble(ctx, doc.AISystemID.String(), auditJobIDStr)
	if err != nil {
		l.logger.Error("data assembly failed", zap.Error(err))
		l.failGeneration(ctx, doc.ID, fmt.Sprintf("data assembly: %v", err))
		return
	}

	// Generate document content
	content, err := l.gen.Generate(ctx, doc, data, generator.GenerateOptions{})
	if err != nil {
		l.logger.Error("document generation failed", zap.Error(err))
		l.failGeneration(ctx, doc.ID, fmt.Sprintf("generation: %v", err))
		return
	}

	// Validate generated content
	if err := l.gen.ValidateContent(content, doc.DocType); err != nil {
		l.logger.Error("content validation failed", zap.Error(err))
		l.failGeneration(ctx, doc.ID, fmt.Sprintf("validation: %v", err))
		return
	}

	// Update document with generated content and set status to editing
	doc.Content = *content
	doc.Status = model.StatusEditing

	if err := l.repo.UpdateDocument(ctx, doc); err != nil {
		l.logger.Error("failed to update document after generation", zap.Error(err))
		l.failGeneration(ctx, doc.ID, fmt.Sprintf("update: %v", err))
		return
	}

	// Create initial version snapshot
	version := &model.DocumentVersion{
		ID:            uuid.New(),
		TenantID:      doc.TenantID,
		DocumentID:    doc.ID,
		VersionNumber: doc.Version,
		Content:       *content,
		CreatedBy:     doc.CreatedBy,
		ChangeSummary: "Initial auto-generated version",
	}

	if err := l.repo.CreateVersion(ctx, version); err != nil {
		l.logger.Warn("failed to create initial version", zap.Error(err))
		// Non-fatal: document is still usable
	}

	l.logger.Info("document generation completed",
		zap.String("document_id", doc.ID.String()),
		zap.Int("sections", len(content.Sections)),
	)
}

// failGeneration updates the document status to failed with an error context.
func (l *DocumentLogic) failGeneration(ctx context.Context, docID uuid.UUID, reason string) {
	if err := l.repo.UpdateDocumentStatus(ctx, docID, model.StatusFailed); err != nil {
		l.logger.Error("failed to set document status to failed", zap.Error(err))
	}
	l.logger.Warn("document generation failed",
		zap.String("document_id", docID.String()),
		zap.String("reason", reason),
	)
}

// GetDocument retrieves a document by ID.
func (l *DocumentLogic) GetDocument(ctx context.Context, req types.GetDocumentRequest) (*types.GetDocumentResponse, error) {
	doc, err := l.repo.GetDocumentByID(ctx, req.DocumentID)
	if err != nil {
		return nil, fmt.Errorf("get document: %w", err)
	}
	if doc == nil {
		return nil, nil
	}

	return &types.GetDocumentResponse{GeneratedDocument: *doc}, nil
}

// ListDocuments lists documents with filtering and pagination.
func (l *DocumentLogic) ListDocuments(ctx context.Context, req types.ListDocumentsRequest) (*types.ListDocumentsResponse, error) {
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 20
	}

	summaries, total, err := l.repo.ListDocuments(ctx, req.AISystemID, req.DocType, req.Status, req.Page, req.PageSize)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}

	pages := total / req.PageSize
	if total%req.PageSize > 0 {
		pages++
	}

	return &types.ListDocumentsResponse{
		Items: summaries,
		Total: total,
		Page:  req.Page,
		Pages: pages,
	}, nil
}

// UpdateDocument updates document content (human-in-the-loop editing).
func (l *DocumentLogic) UpdateDocument(ctx context.Context, req types.UpdateDocumentRequest) (*types.UpdateDocumentResponse, error) {
	doc, err := l.repo.GetDocumentByID(ctx, req.DocumentID)
	if err != nil {
		return nil, fmt.Errorf("get document: %w", err)
	}
	if doc == nil {
		return nil, fmt.Errorf("document not found")
	}

	// Only allow editing documents that are in editing or failed status
	if doc.Status != model.StatusEditing && doc.Status != model.StatusFailed {
		return nil, fmt.Errorf("cannot edit document with status: %s", doc.Status)
	}

	// Create a version snapshot before updating
	version := &model.DocumentVersion{
		ID:            uuid.New(),
		TenantID:      doc.TenantID,
		DocumentID:    doc.ID,
		VersionNumber: doc.Version,
		Content:       doc.Content,
		CreatedBy:     req.UpdatedBy,
		ChangeSummary: "Manual edit before update",
	}

	if err := l.repo.CreateVersion(ctx, version); err != nil {
		l.logger.Warn("failed to create version before update", zap.Error(err))
		// Non-fatal: continue with update
	}

	// Apply update
	doc.Content = req.Content
	doc.Status = model.StatusEditing
	doc.Version++

	if err := l.repo.UpdateDocument(ctx, doc); err != nil {
		return nil, fmt.Errorf("update document: %w", err)
	}

	return &types.UpdateDocumentResponse{GeneratedDocument: *doc}, nil
}

// UpdateDocumentStatus updates the status of a document.
func (l *DocumentLogic) UpdateDocumentStatus(ctx context.Context, req types.UpdateDocumentStatusRequest) (*types.UpdateDocumentStatusResponse, error) {
	validStatuses := map[string]bool{
		model.StatusGenerating: true,
		model.StatusEditing:    true,
		model.StatusApproved:   true,
		model.StatusPublished:  true,
		model.StatusFailed:     true,
	}

	if !validStatuses[req.Status] {
		return nil, fmt.Errorf("invalid status: %s", req.Status)
	}

	doc, err := l.repo.GetDocumentByID(ctx, req.DocumentID)
	if err != nil {
		return nil, fmt.Errorf("get document: %w", err)
	}
	if doc == nil {
		return nil, fmt.Errorf("document not found")
	}

	// Validate status transitions
	if !isValidStatusTransition(doc.Status, req.Status) {
		return nil, fmt.Errorf("invalid status transition from %s to %s", doc.Status, req.Status)
	}

	if err := l.repo.UpdateDocumentStatus(ctx, req.DocumentID, req.Status); err != nil {
		return nil, fmt.Errorf("update status: %w", err)
	}

	return &types.UpdateDocumentStatusResponse{
		Success: true,
		Status:  req.Status,
	}, nil
}

// DeleteDocument deletes a document by ID.
func (l *DocumentLogic) DeleteDocument(ctx context.Context, req types.DeleteDocumentRequest) (*types.DeleteDocumentResponse, error) {
	doc, err := l.repo.GetDocumentByID(ctx, req.DocumentID)
	if err != nil {
		return nil, fmt.Errorf("get document: %w", err)
	}
	if doc == nil {
		return nil, fmt.Errorf("document not found")
	}

	if err := l.repo.DeleteDocument(ctx, req.DocumentID); err != nil {
		return nil, fmt.Errorf("delete document: %w", err)
	}

	return &types.DeleteDocumentResponse{
		Success: true,
	}, nil
}

// isValidStatusTransition checks if a status transition is allowed.
func isValidStatusTransition(from, to string) bool {
	// Define allowed transitions
	transitions := map[string][]string{
		model.StatusGenerating: {model.StatusEditing, model.StatusFailed},
		model.StatusEditing:    {model.StatusApproved, model.StatusFailed, model.StatusGenerating},
		model.StatusApproved:   {model.StatusPublished, model.StatusEditing},
		model.StatusPublished:  {model.StatusEditing},
		model.StatusFailed:     {model.StatusGenerating, model.StatusEditing},
	}

	allowed, ok := transitions[from]
	if !ok {
		return false
	}

	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}
