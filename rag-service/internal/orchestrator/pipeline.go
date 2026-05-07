// Package orchestrator implements the agentic RAG pipeline for compliance chat.
package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
	"sovereign-ai-compliance/shared/llm"
)

var tracer = otel.Tracer("rag-service/orchestrator")

const noResultsAnswer = "The provided documents do not contain sufficient information to answer this question."

// Pipeline orchestrates the agentic RAG execution graph.
type Pipeline struct {
	deps         Dependencies
	logger       *zap.Logger
	guardrailCfg GuardrailConfig
	topK         int
}

// NewPipeline creates a new agentic RAG pipeline.
func NewPipeline(deps Dependencies, logger *zap.Logger, guardrailCfg GuardrailConfig, topK int) *Pipeline {
	if topK < 1 {
		topK = 10
	}
	return &Pipeline{
		deps:         deps,
		logger:       logger,
		guardrailCfg: guardrailCfg,
		topK:         topK,
	}
}

// Execute runs the full pipeline synchronously and returns the final result.
// Progress events are accumulated in result.Progress.
func (p *Pipeline) Execute(ctx context.Context, input PipelineInput) (*PipelineResult, error) {
	ctx, span := tracer.Start(ctx, "orchestrator.Execute")
	defer span.End()
	span.SetAttributes(
		attribute.String("tenant_id", input.TenantID),
		attribute.String("session_id", input.SessionID),
		attribute.Int("history_len", len(input.History)),
	)

	state := &PipelineState{Input: input}

	nodes := []struct {
		name string
		fn   NodeFunc
	}{
		{"analysis", AnalysisNode(p.deps.LLMClient)},
		{"retrieval", RetrievalNode(p.deps.Repository, p.deps.LLMClient, p.deps.EmbeddingModel, p.topK)},
		{"synthesis", SynthesisNode(p.deps.LLMClient)},
	}

	for _, n := range nodes {
		if err := n.fn(ctx, state); err != nil {
			span.SetAttributes(attribute.String("failed_node", n.name))
			span.RecordError(err)
			if err == ErrNoResults {
				if guardErr := GuardrailNode(p.guardrailCfg, p.deps.LLMClient)(ctx, state); guardErr == ErrOffTopicQuestion {
					applyOffTopicFallback(state)
					return p.toResult(state), nil
				}
				generateNoResultsFallback(ctx, state, p.deps.LLMClient, nil)
				return p.toResult(state), nil
			}
			p.logger.Error("pipeline node failed",
				zap.String("stage", string(state.Progress[len(state.Progress)-1].Stage)),
				zap.Error(err))
			state.Err = err
			state.AddProgress(StageError, fmt.Sprintf("Pipeline error: %v", err), nil, 0)
			return p.toResult(state), err
		}
	}

	state.AddProgress(StageDone, "Answer ready.", nil, 0)
	return p.toResult(state), nil
}

// ExecuteWithProgress runs the pipeline and streams progress events to the provided channel.
// The channel is closed when execution completes.
func (p *Pipeline) ExecuteWithProgress(ctx context.Context, input PipelineInput, progressCh chan<- ProgressEvent) (*PipelineResult, error) {
	ctx, span := tracer.Start(ctx, "orchestrator.ExecuteWithProgress")
	defer span.End()
	span.SetAttributes(
		attribute.String("tenant_id", input.TenantID),
		attribute.String("session_id", input.SessionID),
		attribute.Int("history_len", len(input.History)),
	)
	defer close(progressCh)

	state := &PipelineState{Input: input}
	lastLen := 0
	flush := func() {
		for ; lastLen < len(state.Progress); lastLen++ {
			select {
			case progressCh <- state.Progress[lastLen]:
			case <-ctx.Done():
				return
			}
		}
	}

	nodes := []struct {
		name string
		fn   NodeFunc
	}{
		{"analysis", AnalysisNode(p.deps.LLMClient)},
		{"retrieval", RetrievalNode(p.deps.Repository, p.deps.LLMClient, p.deps.EmbeddingModel, p.topK)},
		{"synthesis", SynthesisNode(p.deps.LLMClient)},
	}

	for _, n := range nodes {
		if err := n.fn(ctx, state); err != nil {
			flush()
			span.SetAttributes(attribute.String("failed_node", n.name))
			span.RecordError(err)
			if err == ErrNoResults {
				if guardErr := GuardrailNode(p.guardrailCfg, p.deps.LLMClient)(ctx, state); guardErr == ErrOffTopicQuestion {
					applyOffTopicFallback(state)
					flush()
					return p.toResult(state), nil
				}
				generateNoResultsFallback(ctx, state, p.deps.LLMClient, nil)
				flush()
				return p.toResult(state), nil
			}
			p.logger.Error("pipeline node failed",
				zap.String("stage", string(state.Progress[len(state.Progress)-1].Stage)),
				zap.Error(err))
			state.Err = err
			state.AddProgress(StageError, fmt.Sprintf("Pipeline error: %v", err), nil, 0)
			flush()
			return p.toResult(state), err
		}
		flush()
	}

	state.AddProgress(StageDone, "Answer ready.", nil, 0)
	flush()
	return p.toResult(state), nil
}

