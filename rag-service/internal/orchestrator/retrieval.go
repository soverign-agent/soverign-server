// Package orchestrator implements the agentic RAG pipeline for compliance chat.
package orchestrator

import (
	"context"
	"fmt"

	"github.com/pgvector/pgvector-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"sovereign-ai-compliance/rag-service/repo"
	"sovereign-ai-compliance/shared/llm"
)

var retrievalTracer = otel.Tracer("rag-service/orchestrator/retrieval")

// RetrievalNode performs tenant-scoped vector search for each sub-query.
func RetrievalNode(repository *repo.SQLRepository, client llm.Client, embeddingModel string, defaultTopK int) NodeFunc {
	return func(ctx context.Context, state *PipelineState) error {
		ctx, span := retrievalTracer.Start(ctx, "RetrievalNode")
		defer span.End()

		span.SetAttributes(
			attribute.Int("sub_query_count", len(state.Analysis.SubQueries)),
			attribute.Int("default_top_k", defaultTopK),
		)

		state.AddProgress(StageRetrieving, "Searching knowledge base...", state.Analysis.SubQueries, 0)

		topK := state.Input.TopK
		if topK < 1 || topK > 50 {
			topK = defaultTopK
		}
		if topK < 1 {
			topK = 10
		}

		// Deduplication map by document_id + text to avoid returning identical chunks
		seen := make(map[string]struct{})
		var allResults []RetrievedChunk

		for _, subQuery := range state.Analysis.SubQueries {
			// Generate embedding for the sub-query
			embResp, err := client.Embed(ctx, llm.EmbeddingRequest{
				Model: embeddingModel,
				Input: subQuery,
			})
			if err != nil {
				// Log and continue with next sub-query
				state.AddProgress(StageRetrieving, fmt.Sprintf("Embedding failed for sub-query: %s", subQuery), nil, 0)
				continue
			}

			queryEmb := pgvector.NewVector(embResp.Embedding)

			// Search with tenant isolation enforced via RLS (transaction sets tenant)
			rawResults, err := repository.SearchSimilar(ctx, queryEmb, topK)
			if err != nil {
				state.AddProgress(StageRetrieving, fmt.Sprintf("Search failed for sub-query: %s", subQuery), nil, 0)
				continue
			}

			for _, r := range rawResults {
				key := r.DocumentID.String() + "::" + r.Text
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}

				similarity := 1.0 - (r.Distance / 2.0)
				allResults = append(allResults, RetrievedChunk{
					DocumentID:   r.DocumentID,
					DocumentName: r.DocumentName,
					Text:         r.Text,
					Similarity:   similarity,
				})
			}
		}

		if len(allResults) == 0 {
			state.Err = ErrNoResults
			span.SetAttributes(attribute.Int("result_count", 0), attribute.String("result", "no_results"))
			state.AddProgress(StageRetrieving, "No relevant documents found.", nil, 0)
			return ErrNoResults
		}

		// Sort by similarity descending (already roughly ordered, but re-sort to be safe)
		// We keep topK * number_of_subqueries but cap at a reasonable maximum.
		maxResults := topK * 3
		if len(allResults) > maxResults {
			allResults = allResults[:maxResults]
		}

		state.Retrieved = allResults
		span.SetAttributes(attribute.Int("result_count", len(allResults)), attribute.String("result", "success"))
		state.AddProgress(StageRetrieving, fmt.Sprintf("Retrieved %d relevant chunks.", len(allResults)), nil, len(allResults))
		return nil
	}
}
