package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"sovereign-ai-compliance/rag-service/internal/orchestrator/testutil"
	"sovereign-ai-compliance/shared/llm"
)

func TestPipeline_Execute_Success(t *testing.T) {
	mock := &testutil.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
			content := req.Messages[len(req.Messages)-1].Content
			if contains(content, "domain guardrail") {
				return llm.CompletionResponse{Content: testutil.GuardrailJSON(true, 0.9, "")}, nil
			}
			return llm.CompletionResponse{Content: testutil.AnalysisJSON("direct_retrieval", []string{"What is Annex IV?"}, false)}, nil
		},
		StreamCompleteFunc: func(ctx context.Context, req llm.CompletionRequest, onDelta func(string)) (llm.StreamCompletionResponse, error) {
			if onDelta != nil {
				onDelta("Annex IV requires technical documentation.")
			}
			return llm.StreamCompletionResponse{Content: "Annex IV requires technical documentation."}, nil
		},
	}

	// Repository is nil because retrieval will fail; test the pipeline flow up to that point
	deps := Dependencies{LLMClient: mock, Repository: nil}
	p := NewPipeline(deps, zap.NewNop(), DefaultGuardrailConfig(), 5)

	input := PipelineInput{Query: "What is Annex IV?", TenantID: "tenant-1"}
	result, err := p.Execute(context.Background(), input)

	// Retrieval fails because repo is nil, which returns ErrNoResults -> graceful
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "direct_retrieval", result.Analysis.Intent)
	assert.Equal(t, "Annex IV requires technical documentation.", result.Synthesis.Answer)
	assert.Empty(t, result.Error)
}

func TestPipeline_Execute_OffTopic(t *testing.T) {
	mock := &testutil.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
			content := req.Messages[len(req.Messages)-1].Content
			if contains(content, "domain guardrail") {
				return llm.CompletionResponse{Content: testutil.GuardrailJSON(false, 0.1, "Off topic")}, nil
			}
			return llm.CompletionResponse{Content: testutil.AnalysisJSON("direct_retrieval", []string{"How do I bake?"}, false)}, nil
		},
	}

	deps := Dependencies{LLMClient: mock}
	p := NewPipeline(deps, zap.NewNop(), DefaultGuardrailConfig(), 5)

	input := PipelineInput{Query: "How do I bake a cake?", TenantID: "tenant-1"}
	result, err := p.Execute(context.Background(), input)

	assert.NoError(t, err) // graceful rejection
	assert.True(t, result.Analysis.IsOffTopic)
	assert.Equal(t, "Off topic", result.Synthesis.Answer)
	assert.Empty(t, result.Error)
}

func TestPipeline_StreamAnswer_OffTopicReturnsRenderableAnswer(t *testing.T) {
	mock := &testutil.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
			content := req.Messages[len(req.Messages)-1].Content
			if contains(content, "domain guardrail") {
				return llm.CompletionResponse{Content: testutil.GuardrailJSON(false, 0.1, "The question is too vague and does not provide any context related to AI Act compliance.")}, nil
			}
			return llm.CompletionResponse{Content: testutil.AnalysisJSON("clarification", []string{"who are u"}, false)}, nil
		},
	}

	deps := Dependencies{LLMClient: mock}
	p := NewPipeline(deps, zap.NewNop(), DefaultGuardrailConfig(), 5)

	var tokens []string
	var events []ProgressEvent

	result, err := p.StreamAnswer(context.Background(), PipelineInput{Query: "who are u", TenantID: "tenant-1"},
		func(token string) { tokens = append(tokens, token) },
		func(event ProgressEvent) { events = append(events, event) },
	)

	assert.NoError(t, err)
	assert.Equal(t, "The question is too vague and does not provide any context related to AI Act compliance.", result.Synthesis.Answer)
	assert.Empty(t, tokens)
	assert.Empty(t, result.Error)

	stages := make(map[ProgressStage]bool)
	for _, e := range events {
		stages[e.Stage] = true
	}
	assert.True(t, stages[StageDone])
	assert.False(t, stages[StageError])
}

func TestPipeline_StreamAnswer(t *testing.T) {
	mock := &testutil.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
			content := req.Messages[len(req.Messages)-1].Content
			if contains(content, "domain guardrail") {
				return llm.CompletionResponse{Content: testutil.GuardrailJSON(true, 0.9, "")}, nil
			}
			return llm.CompletionResponse{Content: testutil.AnalysisJSON("direct_retrieval", []string{"What is Annex IV?"}, false)}, nil
		},
		StreamCompleteFunc: func(ctx context.Context, req llm.CompletionRequest, onDelta func(string)) (llm.StreamCompletionResponse, error) {
			if onDelta != nil {
				onDelta("Answer token.")
			}
			return llm.StreamCompletionResponse{Content: "Answer token."}, nil
		},
	}

	deps := Dependencies{LLMClient: mock}
	p := NewPipeline(deps, zap.NewNop(), DefaultGuardrailConfig(), 5)

	var tokens []string
	var events []ProgressEvent

	input := PipelineInput{Query: "What is Annex IV?", TenantID: "tenant-1"}
	result, err := p.StreamAnswer(context.Background(), input,
		func(token string) { tokens = append(tokens, token) },
		func(event ProgressEvent) { events = append(events, event) },
	)

	// No repository means no results; pipeline returns gracefully
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, events)
	assert.Equal(t, []string{"Answer token."}, tokens)
	assert.Equal(t, "Answer token.", result.Synthesis.Answer)
	assert.Empty(t, result.Error)

	stages := make(map[ProgressStage]bool)
	for _, e := range events {
		stages[e.Stage] = true
	}
	assert.True(t, stages[StageAnalyzing])
	assert.True(t, stages[StageGuardrail])
	assert.True(t, stages[StageDone])
	assert.False(t, stages[StageError])
}

func TestPipeline_ExecuteWithProgress(t *testing.T) {
	mock := &testutil.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
			content := req.Messages[len(req.Messages)-1].Content
			if contains(content, "domain guardrail") {
				return llm.CompletionResponse{Content: testutil.GuardrailJSON(true, 0.9, "")}, nil
			}
			return llm.CompletionResponse{Content: testutil.AnalysisJSON("direct_retrieval", []string{"q"}, false)}, nil
		},
		StreamCompleteFunc: func(ctx context.Context, req llm.CompletionRequest, onDelta func(string)) (llm.StreamCompletionResponse, error) {
			return llm.StreamCompletionResponse{Content: "ans"}, nil
		},
	}

	deps := Dependencies{LLMClient: mock}
	p := NewPipeline(deps, zap.NewNop(), DefaultGuardrailConfig(), 5)

	progressCh := make(chan ProgressEvent, 10)
	input := PipelineInput{Query: "q", TenantID: "tenant-1"}

	go func() {
		p.ExecuteWithProgress(context.Background(), input, progressCh)
	}()

	var events []ProgressEvent
	for evt := range progressCh {
		events = append(events, evt)
	}

	assert.NotEmpty(t, events)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
