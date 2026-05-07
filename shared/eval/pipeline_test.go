package eval

import (
	"context"
	"errors"
	"testing"

	"sovereign-ai-compliance/shared/metrics"

	dto "github.com/prometheus/client_model/go"
)

func mustPipeline(t *testing.T, rec *metrics.Recorder) *Pipeline {
	t.Helper()
	p, err := NewPipeline(rec, DefaultThresholds())
	if err != nil {
		t.Fatalf("NewPipeline: %v", err)
	}
	return p
}

func TestNewPipeline_NilRecorderOK(t *testing.T) {
	p := mustPipeline(t, nil)
	res, err := p.Evaluate(context.Background(), EvalInput{
		TenantID:   "t1",
		Model:      "gpt-4",
		OutputText: "Paris is the capital of France.",
		RAGContext: []string{"France's capital is Paris."},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if res == nil {
		t.Fatal("nil result with nil recorder")
	}
}

func TestEvaluate_ContextCancelled(t *testing.T) {
	p := mustPipeline(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := p.Evaluate(ctx, EvalInput{TenantID: "t", Model: "m", OutputText: "x"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestEvaluate_GroundedAnswer_LowHallucination(t *testing.T) {
	p := mustPipeline(t, nil)
	res, err := p.Evaluate(context.Background(), EvalInput{
		TenantID:   "t",
		Model:      "gpt-4",
		OutputText: "Paris is the capital of France.",
		RAGContext: []string{
			"France's capital is Paris.",
			"The Eiffel Tower is located in Paris.",
		},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if res.HallucinationRate >= 0.3 {
		t.Errorf("grounded answer hallucination = %v, want < 0.3", res.HallucinationRate)
	}
	for _, v := range res.Violations {
		if v.Category == "hallucination" {
			t.Errorf("unexpected hallucination violation: %+v", v)
		}
	}
}

func TestEvaluate_UnsupportedAnswer_HighHallucination(t *testing.T) {
	p := mustPipeline(t, nil)
	res, err := p.Evaluate(context.Background(), EvalInput{
		TenantID:   "t",
		Model:      "gpt-4",
		OutputText: "Paris is on Mars and was founded in 1850 by aliens visiting from Andromeda.",
		RAGContext: []string{
			"France's capital is Paris.",
			"Paris has 2 million people.",
		},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if res.HallucinationRate <= 0.5 {
		t.Errorf("unsupported answer hallucination = %v, want > 0.5", res.HallucinationRate)
	}
}

func TestEvaluate_BreachesEmitMetricsAndViolations(t *testing.T) {
	reg := metrics.NewRegistry()
	rec, err := metrics.NewRecorder(reg)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}
	p := mustPipeline(t, rec)

	res, err := p.Evaluate(context.Background(), EvalInput{
		TenantID:   "tenant-1",
		Model:      "gpt-4",
		OutputText: "kill yourself you scumbag, women belong in the kitchen, all muslims are terrorists, old people are useless, all jews are greedy",
		RAGContext: []string{"unrelated"},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if res.BiasScore <= 0 {
		t.Errorf("expected bias > 0, got %v", res.BiasScore)
	}
	if res.ToxicityScore <= 0 {
		t.Errorf("expected toxicity > 0, got %v", res.ToxicityScore)
	}

	wantCategories := map[string]bool{"bias": false, "toxicity": false}
	for _, v := range res.Violations {
		if _, ok := wantCategories[v.Category]; ok {
			wantCategories[v.Category] = true
		}
	}
	for cat, found := range wantCategories {
		if !found {
			t.Errorf("missing %q violation in %+v", cat, res.Violations)
		}
	}

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	gotHisto := map[string]uint64{}
	gotCounter := map[string]float64{}
	for _, mf := range mfs {
		switch mf.GetName() {
		case "ai_hallucination_rate", "ai_bias_score", "ai_toxicity_score":
			for _, m := range mf.Metric {
				if h := m.GetHistogram(); h != nil {
					gotHisto[mf.GetName()] += h.GetSampleCount()
				}
			}
		case "ai_safety_violations_total":
			for _, m := range mf.Metric {
				gotCounter[labelValue(m, "category")] += m.GetCounter().GetValue()
			}
		}
	}
	if gotHisto["ai_hallucination_rate"] != 1 {
		t.Errorf("hallucination samples = %d, want 1", gotHisto["ai_hallucination_rate"])
	}
	if gotHisto["ai_bias_score"] != 1 {
		t.Errorf("bias samples = %d, want 1", gotHisto["ai_bias_score"])
	}
	if gotHisto["ai_toxicity_score"] != 1 {
		t.Errorf("toxicity samples = %d, want 1", gotHisto["ai_toxicity_score"])
	}
	if gotCounter["bias"] < 1 {
		t.Errorf("bias violation counter = %v, want >= 1", gotCounter["bias"])
	}
	if gotCounter["toxicity"] < 1 {
		t.Errorf("toxicity violation counter = %v, want >= 1", gotCounter["toxicity"])
	}
}

func TestEvaluate_NoBreach_NoCounterIncrement(t *testing.T) {
	reg := metrics.NewRegistry()
	rec, err := metrics.NewRecorder(reg)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}
	p := mustPipeline(t, rec)

	_, err = p.Evaluate(context.Background(), EvalInput{
		TenantID:   "t",
		Model:      "m",
		OutputText: "Paris is the capital of France.",
		RAGContext: []string{"France's capital is Paris."},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() == "ai_safety_violations_total" {
			for _, m := range mf.Metric {
				if v := m.GetCounter().GetValue(); v > 0 {
					t.Errorf("unexpected violation counter increment: %s=%v", labelValue(m, "category"), v)
				}
			}
		}
	}
}

func TestDefaultThresholds(t *testing.T) {
	got := DefaultThresholds()
	if got.Hallucination != 0.6 || got.Bias != 0.5 || got.Toxicity != 0.4 {
		t.Errorf("defaults = %+v", got)
	}
}

func labelValue(m *dto.Metric, name string) string {
	for _, l := range m.GetLabel() {
		if l.GetName() == name {
			return l.GetValue()
		}
	}
	return ""
}
