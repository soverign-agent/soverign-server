package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"sovereign-ai-compliance/shared/tenant"
)

const sseMaxBuffer = 1024 * 1024

type openAIStreamRequest struct {
	Model         string         `json:"model"`
	Messages      []message      `json:"messages"`
	Temperature   float64        `json:"temperature,omitempty"`
	MaxTokens     int            `json:"max_tokens,omitempty"`
	TopP          float64        `json:"top_p,omitempty"`
	Stream        bool           `json:"stream"`
	StreamOptions *streamOptions `json:"stream_options,omitempty"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type openAIStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
			Role    string `json:"role"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type openAIStreamError struct {
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// StreamComplete implements Client.StreamComplete for OpenAI.
func (c *OpenAIClient) StreamComplete(ctx context.Context, req CompletionRequest, onDelta func(token string)) (StreamCompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = c.model
	}

	tenantID := resolveTenantID(ctx)
	requestStart := time.Now()

	resp, err := c.openStreamingResponse(ctx, model, req)
	if err != nil {
		c.recordError(tenantID, model, requestStart)
		return StreamCompletionResponse{}, err
	}
	defer resp.Body.Close()

	result, err := c.consumeStream(resp.Body, onDelta, requestStart)
	if err != nil {
		c.recordError(tenantID, model, requestStart)
		return result, err
	}

	c.recordSuccess(tenantID, model, result)
	return result, nil
}

func (c *OpenAIClient) openStreamingResponse(ctx context.Context, model string, req CompletionRequest) (*http.Response, error) {
	messages := make([]message, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = message{Role: msg.Role, Content: msg.Content}
	}

	body := openAIStreamRequest{
		Model:         model,
		Messages:      messages,
		Temperature:   req.Temperature,
		MaxTokens:     req.MaxTokens,
		TopP:          req.TopP,
		Stream:        true,
		StreamOptions: &streamOptions{IncludeUsage: true},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal stream request: %w", err)
	}

	url := fmt.Sprintf("%s/chat/completions", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create stream request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("stream request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		var parsed openAIStreamError
		_ = json.Unmarshal(errBody, &parsed)
		msg := strings.TrimSpace(string(errBody))
		if parsed.Error != nil && parsed.Error.Message != "" {
			msg = parsed.Error.Message
		}
		return nil, fmt.Errorf("openai streaming HTTP %d: %s", resp.StatusCode, msg)
	}
	return resp, nil
}

func (c *OpenAIClient) consumeStream(body io.Reader, onDelta func(string), requestStart time.Time) (StreamCompletionResponse, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, sseMaxBuffer), sseMaxBuffer)

	var (
		contentBuilder strings.Builder
		usage          Usage
		finishReason   string
		firstTokenAt   time.Time
		lastTokenAt    time.Time
	)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			break
		}
		var chunk openAIStreamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return buildResponse(contentBuilder.String(), usage, finishReason, requestStart, firstTokenAt, lastTokenAt),
				fmt.Errorf("decode stream chunk: %w", err)
		}

		now := time.Now()
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				if firstTokenAt.IsZero() {
					firstTokenAt = now
				}
				lastTokenAt = now
				contentBuilder.WriteString(choice.Delta.Content)
				if onDelta != nil {
					onDelta(choice.Delta.Content)
				}
			}
			if choice.FinishReason != "" {
				finishReason = choice.FinishReason
			}
		}
		if chunk.Usage != nil {
			usage = Usage{
				PromptTokens:     chunk.Usage.PromptTokens,
				CompletionTokens: chunk.Usage.CompletionTokens,
				TotalTokens:      chunk.Usage.TotalTokens,
			}
		}
	}

	if err := scanner.Err(); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return buildResponse(contentBuilder.String(), usage, finishReason, requestStart, firstTokenAt, lastTokenAt), err
		}
		return buildResponse(contentBuilder.String(), usage, finishReason, requestStart, firstTokenAt, lastTokenAt),
			fmt.Errorf("read stream: %w", err)
	}
	return buildResponse(contentBuilder.String(), usage, finishReason, requestStart, firstTokenAt, lastTokenAt), nil
}

func buildResponse(content string, usage Usage, finishReason string, requestStart, firstTokenAt, lastTokenAt time.Time) StreamCompletionResponse {
	if lastTokenAt.IsZero() {
		lastTokenAt = time.Now()
	}
	timings := StreamTimings{
		RequestStart:  requestStart,
		FirstTokenAt:  firstTokenAt,
		LastTokenAt:   lastTokenAt,
		TotalDuration: lastTokenAt.Sub(requestStart),
	}
	if !firstTokenAt.IsZero() {
		timings.TTFT = firstTokenAt.Sub(requestStart)
		if usage.CompletionTokens > 0 {
			timings.TPOT = lastTokenAt.Sub(firstTokenAt) / time.Duration(usage.CompletionTokens)
		}
	}
	return StreamCompletionResponse{
		Content:      content,
		Usage:        usage,
		FinishReason: finishReason,
		Timings:      timings,
	}
}

func (c *OpenAIClient) recordSuccess(tenantID, model string, r StreamCompletionResponse) {
	if c.recorder == nil {
		return
	}
	provider := string(ProviderOpenAI)
	c.recorder.ObserveTotalDuration(tenantID, model, provider, "success", r.Timings.TotalDuration.Seconds())
	if !r.Timings.FirstTokenAt.IsZero() {
		c.recorder.ObserveTTFT(tenantID, model, provider, r.Timings.TTFT.Seconds())
	}
	if r.Usage.CompletionTokens > 0 {
		c.recorder.ObserveTPOT(tenantID, model, provider, r.Timings.TPOT.Seconds())
	}
	if r.Usage.PromptTokens > 0 {
		c.recorder.AddPromptTokens(tenantID, model, provider, r.Usage.PromptTokens)
	}
	if r.Usage.CompletionTokens > 0 {
		c.recorder.AddCompletionTokens(tenantID, model, provider, r.Usage.CompletionTokens)
	}
}

func (c *OpenAIClient) recordError(tenantID, model string, requestStart time.Time) {
	if c.recorder == nil {
		return
	}
	c.recorder.ObserveTotalDuration(tenantID, model, string(ProviderOpenAI), "error", time.Since(requestStart).Seconds())
}

func resolveTenantID(ctx context.Context) string {
	if id, ok := tenant.FromContext(ctx); ok && id != "" {
		return id
	}
	return "unknown"
}
