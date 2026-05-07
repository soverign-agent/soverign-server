// Package llm provides a provider-agnostic abstraction for LLM clients.
package llm

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// Message represents a single message in a chat conversation.
type Message struct {
	Role    string // system, user, assistant
	Content string
}

// CompletionRequest holds parameters for a chat completion request.
type CompletionRequest struct {
	Model       string
	Messages    []Message
	Temperature float64
	MaxTokens   int
	TopP        float64
	Stream      bool
}

// CompletionResponse holds the result of a chat completion request.
type CompletionResponse struct {
	Content      string
	Usage        Usage
	FinishReason string
}

// Usage tracks token consumption for a completion request.
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// EmbeddingRequest holds parameters for an embedding request.
type EmbeddingRequest struct {
	Model string
	Input string
}

// EmbeddingResponse holds the result of an embedding request.
type EmbeddingResponse struct {
	Embedding []float32
	Usage     Usage
}

// StreamCompletionResponse is the aggregated result of a streaming chat completion,
// including timing data captured during the stream.
type StreamCompletionResponse struct {
	Content      string
	Usage        Usage
	FinishReason string
	Timings      StreamTimings
}

// StreamTimings captures wall-clock timestamps for a streaming completion. TTFT
// is the time from request start until the first non-empty content delta. TPOT
// is the average time per output token measured between the first and last
// delta. TotalDuration is end-to-end.
type StreamTimings struct {
	RequestStart  time.Time
	FirstTokenAt  time.Time
	LastTokenAt   time.Time
	TTFT          time.Duration
	TPOT          time.Duration
	TotalDuration time.Duration
}

// Client is the provider-agnostic interface for LLM operations.
type Client interface {
	// Complete sends a chat completion request and returns the response.
	Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)

	// StreamComplete sends a streaming chat completion. Each non-empty content
	// delta is delivered to onDelta (which may be nil). Returns the aggregated
	// response with timing metadata.
	StreamComplete(ctx context.Context, req CompletionRequest, onDelta func(token string)) (StreamCompletionResponse, error)

	// Embed sends an embedding request and returns the vector representation.
	Embed(ctx context.Context, req EmbeddingRequest) (EmbeddingResponse, error)

	// Health checks whether the LLM provider is reachable and healthy.
	Health(ctx context.Context) error
}

// Provider enumerates supported LLM backends.
type Provider string

const (
	ProviderOpenAI    Provider = "openai"
	ProviderAnthropic Provider = "anthropic"
	ProviderCustom    Provider = "custom"
)

// Config holds LLM client construction parameters.
type Config struct {
	Provider    Provider
	APIKey      string
	BaseURL     string // optional override for custom or proxy endpoints
	Model       string
	Timeout     int // seconds
	MaxTokens   int
	Temperature float64
}

// NewClient creates a concrete Client based on the provided Config.
func NewClient(cfg Config, logger *zap.Logger) (Client, error) {
	switch cfg.Provider {
	case ProviderOpenAI:
		return NewOpenAIClient(cfg, logger), nil
	case ProviderAnthropic:
		return nil, fmt.Errorf("provider %q not yet implemented", cfg.Provider)
	case ProviderCustom:
		return nil, fmt.Errorf("custom provider requires explicit wiring at service startup")
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %q", cfg.Provider)
	}
}
