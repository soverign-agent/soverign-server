package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"go.uber.org/zap"

	"sovereign-ai-compliance/shared/metrics"
	"sovereign-ai-compliance/shared/tenant"
)

func newTestServer(t *testing.T, h http.HandlerFunc) (*OpenAIClient, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := NewOpenAIClient(Config{
		Provider: ProviderOpenAI,
		APIKey:   "test-key",
		BaseURL:  srv.URL,
		Model:    "gpt-4o-mini",
		Timeout:  5,
	}, zap.NewNop())
	return c, srv
}

func sseHandler(chunks []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		for _, c := range chunks {
			fmt.Fprint(w, c)
			if flusher != nil {
				flusher.Flush()
			}
			time.Sleep(2 * time.Millisecond)
		}
	}
}

func TestStreamComplete_HappyPath(t *testing.T) {
	chunks := []string{
		"data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n",
		"data: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\n",
		"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":5,\"completion_tokens\":2,\"total_tokens\":7}}\n\n",
		"data: [DONE]\n\n",
	}
	c, _ := newTestServer(t, sseHandler(chunks))

	var captured []string
	var mu sync.Mutex
	resp, err := c.StreamComplete(context.Background(), CompletionRequest{
		Model:    "gpt-4o-mini",
		Messages: []Message{{Role: "user", Content: "hi"}},
	}, func(tok string) {
		mu.Lock()
		captured = append(captured, tok)
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.Content != "Hello world" {
		t.Errorf("content=%q", resp.Content)
	}
	if resp.FinishReason != "stop" {
		t.Errorf("finish=%q", resp.FinishReason)
	}
	if resp.Usage.PromptTokens != 5 || resp.Usage.CompletionTokens != 2 {
		t.Errorf("usage=%+v", resp.Usage)
	}
	if resp.Timings.TTFT <= 0 {
		t.Errorf("TTFT=%v should be positive", resp.Timings.TTFT)
	}
	if resp.Timings.TPOT <= 0 {
		t.Errorf("TPOT=%v should be positive", resp.Timings.TPOT)
	}
	if resp.Timings.TotalDuration <= 0 {
		t.Errorf("TotalDuration=%v", resp.Timings.TotalDuration)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(captured) != 2 || captured[0] != "Hello" || captured[1] != " world" {
		t.Errorf("captured=%v", captured)
	}
}

func TestStreamComplete_NilCallback(t *testing.T) {
	chunks := []string{
		"data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n",
		"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1,\"total_tokens\":2}}\n\n",
		"data: [DONE]\n\n",
	}
	c, _ := newTestServer(t, sseHandler(chunks))

	resp, err := c.StreamComplete(context.Background(), CompletionRequest{
		Messages: []Message{{Role: "user", Content: "x"}},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.Content != "ok" {
		t.Errorf("got %q", resp.Content)
	}
}

func TestStreamComplete_HTTPError(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"boom","type":"server"}}`))
	})

	_, err := c.StreamComplete(context.Background(), CompletionRequest{
		Messages: []Message{{Role: "user", Content: "x"}},
	}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected 500 in error, got %v", err)
	}
}

func TestStreamComplete_MalformedJSON(t *testing.T) {
	chunks := []string{
		"data: {not-json\n\n",
	}
	c, _ := newTestServer(t, sseHandler(chunks))

	_, err := c.StreamComplete(context.Background(), CompletionRequest{
		Messages: []Message{{Role: "user", Content: "x"}},
	}, nil)
	if err == nil {
		t.Fatal("expected JSON parse error")
	}
}

func TestStreamComplete_ContextCancellation(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"a\"}}]}\n\n")
		if flusher != nil {
			flusher.Flush()
		}
		time.Sleep(500 * time.Millisecond)
	})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	_, err := c.StreamComplete(ctx, CompletionRequest{
		Messages: []Message{{Role: "user", Content: "x"}},
	}, nil)
	if err == nil {
		t.Fatal("expected ctx error")
	}
}

func TestStreamComplete_EmptyStream(t *testing.T) {
	chunks := []string{
		"data: [DONE]\n\n",
	}
	c, _ := newTestServer(t, sseHandler(chunks))

	resp, err := c.StreamComplete(context.Background(), CompletionRequest{
		Messages: []Message{{Role: "user", Content: "x"}},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.Content != "" {
		t.Errorf("expected empty content, got %q", resp.Content)
	}
	if resp.Usage.CompletionTokens != 0 {
		t.Errorf("expected 0 completion tokens, got %d", resp.Usage.CompletionTokens)
	}
	if resp.Timings.TPOT != 0 {
		t.Errorf("expected zero TPOT for empty stream, got %v", resp.Timings.TPOT)
	}
}

func TestStreamComplete_RecordsMetrics(t *testing.T) {
	chunks := []string{
		"data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n",
		"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":1,\"total_tokens\":4}}\n\n",
		"data: [DONE]\n\n",
	}
	srv := httptest.NewServer(sseHandler(chunks))
	t.Cleanup(srv.Close)

	reg := metrics.NewRegistry()
	rec, err := metrics.NewRecorder(reg)
	if err != nil {
		t.Fatalf("recorder: %v", err)
	}

	c := NewOpenAIClientWithRecorder(Config{
		Provider: ProviderOpenAI,
		APIKey:   "test-key",
		BaseURL:  srv.URL,
		Model:    "gpt-4o-mini",
	}, zap.NewNop(), rec)

	ctx := tenant.WithContext(context.Background(), "tenant-A")
	_, err = c.StreamComplete(ctx, CompletionRequest{
		Messages: []Message{{Role: "user", Content: "x"}},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	if got := sumCounter(mfs, "ai_inference_prompt_tokens_total"); got != 3 {
		t.Errorf("prompt tokens = %v, want 3", got)
	}
	if got := sumCounter(mfs, "ai_inference_completion_tokens_total"); got != 1 {
		t.Errorf("completion tokens = %v, want 1", got)
	}
	if got := histogramSampleCount(mfs, "ai_inference_ttft_seconds"); got != 1 {
		t.Errorf("TTFT sample count = %d, want 1", got)
	}
	if got := histogramSampleCount(mfs, "ai_inference_total_duration_seconds"); got != 1 {
		t.Errorf("total duration sample count = %d, want 1", got)
	}
}

func TestStreamComplete_ErrorRecordsErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad"}}`))
	}))
	t.Cleanup(srv.Close)

	reg := metrics.NewRegistry()
	rec, _ := metrics.NewRecorder(reg)
	c := NewOpenAIClientWithRecorder(Config{
		APIKey:  "k",
		BaseURL: srv.URL,
		Model:   "m",
	}, zap.NewNop(), rec)

	_, err := c.StreamComplete(context.Background(), CompletionRequest{
		Messages: []Message{{Role: "user", Content: "x"}},
	}, nil)
	if err == nil {
		t.Fatal("expected error")
	}

	mfs, _ := reg.Gather()
	for _, mf := range mfs {
		if mf.GetName() != "ai_inference_total_duration_seconds" {
			continue
		}
		for _, m := range mf.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == "status" && l.GetValue() == "error" {
					return
				}
			}
		}
	}
	t.Error("expected total_duration metric with status=error")
}

func sumCounter(mfs []*dto.MetricFamily, name string) float64 {
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		var sum float64
		for _, m := range mf.GetMetric() {
			if c := m.GetCounter(); c != nil {
				sum += c.GetValue()
			}
		}
		return sum
	}
	return 0
}

func histogramSampleCount(mfs []*dto.MetricFamily, name string) uint64 {
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		var total uint64
		for _, m := range mf.GetMetric() {
			if h := m.GetHistogram(); h != nil {
				total += h.GetSampleCount()
			}
		}
		return total
	}
	return 0
}
