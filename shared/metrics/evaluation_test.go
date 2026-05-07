package metrics

import (
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestEvaluationHistograms(t *testing.T) {
	tests := []struct {
		name   string
		record func(r *Recorder)
		metric string
	}{
		{"hallucination", func(r *Recorder) { r.ObserveHallucinationRate("t", "m", "rag", 0.45) }, "ai_hallucination_rate"},
		{"bias", func(r *Recorder) { r.ObserveBiasScore("t", "m", 0.2) }, "ai_bias_score"},
		{"toxicity", func(r *Recorder) { r.ObserveToxicityScore("t", "m", 0.7) }, "ai_toxicity_score"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec, reg := newTestRecorder(t)
			tc.record(rec)
			mfs, err := reg.Gather()
			if err != nil {
				t.Fatalf("Gather: %v", err)
			}
			var found bool
			for _, mf := range mfs {
				if mf.GetName() != tc.metric {
					continue
				}
				if len(mf.Metric) != 1 {
					t.Fatalf("%s: expected 1 series, got %d", tc.metric, len(mf.Metric))
				}
				if mf.Metric[0].GetHistogram().GetSampleCount() != 1 {
					t.Errorf("%s: sample count = %d, want 1", tc.metric, mf.Metric[0].GetHistogram().GetSampleCount())
				}
				found = true
			}
			if !found {
				t.Errorf("metric %q not gathered", tc.metric)
			}
		})
	}
}

func TestSafetyViolationCounter(t *testing.T) {
	rec, _ := newTestRecorder(t)
	rec.IncSafetyViolation("tenant1", "gpt-4", "bias")
	rec.IncSafetyViolation("tenant1", "gpt-4", "bias")
	rec.IncSafetyViolation("tenant1", "gpt-4", "toxicity")

	got := testutil.ToFloat64(rec.safetyViolations.WithLabelValues("tenant1", "gpt-4", "bias"))
	if got != 2 {
		t.Errorf("bias violations = %v, want 2", got)
	}
	got = testutil.ToFloat64(rec.safetyViolations.WithLabelValues("tenant1", "gpt-4", "toxicity"))
	if got != 1 {
		t.Errorf("toxicity violations = %v, want 1", got)
	}
}

func TestEvaluation_ConcurrentAccess(t *testing.T) {
	rec, _ := newTestRecorder(t)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec.ObserveHallucinationRate("t", "m", "rag", 0.5)
			rec.ObserveBiasScore("t", "m", 0.5)
			rec.ObserveToxicityScore("t", "m", 0.5)
			rec.IncSafetyViolation("t", "m", "bias")
		}()
	}
	wg.Wait()
	got := testutil.ToFloat64(rec.safetyViolations.WithLabelValues("t", "m", "bias"))
	if got != 50 {
		t.Errorf("safety violations after concurrent inc = %v, want 50", got)
	}
}
