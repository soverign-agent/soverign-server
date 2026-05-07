package metrics

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestNewRegistry_ReturnsIsolatedRegistry(t *testing.T) {
	r1 := NewRegistry()
	r2 := NewRegistry()
	if r1 == nil || r2 == nil {
		t.Fatal("NewRegistry returned nil")
	}
	if r1 == r2 {
		t.Fatal("NewRegistry returned the same instance twice")
	}
}

func TestNewRecorder_RegistersAllCollectors(t *testing.T) {
	reg := NewRegistry()
	rec, err := NewRecorder(reg)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}
	if rec == nil {
		t.Fatal("NewRecorder returned nil")
	}

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}

	want := []string{
		"ai_inference_ttft_seconds",
		"ai_inference_tpot_seconds",
		"ai_inference_total_duration_seconds",
		"ai_inference_prompt_tokens_total",
		"ai_inference_completion_tokens_total",
		"ai_hallucination_rate",
		"ai_bias_score",
		"ai_toxicity_score",
		"ai_safety_violations_total",
	}
	rec.AddPromptTokens("t", "m", "p", 1)
	rec.AddCompletionTokens("t", "m", "p", 1)
	rec.IncSafetyViolation("t", "m", "bias")
	rec.ObserveTTFT("t", "m", "p", 0.1)
	rec.ObserveTPOT("t", "m", "p", 0.01)
	rec.ObserveTotalDuration("t", "m", "p", "ok", 1)
	rec.ObserveHallucinationRate("t", "m", "rag", 0.2)
	rec.ObserveBiasScore("t", "m", 0.3)
	rec.ObserveToxicityScore("t", "m", 0.4)

	mfs, err = reg.Gather()
	if err != nil {
		t.Fatalf("Gather post-record: %v", err)
	}
	got := map[string]bool{}
	for _, mf := range mfs {
		got[mf.GetName()] = true
	}
	for _, name := range want {
		if !got[name] {
			t.Errorf("expected metric %q registered, found: %v", name, got)
		}
	}
}

func TestNewRecorder_NilRegistry(t *testing.T) {
	if _, err := NewRecorder(nil); err == nil {
		t.Fatal("expected error for nil registry")
	}
}

func TestNewRecorder_DoubleRegistrationFails(t *testing.T) {
	reg := NewRegistry()
	if _, err := NewRecorder(reg); err != nil {
		t.Fatalf("first NewRecorder: %v", err)
	}
	_, err := NewRecorder(reg)
	if err == nil {
		t.Fatal("expected duplicate registration to fail")
	}
	if !strings.Contains(err.Error(), "register collector") {
		t.Errorf("expected wrapped error, got %v", err)
	}
}

func TestNewRecorder_RegistrationConflictWithExternalCollector(t *testing.T) {
	reg := NewRegistry()
	conflict := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ai_inference_prompt_tokens_total",
		Help: "duplicate",
	})
	if err := reg.Register(conflict); err != nil {
		t.Fatalf("seed register: %v", err)
	}
	if _, err := NewRecorder(reg); err == nil {
		t.Fatal("expected conflict error")
	}
}
