// Package orchestrator implements the agentic RAG pipeline for compliance chat.
package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"sovereign-ai-compliance/shared/llm"
)

var analysisTracer = otel.Tracer("rag-service/orchestrator/analysis")

// AnalysisNode classifies user intent and decomposes multi-part questions.
func AnalysisNode(client llm.Client) NodeFunc {
	return func(ctx context.Context, state *PipelineState) error {
		ctx, span := analysisTracer.Start(ctx, "AnalysisNode")
		defer span.End()

		span.SetAttributes(
			attribute.String("query", truncateAttr(state.Input.Query, 200)),
			attribute.Int("history_len", len(state.Input.History)),
		)

		state.AddProgress(StageAnalyzing, "Analyzing your question...", nil, 0)

		query := state.Input.Query
		history := state.Input.History

		// Build conversation context for follow-up resolution
		var historyContext string
		if len(history) > 0 {
			var sb strings.Builder
			// Include last 6 messages (3 turns) for context
			start := 0
			if len(history) > 6 {
				start = len(history) - 6
			}
			for _, msg := range history[start:] {
				fmt.Fprintf(&sb, "%s: %s\n", msg.Role, msg.Content)
			}
			historyContext = sb.String()
		}

		prompt := buildAnalysisPrompt(query, historyContext)

		resp, err := client.Complete(ctx, llm.CompletionRequest{
			Messages: []llm.Message{
				{Role: "system", Content: "You are a query analysis engine for an EU AI Act compliance platform. You only output valid JSON."},
				{Role: "user", Content: prompt},
			},
			Temperature: 0.1,
			MaxTokens:   512,
		})
		if err != nil {
			// Fallback: treat as single direct retrieval query
			state.Analysis = AnalysisResult{
				Intent:     "direct_retrieval",
				SubQueries: []string{query},
				IsOffTopic: false,
			}
			span.SetAttributes(attribute.String("fallback_reason", "llm_error"))
			span.RecordError(err)
			state.AddProgress(StageAnalyzing, "Using direct retrieval fallback.", state.Analysis.SubQueries, 0)
			return nil
		}

		result, err := parseAnalysisResponse(resp.Content)
		if err != nil {
			// Fallback on parse error
			state.Analysis = AnalysisResult{
				Intent:     "direct_retrieval",
				SubQueries: []string{query},
				IsOffTopic: false,
			}
			span.SetAttributes(attribute.String("fallback_reason", "parse_error"))
			span.RecordError(err)
			state.AddProgress(StageAnalyzing, "Using direct retrieval fallback.", state.Analysis.SubQueries, 0)
			return nil
		}

		state.Analysis = result
		span.SetAttributes(
			attribute.String("intent", result.Intent),
			attribute.Int("sub_query_count", len(result.SubQueries)),
			attribute.Bool("is_off_topic", result.IsOffTopic),
		)
		if result.IsOffTopic {
			span.SetAttributes(attribute.String("off_topic_reason", result.OffTopicReason))
		}
		state.AddProgress(StageAnalyzing, fmt.Sprintf("Identified intent: %s", result.Intent), result.SubQueries, 0)
		return nil
	}
}

func buildAnalysisPrompt(query, historyContext string) string {
	var sb strings.Builder
	sb.WriteString("Analyze the following user question for an EU AI Act compliance chatbot.\n\n")

	if historyContext != "" {
		sb.WriteString("Recent conversation history:\n")
		sb.WriteString(historyContext)
		sb.WriteString("\n")
	}

	fmt.Fprintf(&sb, "Current question: \"%s\"\n\n", query)
	sb.WriteString(`Respond with a JSON object exactly in this format (no markdown, no extra text):
{
  "intent": "direct_retrieval|multi_part|clarification|summary",
  "sub_queries": ["first sub-query", "second sub-query"],
  "is_off_topic": false,
  "off_topic_reason": ""
}

Rules:
- intent "direct_retrieval": a single factual question answerable by one document chunk.
- intent "multi_part": a complex question spanning multiple documents or topics; decompose into 2-4 sub-queries.
- intent "clarification": the question is ambiguous or refers to prior context without enough detail.
- intent "summary": the user wants a summary across multiple documents.
- sub_queries: if multi_part, list the decomposed questions; otherwise use the original question as a single item.
- is_off_topic: only true if the question is clearly unrelated to AI Act compliance, risk, audit, or governance.
- If the question is a follow-up (e.g., "What about that?", "Why?", "How?"), resolve the referent using the conversation history and formulate a complete standalone sub-query.
`)
	return sb.String()
}

// analysisResponse is the expected JSON shape from the LLM analysis prompt.
type analysisResponse struct {
	Intent         string   `json:"intent"`
	SubQueries     []string `json:"sub_queries"`
	IsOffTopic     bool     `json:"is_off_topic"`
	OffTopicReason string   `json:"off_topic_reason"`
}

func parseAnalysisResponse(raw string) (AnalysisResult, error) {
	clean := strings.TrimSpace(raw)
	clean = strings.TrimPrefix(clean, "```json")
	clean = strings.TrimPrefix(clean, "```")
	clean = strings.TrimSuffix(clean, "```")
	clean = strings.TrimSpace(clean)

	var parsed analysisResponse
	if err := json.Unmarshal([]byte(clean), &parsed); err != nil {
		return AnalysisResult{}, fmt.Errorf("unmarshal analysis response: %w", err)
	}

	// Normalize sub_queries: never empty
	if len(parsed.SubQueries) == 0 {
		parsed.SubQueries = []string{clean}
	}

	return AnalysisResult{
		Intent:         parsed.Intent,
		SubQueries:     parsed.SubQueries,
		IsOffTopic:     parsed.IsOffTopic,
		OffTopicReason: parsed.OffTopicReason,
	}, nil
}

// truncateAttr returns s shortened to max runes for use in span attributes.
func truncateAttr(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}
