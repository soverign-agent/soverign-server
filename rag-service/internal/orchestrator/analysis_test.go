package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"sovereign-ai-compliance/rag-service/internal/orchestrator/testutil"
	"sovereign-ai-compliance/shared/llm"
)

func TestAnalysisNode_DirectRetrieval(t *testing.T) {
	mock := &testutil.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
			return llm.CompletionResponse{
				Content: testutil.AnalysisJSON("direct_retrieval", []string{"What is Annex IV?"}, false),
			}, nil
		},
	}

	state := &PipelineState{
		Input: PipelineInput{Query: "What is Annex IV?"},
	}

	err := AnalysisNode(mock)(context.Background(), state)
	assert.NoError(t, err)
	assert.Equal(t, "direct_retrieval", state.Analysis.Intent)
	assert.Len(t, state.Analysis.SubQueries, 1)
	assert.Equal(t, "What is Annex IV?", state.Analysis.SubQueries[0])
	assert.False(t, state.Analysis.IsOffTopic)
	assert.NotEmpty(t, state.Progress)
	assert.Equal(t, StageAnalyzing, state.Progress[0].Stage)
}

func TestAnalysisNode_MultiPart(t *testing.T) {
	mock := &testutil.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
			return llm.CompletionResponse{
				Content: testutil.AnalysisJSON("multi_part", []string{"What is high-risk?", "How does it affect Annex IV?"}, false),
			}, nil
		},
	}

	state := &PipelineState{
		Input: PipelineInput{Query: "What are high-risk items and how do they affect Annex IV?"},
	}

	err := AnalysisNode(mock)(context.Background(), state)
	assert.NoError(t, err)
	assert.Equal(t, "multi_part", state.Analysis.Intent)
	assert.Len(t, state.Analysis.SubQueries, 2)
}

func TestAnalysisNode_FallbackOnLLMError(t *testing.T) {
	mock := &testutil.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
			return llm.CompletionResponse{}, assert.AnError
		},
	}

	state := &PipelineState{
		Input: PipelineInput{Query: "What is Annex IV?"},
	}

	err := AnalysisNode(mock)(context.Background(), state)
	assert.NoError(t, err)
	assert.Equal(t, "direct_retrieval", state.Analysis.Intent)
	assert.Equal(t, []string{"What is Annex IV?"}, state.Analysis.SubQueries)
}

func TestAnalysisNode_FollowUpResolution(t *testing.T) {
	mock := &testutil.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
			return llm.CompletionResponse{
				Content: testutil.AnalysisJSON("direct_retrieval", []string{"What are the risk scores for high-risk AI systems?"}, false),
			}, nil
		},
	}

	state := &PipelineState{
		Input: PipelineInput{
			Query: "What about the risk score for that?",
			History: []ChatMessage{
				{Role: "user", Content: "Tell me about high-risk AI systems"},
				{Role: "assistant", Content: "High-risk AI systems include biometric identification, critical infrastructure, education, etc."},
			},
		},
	}

	err := AnalysisNode(mock)(context.Background(), state)
	assert.NoError(t, err)
	assert.NotEmpty(t, state.Analysis.SubQueries)
}

func TestAnalysisNode_ParseErrorFallback(t *testing.T) {
	mock := &testutil.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
			return llm.CompletionResponse{Content: "not valid json"}, nil
		},
	}

	state := &PipelineState{
		Input: PipelineInput{Query: "What is Annex IV?"},
	}

	err := AnalysisNode(mock)(context.Background(), state)
	assert.NoError(t, err)
	assert.Equal(t, "direct_retrieval", state.Analysis.Intent)
}
