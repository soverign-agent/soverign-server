// Package orchestrator implements the agentic RAG pipeline for compliance chat.
// It uses a deterministic execution graph (analysis -> guardrail -> retrieval -> synthesis)
// without external orchestration frameworks, matching the existing supervisor pattern
// used elsewhere in the codebase.
package orchestrator

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"sovereign-ai-compliance/rag-service/repo"
	"sovereign-ai-compliance/shared/llm"
)

// ProgressStage indicates which step of the pipeline is currently running.
type ProgressStage string

const (
	StageAnalyzing    ProgressStage = "analyzing"
	StageGuardrail    ProgressStage = "guardrail"
	StageRetrieving   ProgressStage = "retrieving"
	StageSynthesizing ProgressStage = "synthesizing"
	StageFormatting   ProgressStage = "formatting"
	StageDone         ProgressStage = "done"
	StageError        ProgressStage = "error"
)

// ProgressEvent is emitted during pipeline execution so callers can stream status.
type ProgressEvent struct {
	Stage       ProgressStage `json:"stage"`
	Message     string        `json:"message"`
	Timestamp   time.Time     `json:"timestamp"`
	SubQueries  []string      `json:"sub_queries,omitempty"`
	ResultCount int           `json:"result_count,omitempty"`
}

// ChatMessage represents a single turn in the conversation history.
type ChatMessage struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// Citation maps a claim in the synthesized answer to its source chunk.
type Citation struct {
	ChunkIndex   int       `json:"chunk_index"`
	DocumentID   uuid.UUID `json:"document_id"`
	DocumentName string    `json:"document_name"`
	Text         string    `json:"text"`
	Similarity   float64   `json:"similarity"`
}

// RetrievedChunk is a single chunk returned from vector search.
type RetrievedChunk struct {
	DocumentID   uuid.UUID `json:"document_id"`
	DocumentName string    `json:"document_name"`
	Text         string    `json:"text"`
	Similarity   float64   `json:"similarity"`
}

// AnalysisResult holds the output of the query analysis node.
type AnalysisResult struct {
	Intent         string   `json:"intent"`
	SubQueries     []string `json:"sub_queries"`
	IsOffTopic     bool     `json:"is_off_topic"`
	OffTopicReason string   `json:"off_topic_reason,omitempty"`
}

// SynthesisResult holds the output of the synthesis node.
type SynthesisResult struct {
	Answer    string     `json:"answer"`
	Citations []Citation `json:"citations"`
}

// PipelineResult is the final output of the agentic RAG pipeline.
type PipelineResult struct {
	Analysis  AnalysisResult   `json:"analysis"`
	Retrieved []RetrievedChunk `json:"retrieved"`
	Synthesis SynthesisResult  `json:"synthesis"`
	Progress  []ProgressEvent  `json:"progress"`
	Error     string           `json:"error,omitempty"`
}

// PipelineInput is the input to the agentic RAG pipeline.
type PipelineInput struct {
	Query       string        `json:"query"`
	History     []ChatMessage `json:"history"`
	TenantID    string        `json:"tenant_id"`
	UserID      string        `json:"user_id"`
	SessionID   string        `json:"session_id"`
	TopK        int           `json:"top_k"`
	StreamCites bool          `json:"stream_cites"` // if true, citations are inline markers
}

// NodeFunc is a single step in the execution graph.
type NodeFunc func(ctx context.Context, state *PipelineState) error

// PipelineState carries mutable state through the execution graph.
type PipelineState struct {
	Input     PipelineInput
	Analysis  AnalysisResult
	Retrieved []RetrievedChunk
	Synthesis SynthesisResult
	Progress  []ProgressEvent
	Err       error
}

// Dependencies holds external dependencies required by pipeline nodes.
type Dependencies struct {
	LLMClient      llm.Client
	EmbeddingModel string
	Repository     *repo.SQLRepository
}

// Errors returned by the orchestrator.
var (
	ErrOffTopicQuestion = errors.New("question is outside the compliance domain")
	ErrNoResults        = errors.New("no relevant documents found")
)
