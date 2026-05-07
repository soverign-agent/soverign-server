package metrics

import (
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func newTestRecorder(t *testing.T) (*Recorder, *prometheus.Registry) {
	t.Helper()
	reg := NewRegistry()
	rec, err := NewRecorder(reg)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}
	return rec, reg
}

func TestInferenceCounters(t *testing.T) {
	tests := []struct {
		name    string
		record  func(r *Recorder)
		metric  string
		labels  []string
		wantSum float64
	}{
		{
			name: "prompt tokens accumulate",
			record: func(r *Recorder) {
				r.AddPromptTokens("tenant1", "gpt-4", "openai", 100)
				r.AddPromptTokens("tenant1", "gpt-4", "openai", 50)
			},
			metric:  "ai_inference_prompt_tokens_total",
			labels:  []string{"tenant1", "gpt-4", "openai"},
			wantSum: 150,
		},
		{
			name:    "completion tokens accumulate",
			record:  func(r *Recorder) { r.AddCompletionTokens("tenantA", "gpt-3.5", "openai", 25) },
			metric:  "ai_inference_completion_tokens_total",
			labels:  []string{"tenantA", "gpt-3.5", "openai"},
			wantSum: 25,
		},
		{
			name:    "zero tokens are recorded",
			record:  func(r *Recorder) { r.AddPromptTokens("t", "m", "p", 0) },
			metric:  "ai_inference_prompt_tokens_total",
			labels:  []string{"t", "m", "p"},
			wantSum: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec, _ := newTestRecorder(t)
			tc.record(rec)

			var counter prometheus.Counter
			switch tc.metric {
			case "ai_inference_prompt_tokens_total":
				counter = rec.promptTokens.WithLabelValues(tc.labels...)
			case "ai_inference_completion_tokens_total":
				counter = rec.completionTokens.WithLabelValues(tc.labels...)
			default:
				t.Fatalf("unknown metric %s", tc.metric)
			}
			got := testutil.ToFloat64(counter)
			if got != tc.wantSum {
				t.Errorf("%s sum = %v, want %v", tc.metric, got, tc.wantSum)
			}
		})
	}
}

func TestInferenceHistograms_RecordObservation(t *testing.T) {
	rec, reg := newTestRecorder(t)

	rec.ObserveTTFT("tenant1", "gpt-4", "openai", 0.3)
	rec.ObserveTPOT("tenant1", "gpt-4", "openai", 0.02)
	rec.ObserveTotalDuration("tenant1", "gpt-4", "openai", "success", 1.5)

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}

	want := map[string]uint64{
		"ai_inference_ttft_seconds":           1,
		"ai_inference_tpot_seconds":           1,
		"ai_inference_total_duration_seconds": 1,
	}
	for _, mf := range mfs {
		expected, ok := want[mf.GetName()]
		if !ok {
			continue
		}
		if len(mf.Metric) != 1 {
			t.Errorf("%s: expected 1 series, got %d", mf.GetName(), len(mf.Metric))
			continue
		}
		h := mf.Metric[0].GetHistogram()
		if h.GetSampleCount() != expected {
			t.Errorf("%s: sample count = %d, want %d", mf.GetName(), h.GetSampleCount(), expected)
		}
	}
}

func TestInferenceHistograms_StatusLabelDifferentiates(t *testing.T) {
	rec, reg := newTestRecorder(t)
	rec.ObserveTotalDuration("t", "m", "p", "success", 1)
	rec.ObserveTotalDuration("t", "m", "p", "error", 2)

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() != "ai_inference_total_duration_seconds" {
			continue
		}
		if len(mf.Metric) != 2 {
			t.Fatalf("expected 2 series for status label, got %d", len(mf.Metric))
		}
	}
}

func TestRecorder_ConcurrentAccess(t *testing.T) {
	rec, _ := newTestRecorder(t)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec.AddPromptTokens("t", "m", "p", 1)
			rec.AddCompletionTokens("t", "m", "p", 1)
			rec.ObserveTTFT("t", "m", "p", 0.1)
			rec.ObserveTPOT("t", "m", "p", 0.01)
			rec.ObserveTotalDuration("t", "m", "p", "ok", 0.5)
		}()
	}
	wg.Wait()

	got := testutil.ToFloat64(rec.promptTokens.WithLabelValues("t", "m", "p"))
	if got != 50 {
		t.Errorf("prompt tokens after concurrent adds = %v, want 50", got)
	}
}
