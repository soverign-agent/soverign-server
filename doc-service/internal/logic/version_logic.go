// Package logic provides business logic for the document service.
package logic

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"sovereign-ai-compliance/doc-service/internal/types"
	"sovereign-ai-compliance/doc-service/model"
	"sovereign-ai-compliance/doc-service/repo"
)

// VersionLogic handles document version management.
type VersionLogic struct {
	repo   repo.Repository
	logger *zap.Logger
}

// NewVersionLogic creates a new VersionLogic.
func NewVersionLogic(repo repo.Repository, logger *zap.Logger) *VersionLogic {
	return &VersionLogic{
		repo:   repo,
		logger: logger,
	}
}

// ListVersions retrieves version history for a document.
func (l *VersionLogic) ListVersions(ctx context.Context, req types.ListVersionsRequest) (*types.ListVersionsResponse, error) {
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 20
	}

	versions, total, err := l.repo.GetVersionsForDocument(ctx, req.DocumentID, req.Page, req.PageSize)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}

	pages := total / req.PageSize
	if total%req.PageSize > 0 {
		pages++
	}

	return &types.ListVersionsResponse{
		Items: versions,
		Total: total,
		Page:  req.Page,
		Pages: pages,
	}, nil
}

// RollbackToVersion restores a document to a specific version.
func (l *VersionLogic) RollbackToVersion(ctx context.Context, req types.RollbackVersionRequest) (*types.RollbackVersionResponse, error) {
	// Get the document
	doc, err := l.repo.GetDocumentByID(ctx, req.DocumentID)
	if err != nil {
		return nil, fmt.Errorf("get document: %w", err)
	}
	if doc == nil {
		return nil, fmt.Errorf("document not found")
	}

	// Get all versions to find the target
	versions, _, err := l.repo.GetVersionsForDocument(ctx, req.DocumentID, 1, 1000)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}

	var targetVersion *model.DocumentVersion
	for i := range versions {
		if versions[i].VersionNumber == req.VersionNumber {
			targetVersion = &versions[i]
			break
		}
	}

	if targetVersion == nil {
		return nil, fmt.Errorf("version %d not found", req.VersionNumber)
	}

	// Create a snapshot of current state before rollback
	currentVersion := &model.DocumentVersion{
		ID:            uuid.New(),
		TenantID:      doc.TenantID,
		DocumentID:    doc.ID,
		VersionNumber: doc.Version,
		Content:       doc.Content,
		CreatedBy:     doc.CreatedBy,
		ChangeSummary: fmt.Sprintf("Auto-snapshot before rollback to version %d", req.VersionNumber),
	}

	if err := l.repo.CreateVersion(ctx, currentVersion); err != nil {
		l.logger.Warn("failed to create snapshot before rollback", zap.Error(err))
		// Non-fatal: continue with rollback
	}

	// Apply rollback
	doc.Content = targetVersion.Content
	doc.Version++
	doc.Status = model.StatusEditing

	if err := l.repo.UpdateDocument(ctx, doc); err != nil {
		return nil, fmt.Errorf("update document after rollback: %w", err)
	}

	// Create a version record for the rollback itself
	rollbackVersion := &model.DocumentVersion{
		ID:            uuid.New(),
		TenantID:      doc.TenantID,
		DocumentID:    doc.ID,
		VersionNumber: doc.Version,
		Content:       targetVersion.Content,
		CreatedBy:     doc.CreatedBy,
		ChangeSummary: fmt.Sprintf("Rolled back to version %d", req.VersionNumber),
	}

	if err := l.repo.CreateVersion(ctx, rollbackVersion); err != nil {
		l.logger.Warn("failed to create rollback version record", zap.Error(err))
	}

	l.logger.Info("document rolled back",
		zap.String("document_id", doc.ID.String()),
		zap.Int("to_version", req.VersionNumber),
		zap.Int("new_version", doc.Version),
	)

	return &types.RollbackVersionResponse{GeneratedDocument: *doc}, nil
}
