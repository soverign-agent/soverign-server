// Package logic contains the business logic for rag-service.
package logic

import (
	"context"

	"sovereign-ai-compliance/rag-service/repo"
	"sovereign-ai-compliance/shared/tenant"

	"go.uber.org/zap"
)

// StatsLogic handles RAG statistics business logic.
type StatsLogic struct {
	repo   *repo.SQLRepository
	logger *zap.Logger
}

// NewStatsLogic creates a new StatsLogic.
func NewStatsLogic(repo *repo.SQLRepository, logger *zap.Logger) *StatsLogic {
	return &StatsLogic{
		repo:   repo,
		logger: logger,
	}
}

// GetStatsResponse is the statistics response.
type GetStatsResponse struct {
	DocumentCount      int   `json:"document_count"`
	CompletedDocuments int   `json:"completed_documents"`
	PendingDocuments   int   `json:"pending_documents"`
	EmbeddingCount     int   `json:"embedding_count"`
	TotalFileSizeBytes int64 `json:"total_file_size_bytes"`
}

// GetStats gets RAG statistics for the current tenant.
func (l *StatsLogic) GetStats(ctx context.Context) (*GetStatsResponse, error) {
	_, ok := tenant.FromContext(ctx)
	if !ok {
		return nil, ErrTenantContextRequired
	}

	stats, err := l.repo.GetStats(ctx)
	if err != nil {
		return nil, err
	}

	return &GetStatsResponse{
		DocumentCount:      stats.DocumentCount,
		CompletedDocuments: stats.CompletedDocuments,
		PendingDocuments:   stats.PendingDocuments,
		EmbeddingCount:     stats.EmbeddingCount,
		TotalFileSizeBytes: stats.TotalFileSizeBytes,
	}, nil
}