// StreamAnswer runs the pipeline and streams answer tokens via onDelta while also
// emitting progress events via onProgress. This is the primary entry point for
// real-time chat responses.
func (p *Pipeline) StreamAnswer(ctx context.Context, input PipelineInput, onDelta func(token string), onProgress func(event ProgressEvent)) (*PipelineResult, error) {
	ctx, span := tracer.Start(ctx, "orchestrator.StreamAnswer")
	defer span.End()
	span.SetAttributes(
		attribute.String("tenant_id", input.TenantID),
		attribute.String("session_id", input.SessionID),
		attribute.Int("history_len", len(input.History)),
	)

	state := &PipelineState{Input: input}
	lastLen := 0
	flush := func() {
		if onProgress == nil {
			lastLen = len(state.Progress)
			return
		}
		for ; lastLen < len(state.Progress); lastLen++ {
			onProgress(state.Progress[lastLen])
		}
	}

	// Run analysis
	if err := AnalysisNode(p.deps.LLMClient)(ctx, state); err != nil {
		p.logger.Error("analysis failed", zap.Error(err))
		state.Err = err
		state.AddProgress(StageError, fmt.Sprintf("Analysis error: %v", err), nil, 0)
		flush()
		return p.toResult(state), err
	}
	flush()

	// Run retrieval
	if err := RetrievalNode(p.deps.Repository, p.deps.LLMClient, p.deps.EmbeddingModel, p.topK)(ctx, state); err != nil {
		flush()
		if err == ErrNoResults {
			if guardErr := GuardrailNode(p.guardrailCfg, p.deps.LLMClient)(ctx, state); guardErr == ErrOffTopicQuestion {
				applyOffTopicFallback(state)
				flush()
				return p.toResult(state), nil
			}
			generateNoResultsFallback(ctx, state, p.deps.LLMClient, onDelta)
			flush()
			return p.toResult(state), nil
		}
		state.Err = err
		state.AddProgress(StageError, fmt.Sprintf("Retrieval error: %v", err), nil, 0)
		flush()
		return p.toResult(state), err
	}
	flush()

	// Run synthesis with streaming
	state.AddProgress(StageSynthesizing, "Synthesizing answer...", nil, len(state.Retrieved))
	flush()

	query := state.Input.Query
	history := state.Input.History
	retrieved := state.Retrieved
	prompt := buildSynthesisPrompt(query, history, retrieved)

	var answerBuilder strings.Builder
	resp, err := p.deps.LLMClient.StreamComplete(ctx, llm.CompletionRequest{
		Messages: []llm.Message{
			{Role: "system", Content: "You are a compliance assistant for the EU AI Act. You answer based ONLY on the provided context. Always cite your sources using [CITE:N] markers where N is the chunk index. If the context does not contain the answer, say so clearly."},
			{Role: "user", Content: prompt},
		},
		Temperature: 0.2,
		MaxTokens:   2048,
	}, func(token string) {
		answerBuilder.WriteString(token)
		if onDelta != nil {
			onDelta(token)
		}
	})
	if err != nil {
		state.Err = err
		state.AddProgress(StageError, fmt.Sprintf("Synthesis error: %v", err), nil, 0)
		flush()
		return p.toResult(state), err
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

	state.AddProgress(StageDone, "Answer ready.", nil, 0)
	flush()
	return p.toResult(state), nil
}

func (p *Pipeline) toResult(state *PipelineState) *PipelineResult {
	result := &PipelineResult{
		Analysis:  state.Analysis,
		Retrieved: state.Retrieved,
		Synthesis: state.Synthesis,
		Progress:  state.Progress,
	}
	if state.Err != nil {
		result.Error = state.Err.Error()
	}
	return result
}

func applyNoResultsFallback(state *PipelineState) {
	state.Synthesis = SynthesisResult{
		Answer:    noResultsAnswer,
		Citations: []Citation{},
	}
	state.AddProgress(StageDone, "No relevant documents were found. Returning a fallback answer.", nil, 0)
}

func applyOffTopicFallback(state *PipelineState) {
	answer := strings.TrimSpace(state.Analysis.OffTopicReason)
	if answer == "" {
		answer = "Please ask a question related to AI Act compliance, risk assessment, or technical documentation."
	}
	state.Err = nil
	state.Synthesis = SynthesisResult{
		Answer:    answer,
		Citations: []Citation{},
	}
	state.AddProgress(StageDone, "Returning a clarification response.", nil, 0)
}

func generateNoResultsFallback(
	ctx context.Context,
	state *PipelineState,
	client llm.Client,
	onDelta func(token string),
) {
	state.Err = nil
	state.AddProgress(StageSynthesizing, "No relevant documents found. Falling back to general chat...", nil, 0)

	var historyContext strings.Builder
	if len(state.Input.History) > 0 {
		start := 0
		if len(state.Input.History) > 6 {
			start = len(state.Input.History) - 6
		}
		for _, msg := range state.Input.History[start:] {
			fmt.Fprintf(&historyContext, "%s: %s\n", msg.Role, msg.Content)
		}
	}

	messages := []llm.Message{
		{
			Role:    "system",
			Content: "You are a helpful general-purpose assistant. Answer naturally and directly. Do not claim to have used documents or citations when none were retrieved.",
		},
	}
	if historyContext.Len() > 0 {
		messages = append(messages, llm.Message{
			Role:    "system",
			Content: "Recent conversation history:\n" + historyContext.String(),
		})
	}
	messages = append(messages, llm.Message{
		Role:    "user",
		Content: state.Input.Query,
	})

	var answerBuilder strings.Builder
	resp, err := client.StreamComplete(ctx, llm.CompletionRequest{
		Messages:    messages,
		Temperature: 0.4,
		MaxTokens:   1024,
	}, func(token string) {
		answerBuilder.WriteString(token)
		if onDelta != nil {
			onDelta(token)
		}
	})
	if err != nil {
		applyNoResultsFallback(state)
		return
	}

	answer := answerBuilder.String()
	if answer == "" {
		answer = resp.Content
	}
	if answer == "" {
		applyNoResultsFallback(state)
		return
	}

	state.Synthesis = SynthesisResult{
		Answer:    answer,
		Citations: []Citation{},
	}
	state.AddProgress(StageDone, "Answer ready.", nil, 0)
}
