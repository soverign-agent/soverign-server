// Package llm provides a provider-agnostic abstraction for LLM clients.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	"sovereign-ai-compliance/shared/metrics"
)

// OpenAIClient implements Client for OpenAI API.
type OpenAIClient struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
	logger     *zap.Logger
	recorder   *metrics.Recorder
}

// NewOpenAIClient creates a new OpenAI client without metric recording.
func NewOpenAIClient(cfg Config, logger *zap.Logger) *OpenAIClient {
	return NewOpenAIClientWithRecorder(cfg, logger, nil)
}

// NewOpenAIClientWithRecorder creates an OpenAI client that emits Prometheus
// metrics through the supplied Recorder. Passing nil disables metric emission
// (useful for tests and tooling that does not need observability).
func NewOpenAIClientWithRecorder(cfg Config, logger *zap.Logger, rec *metrics.Recorder) *OpenAIClient {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30
	}

	return &OpenAIClient{
		apiKey:  cfg.APIKey,
		baseURL: baseURL,
		model:   cfg.Model,
		httpClient: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
		logger:   logger,
		recorder: rec,
	}
}

// openAIEmbeddingRequest is the request body for OpenAI embeddings API.
type openAIEmbeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

// openAIEmbeddingResponse is the response body from OpenAI embeddings API.
type openAIEmbeddingResponse struct {
	Object string `json:"object"`
	Model  string `json:"model"`
	Data   []struct {
		Object    string    `json:"object"`
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// Embed implements Client.Embed for OpenAI.
func (c *OpenAIClient) Embed(ctx context.Context, req EmbeddingRequest) (EmbeddingResponse, error) {
	model := req.Model
	if model == "" {
		model = c.model
	}

	requestBody := openAIEmbeddingRequest{
		Model: model,
		Input: req.Input,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return EmbeddingResponse{}, fmt.Errorf("failed to marshal embedding request: %w", err)
	}

	url := fmt.Sprintf("%s/embeddings", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return EmbeddingResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return EmbeddingResponse{}, fmt.Errorf("embedding request failed: %w", err)
	}
	defer resp.Body.Close()

	var result openAIEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return EmbeddingResponse{}, fmt.Errorf("failed to decode embedding response: %w", err)
	}

	if result.Error != nil {
		return EmbeddingResponse{}, fmt.Errorf("openai API error: %s - %s", result.Error.Type, result.Error.Message)
	}

	if len(result.Data) == 0 {
		return EmbeddingResponse{}, fmt.Errorf("no embedding data returned from OpenAI")
	}

	return EmbeddingResponse{
		Embedding: result.Data[0].Embedding,
		Usage: Usage{
			PromptTokens: result.Usage.PromptTokens,
			TotalTokens:  result.Usage.TotalTokens,
		},
	}, nil
}

// openAICompletionRequest is the request body for OpenAI chat completions API.
type openAICompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []message `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	TopP        float64   `json:"top_p,omitempty"`
	Stream      bool      `json:"stream"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openAICompletionResponse is the response body from OpenAI chat completions API.
type openAICompletionResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int     `json:"index"`
		Message      message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// Complete implements Client.Complete for OpenAI.
func (c *OpenAIClient) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = c.model
	}

	messages := make([]message, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	requestBody := openAICompletionRequest{
		Model:       model,
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		TopP:        req.TopP,
		Stream:      false, // We don't support streaming yet
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("failed to marshal completion request: %w", err)
	}

	url := fmt.Sprintf("%s/chat/completions", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("completion request failed: %w", err)
	}
	defer resp.Body.Close()

	var result openAICompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return CompletionResponse{}, fmt.Errorf("failed to decode completion response: %w", err)
	}

	if result.Error != nil {
		return CompletionResponse{}, fmt.Errorf("openai API error: %s - %s", result.Error.Type, result.Error.Message)
	}

	if len(result.Choices) == 0 {
		return CompletionResponse{}, fmt.Errorf("no completion choices returned from OpenAI")
	}

	choice := result.Choices[0]

	return CompletionResponse{
		Content:      choice.Message.Content,
		FinishReason: choice.FinishReason,
		Usage: Usage{
			PromptTokens:     result.Usage.PromptTokens,
			CompletionTokens: result.Usage.CompletionTokens,
			TotalTokens:      result.Usage.TotalTokens,
		},
	}, nil
}

// Health checks if the OpenAI API is reachable.
func (c *OpenAIClient) Health(ctx context.Context) error {
	// Simple health check by listing models (doesn't require heavy payload)
	url := fmt.Sprintf("%s/models", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}

	return nil
}
