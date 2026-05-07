package orchestrator

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"sovereign-ai-compliance/rag-service/internal/orchestrator/testutil"
	"sovereign-ai-compliance/shared/llm"
)

func TestSynthesisNode_GeneratesAnswer(t *testing.T) {
	mock := &testutil.MockLLMClient{
		StreamCompleteFunc: func(ctx context.Context, req llm.CompletionRequest, onDelta func(string)) (llm.StreamCompletionResponse, error) {
			if onDelta != nil {
				onDelta("Annex IV requires technical documentation.")
			}
			return llm.StreamCompletionResponse{
				Content: "Annex IV requires technical documentation.",
			}, nil
		},
	}

	state := &PipelineState{
		Input: PipelineInput{Query: "What does Annex IV require?"},
		Retrieved: []RetrievedChunk{
			{DocumentID: uuid.New(), DocumentName: "doc1", Text: "Annex IV requires technical documentation.", Similarity: 0.95},
		},
	}

	err := SynthesisNode(mock)(context.Background(), state)
	assert.NoError(t, err)
	assert.Contains(t, state.Synthesis.Answer, "Annex IV requires technical documentation")
}

func TestSynthesisNode_ExtractsCitations(t *testing.T) {
	mock := &testutil.MockLLMClient{
		StreamCompleteFunc: func(ctx context.Context, req llm.CompletionRequest, onDelta func(string)) (llm.StreamCompletionResponse, error) {
			content := "According to the guidelines [CITE:1], high-risk systems need documentation [CITE:2]."
			if onDelta != nil {
				onDelta(content)
			}
			return llm.StreamCompletionResponse{Content: content}, nil
		},
	}

	doc1 := uuid.New()
	doc2 := uuid.New()
	state := &PipelineState{
		Input: PipelineInput{Query: "What do high-risk systems need?"},
		Retrieved: []RetrievedChunk{
			{DocumentID: doc1, DocumentName: "guidelines", Text: "High-risk systems need documentation.", Similarity: 0.92},
			{DocumentID: doc2, DocumentName: "annex", Text: "Annex IV lists requirements.", Similarity: 0.88},
		},
	}

	err := SynthesisNode(mock)(context.Background(), state)
	assert.NoError(t, err)
	assert.Len(t, state.Synthesis.Citations, 2)
	assert.Equal(t, doc1, state.Synthesis.Citations[0].DocumentID)
	assert.Equal(t, doc2, state.Synthesis.Citations[1].DocumentID)
}

func TestSynthesisNode_FallbackOnError(t *testing.T) {
	mock := &testutil.MockLLMClient{
		StreamCompleteFunc: func(ctx context.Context, req llm.CompletionRequest, onDelta func(string)) (llm.StreamCompletionResponse, error) {
			return llm.StreamCompletionResponse{}, assert.AnError
		},
	}

	state := &PipelineState{
		Input:     PipelineInput{Query: "What does Annex IV require?"},
		Retrieved: []RetrievedChunk{{DocumentID: uuid.New(), DocumentName: "doc1", Text: "text", Similarity: 0.9}},
	}

	err := SynthesisNode(mock)(context.Background(), state)
	assert.Error(t, err)
	assert.Contains(t, state.Synthesis.Answer, "sorry")
}

func TestExtractCitations(t *testing.T) {
	retrieved := []RetrievedChunk{
		{DocumentID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), DocumentName: "A", Text: "text A", Similarity: 0.9},
		{DocumentID: uuid.MustParse("22222222-2222-2222-2222-222222222222"), DocumentName: "B", Text: "text B", Similarity: 0.8},
	}

	answer := "Hello [CITE:1] world [CITE:2] and [CITE:1] again."
	citations := extractCitations(answer, retrieved)

	assert.Len(t, citations, 2)
	assert.Equal(t, 1, citations[0].ChunkIndex)
	assert.Equal(t, "A", citations[0].DocumentName)
	assert.Equal(t, 2, citations[1].ChunkIndex)
	assert.Equal(t, "B", citations[1].DocumentName)
}
