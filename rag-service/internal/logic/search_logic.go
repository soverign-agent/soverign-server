// Package logic contains the business logic for rag-service.
package logic

import (
	"context"

	"sovereign-ai-compliance/rag-service/repo"
	"sovereign-ai-compliance/shared/llm"
	"sovereign-ai-compliance/shared/tenant"

	"github.com/pgvector/pgvector-go"
	"go.uber.org/zap"
)

// SearchLogic handles RAG search business logic.
type SearchLogic struct {
	repo      *repo.SQLRepository
	llmClient llm.Client
	logger    *zap.Logger
}

// NewSearchLogic creates a new SearchLogic.
func NewSearchLogic(
	repo *repo.SQLRepository,
	llmClient llm.Client,
	logger *zap.Logger,
) *SearchLogic {
	return &SearchLogic{
		repo:      repo,
		llmClient: llmClient,
		logger:    logger,
	}
}

// SearchRequest is the RAG search request.
type SearchRequest struct {
	Query string `json:"query"`
	TopK  int    `json:"top_k"`
}

// SearchResult represents a single search result.
type SearchResult struct {
	DocumentID   string  `json:"document_id"`
	DocumentName string  `json:"document_name"`
	Text         string  `json:"text"`
	Similarity   float64 `json:"similarity"` // 1 - distance, higher = more similar
}

// SearchResponse is the RAG search response.
type SearchResponse struct {
	Query   string         `json:"query"`
	Results []SearchResult `json:"results"`
	TopK    int            `json:"top_k"`
}

// Search performs similarity search for the query.
func (l *SearchLogic) Search(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	_, ok := tenant.FromContext(ctx)
	if !ok {
		return nil, ErrTenantContextRequired
	}

	if req.TopK < 1 || req.TopK > 50 {
		req.TopK = 10 // Default top 10 results
	}

	// Generate embedding for the query
	embResp, err := l.llmClient.Embed(ctx, llm.EmbeddingRequest{
		Input: req.Query,
	})
	if err != nil {
		l.logger.Error("failed to generate query embedding", zap.Error(err))
		return nil, err
	}

	queryEmb := pgvector.NewVector(embResp.Embedding)

	// Search for similar chunks
	results, err := l.repo.SearchSimilar(ctx, queryEmb, req.TopK)
	if err != nil {
		return nil, err
	}

	// Convert to response format
	responseResults := make([]SearchResult, 0, len(results))
	for _, res := range results {
		// Convert distance (lower = more similar) to similarity (higher = more similar)
		// Cosine distance ranges from 0 to 2
		similarity := 1.0 - (res.Distance / 2.0)
		responseResults = append(responseResults, SearchResult{
			DocumentID:   res.DocumentID.String(),
			DocumentName: res.DocumentName,
			Text:         res.Text,
			Similarity:   similarity,
		})
	}

	l.logger.Info("RAG search completed",
		zap.Int("results_found", len(responseResults)),
	)

	return &SearchResponse{
		Query:   req.Query,
		Results: responseResults,
		TopK:    req.TopK,
	}, nil
}
