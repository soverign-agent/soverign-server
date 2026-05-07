package metrics

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewHandler_ServesPrometheusFormat(t *testing.T) {
	reg := NewRegistry()
	rec, err := NewRecorder(reg)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}
	rec.AddPromptTokens("tenant1", "gpt-4", "openai", 42)
	rec.ObserveTTFT("tenant1", "gpt-4", "openai", 0.25)
	rec.IncSafetyViolation("tenant1", "gpt-4", "bias")

	h := NewHandler(reg)
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") && !strings.Contains(ct, "openmetrics-text") {
		t.Errorf("unexpected content-type %q", ct)
	}
	body := w.Body.String()
	wantSubstrings := []string{
		"ai_inference_prompt_tokens_total",
		"ai_inference_ttft_seconds",
		"ai_safety_violations_total",
		`tenant_id="tenant1"`,
		`model="gpt-4"`,
		`provider="openai"`,
	}
	for _, s := range wantSubstrings {
		if !strings.Contains(body, s) {
			t.Errorf("body missing %q\nbody:\n%s", s, body)
		}
	}
}

func TestNewHandler_NegotiatesOpenMetrics(t *testing.T) {
	reg := NewRegistry()
	if _, err := NewRecorder(reg); err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}
	h := NewHandler(reg)
	req := httptest.NewRequest("GET", "/metrics", nil)
	req.Header.Set("Accept", "application/openmetrics-text")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Header().Get("Content-Type"), "openmetrics-text") {
		t.Errorf("expected openmetrics negotiation, got %q", w.Header().Get("Content-Type"))
	}
}
