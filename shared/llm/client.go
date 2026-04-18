// Package llm provides a provider-agnostic abstraction for LLM clients.
package llm

import (
	"context"
	"fmt"
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

// Client is the provider-agnostic interface for LLM operations.
type Client interface {
	// Complete sends a chat completion request and returns the response.
	Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)

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
// This is a factory that will delegate to provider-specific implementations
// when they are registered. For now it validates configuration.
func NewClient(cfg Config) (Client, error) {
	switch cfg.Provider {
	case ProviderOpenAI, ProviderAnthropic, ProviderCustom:
		// Valid providers; concrete implementations are wired at service startup.
		return nil, fmt.Errorf("provider %q not yet implemented: wire the concrete client at service startup", cfg.Provider)
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %q", cfg.Provider)
	}
}
