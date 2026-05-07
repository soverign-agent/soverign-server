package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"sovereign-ai-compliance/rag-service/internal/orchestrator/testutil"
	"sovereign-ai-compliance/shared/llm"
)

func TestGuardrailNode_AllowsComplianceQuestion(t *testing.T) {
	mock := &testutil.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
			return llm.CompletionResponse{
				Content: testutil.GuardrailJSON(true, 0.9, ""),
			}, nil
		},
	}

	state := &PipelineState{
		Input: PipelineInput{Query: "What are the Annex IV documentation requirements?"},
	}

	node := GuardrailNode(GuardrailConfig{Enabled: true, UseLLMCheck: true, BlockThreshold: 0.3}, mock)
	err := node(context.Background(), state)
	assert.NoError(t, err)
	assert.False(t, state.Analysis.IsOffTopic)
}

func TestGuardrailNode_BlocksOffTopicQuestion(t *testing.T) {
	mock := &testutil.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
			return llm.CompletionResponse{
				Content: testutil.GuardrailJSON(false, 0.1, "This is about cooking, not compliance."),
			}, nil
		},
	}

	state := &PipelineState{
		Input: PipelineInput{Query: "How do I bake a cake?"},
	}

	node := GuardrailNode(GuardrailConfig{Enabled: true, UseLLMCheck: true, BlockThreshold: 0.3}, mock)
	err := node(context.Background(), state)
	assert.ErrorIs(t, err, ErrOffTopicQuestion)
	assert.True(t, state.Analysis.IsOffTopic)
	assert.Contains(t, state.Analysis.OffTopicReason, "cooking")
}

func TestGuardrailNode_HeuristicFastPath(t *testing.T) {
	// LLM should not be called because keywords give a high heuristic score
	mock := &testutil.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
			t.Fatal("LLM should not be called for high-confidence heuristic match")
			return llm.CompletionResponse{}, nil
		},
	}

	state := &PipelineState{
		Input: PipelineInput{Query: "What are the high-risk AI Act compliance requirements for Annex IV documentation?"},
	}

	node := GuardrailNode(GuardrailConfig{Enabled: true, UseLLMCheck: true, BlockThreshold: 0.3}, mock)
	err := node(context.Background(), state)
	assert.NoError(t, err)
}

func TestGuardrailNode_Disabled(t *testing.T) {
	mock := &testutil.MockLLMClient{}

	state := &PipelineState{
		Input: PipelineInput{Query: "How do I bake a cake?"},
	}

	node := GuardrailNode(GuardrailConfig{Enabled: false}, mock)
	err := node(context.Background(), state)
	assert.NoError(t, err)
}

func TestGuardrailNode_NoLLMCheck(t *testing.T) {
	mock := &testutil.MockLLMClient{}

	state := &PipelineState{
		Input: PipelineInput{Query: "random question without keywords"},
	}

	node := GuardrailNode(GuardrailConfig{Enabled: true, UseLLMCheck: false, BlockThreshold: 0.3}, mock)
	err := node(context.Background(), state)
	assert.ErrorIs(t, err, ErrOffTopicQuestion)
}

func TestGuardrailNode_LLMErrorFailsOpen(t *testing.T) {
	mock := &testutil.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
			return llm.CompletionResponse{}, assert.AnError
		},
	}

	state := &PipelineState{
		Input: PipelineInput{Query: "vague question"},
	}

	node := GuardrailNode(GuardrailConfig{Enabled: true, UseLLMCheck: true, BlockThreshold: 0.3}, mock)
	err := node(context.Background(), state)
	assert.NoError(t, err) // fails open
}
