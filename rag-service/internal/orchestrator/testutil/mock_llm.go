// Package testutil provides test helpers for the orchestrator package.
package testutil

import (
	"context"
	"fmt"
	"strings"

	"sovereign-ai-compliance/shared/llm"
)

// MockLLMClient is a test double for llm.Client.
type MockLLMClient struct {
	CompleteFunc       func(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error)
	StreamCompleteFunc func(ctx context.Context, req llm.CompletionRequest, onDelta func(string)) (llm.StreamCompletionResponse, error)
	EmbedFunc          func(ctx context.Context, req llm.EmbeddingRequest) (llm.EmbeddingResponse, error)
	HealthFunc         func(ctx context.Context) error
}

// Complete implements llm.Client.
func (m *MockLLMClient) Complete(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
	if m.CompleteFunc != nil {
		return m.CompleteFunc(ctx, req)
	}
	return llm.CompletionResponse{}, fmt.Errorf("Complete not implemented")
}

// StreamComplete implements llm.Client.
func (m *MockLLMClient) StreamComplete(ctx context.Context, req llm.CompletionRequest, onDelta func(string)) (llm.StreamCompletionResponse, error) {
	if m.StreamCompleteFunc != nil {
		return m.StreamCompleteFunc(ctx, req, onDelta)
	}
	return llm.StreamCompletionResponse{}, fmt.Errorf("StreamComplete not implemented")
}

// Embed implements llm.Client.
func (m *MockLLMClient) Embed(ctx context.Context, req llm.EmbeddingRequest) (llm.EmbeddingResponse, error) {
	if m.EmbedFunc != nil {
		return m.EmbedFunc(ctx, req)
	}
	return llm.EmbeddingResponse{}, fmt.Errorf("Embed not implemented")
}

// Health implements llm.Client.
func (m *MockLLMClient) Health(ctx context.Context) error {
	if m.HealthFunc != nil {
		return m.HealthFunc(ctx)
	}
	return nil
}

// StaticEmbedding returns a fixed-length embedding vector for deterministic tests.
func StaticEmbedding(dim int) []float32 {
	v := make([]float32, dim)
	for i := range v {
		v[i] = 0.1 * float32(i%10)
	}
	return v
}

// AnalysisJSON returns a valid analysis response JSON.
func AnalysisJSON(intent string, subQueries []string, offTopic bool) string {
	q := strings.Join(subQueries, `","`)
	return fmt.Sprintf(`{"intent":"%s","sub_queries":["%s"],"is_off_topic":%t,"off_topic_reason":""}`, intent, q, offTopic)
}

// GuardrailJSON returns a valid guardrail response JSON.
func GuardrailJSON(compliant bool, score float64, reason string) string {
	return fmt.Sprintf(`{"is_compliant":%t,"score":%f,"reason":"%s"}`, compliant, score, reason)
}
