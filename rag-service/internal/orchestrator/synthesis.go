// Package orchestrator implements the agentic RAG pipeline for compliance chat.
package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"sovereign-ai-compliance/shared/llm"
)

var synthesisTracer = otel.Tracer("rag-service/orchestrator/synthesis")

// SynthesisNode generates a cited answer from retrieved chunks and conversation history.
func SynthesisNode(client llm.Client) NodeFunc {
	return func(ctx context.Context, state *PipelineState) error {
		ctx, span := synthesisTracer.Start(ctx, "SynthesisNode")
		defer span.End()

		span.SetAttributes(
			attribute.Int("retrieved_count", len(state.Retrieved)),
			attribute.Int("history_len", len(state.Input.History)),
		)

		state.AddProgress(StageSynthesizing, "Synthesizing answer...", nil, len(state.Retrieved))

		query := state.Input.Query
		history := state.Input.History
		retrieved := state.Retrieved

		prompt := buildSynthesisPrompt(query, history, retrieved)

		var answerBuilder strings.Builder
		resp, err := client.StreamComplete(ctx, llm.CompletionRequest{
			Messages: []llm.Message{
				{Role: "system", Content: "You are a compliance assistant for the EU AI Act. You answer based ONLY on the provided context. Always cite your sources using [CITE:N] markers where N is the chunk index. If the context does not contain the answer, say so clearly."},
				{Role: "user", Content: prompt},
			},
			Temperature: 0.2,
			MaxTokens:   2048,
		}, func(token string) {
			answerBuilder.WriteString(token)
		})
		if err != nil {
			state.Synthesis = SynthesisResult{
				Answer:    "I'm sorry, I encountered an error while generating the answer. Please try again.",
				Citations: []Citation{},
			}
			span.SetAttributes(attribute.String("result", "error"))
			span.RecordError(err)
			return fmt.Errorf("synthesis stream failed: %w", err)
		}

		answer := answerBuilder.String()
		if answer == "" {
			answer = resp.Content
		}

		citations := extractCitations(answer, retrieved)

		state.Synthesis = SynthesisResult{
			Answer:    answer,
			Citations: citations,
		}

		span.SetAttributes(
			attribute.String("result", "success"),
			attribute.Int("citation_count", len(citations)),
			attribute.Int("answer_length", len([]rune(answer))),
		)

		state.AddProgress(StageFormatting, "Formatting citations...", nil, len(citations))
		return nil
	}
}

func buildSynthesisPrompt(query string, history []ChatMessage, retrieved []RetrievedChunk) string {
	var sb strings.Builder

	// Conversation context
	if len(history) > 0 {
		sb.WriteString("Conversation history (most recent first):\n")
		start := 0
		if len(history) > 6 {
			start = len(history) - 6
		}
		for i := len(history) - 1; i >= start; i-- {
			msg := history[i]
			fmt.Fprintf(&sb, "%s: %s\n", msg.Role, msg.Content)
		}
		sb.WriteString("\n")
	}

	fmt.Fprintf(&sb, "User question: \"%s\"\n\n", query)
	sb.WriteString("Retrieved context chunks:\n")
	for i, chunk := range retrieved {
		fmt.Fprintf(&sb, "[CHUNK %d] Document: %s (similarity: %.3f)\n%s\n\n", i+1, chunk.DocumentName, chunk.Similarity, chunk.Text)
	}

	sb.WriteString(`Instructions:
1. Answer the user's question using ONLY the retrieved context above.
2. For every factual claim, include an inline citation in the format [CITE:N] where N matches the chunk number.
3. If the context does not contain enough information to answer, explicitly state: "The provided documents do not contain sufficient information to answer this question."
4. Be concise but thorough. Use bullet points for multi-part answers when appropriate.
5. Do not make up facts that are not present in the context.
`)
	return sb.String()
}

// extractCitations parses [CITE:N] markers from the answer and maps them to RetrievedChunk metadata.
func extractCitations(answer string, retrieved []RetrievedChunk) []Citation {
	var citations []Citation
	seen := make(map[int]struct{})

	// Simple parser: look for [CITE:1], [CITE:2], etc.
	for i := 1; i <= len(retrieved); i++ {
		marker := fmt.Sprintf("[CITE:%d]", i)
		if strings.Contains(answer, marker) {
			if _, ok := seen[i]; ok {
				continue
			}
			seen[i] = struct{}{}
			chunk := retrieved[i-1]
			citations = append(citations, Citation{
				ChunkIndex:   i,
				DocumentID:   chunk.DocumentID,
				DocumentName: chunk.DocumentName,
				Text:         chunk.Text,
				Similarity:   chunk.Similarity,
			})
		}
	}
	return citations
}
