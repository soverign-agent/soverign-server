// Package tests contains end-to-end smoke tests for the M18 data path.
package tests

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"go.uber.org/zap"

	"sovereign-ai-compliance/agent-orchestrator/internal/orchestrator"
	sharedconfig "sovereign-ai-compliance/shared/config"
	"sovereign-ai-compliance/shared/eval"
	"sovereign-ai-compliance/shared/llm"
	"sovereign-ai-compliance/shared/metrics"
	agentv1 "sovereign-ai-compliance/shared/proto/agent/v1"
	"sovereign-ai-compliance/shared/tenant"
)

// TestSmoke_EndToEndMetrics verifies that a synthetic streaming LLM call through
// the supervisor produces all inference and evaluation metrics in the Prometheus
// registry.  This test is the automated equivalent of the Phase C smoke test.
func TestSmoke_EndToEndMetrics(t *testing.T) {
	ctx := tenant.WithContext(context.Background(), "tenant-smoke")

	reg := metrics.NewRegistry()
	rec, err := metrics.NewRecorder(reg)
	if err != nil {
		t.Fatalf("init recorder: %v", err)
	}

	evalPipeline, err := eval.NewPipeline(rec, eval.DefaultThresholds())
	if err != nil {
		t.Fatalf("init eval pipeline: %v", err)
	}

	// Mock OpenAI SSE endpoint that returns a short completion.
	mockOpenAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		chunks := []string{
			"data: {\"choices\":[{\"delta\":{\"content\":\"The EU AI Act\"}}]}\n\n",
			"data: {\"choices\":[{\"delta\":{\"content\":\" requires transparency.\"}}]}\n\n",
			"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":4,\"total_tokens\":14}}\n\n",
			"data: [DONE]\n\n",
		}
		for _, c := range chunks {
			fmt.Fprint(w, c)
			if flusher != nil {
				flusher.Flush()
			}
			time.Sleep(2 * time.Millisecond)
		}
	}))
	defer mockOpenAI.Close()

	llmClient := llm.NewOpenAIClientWithRecorder(llm.Config{
		Provider: llm.ProviderOpenAI,
		APIKey:   "smoke-key",
		BaseURL:  mockOpenAI.URL,
		Model:    "gpt-4o",
		Timeout:  30,
	}, zap.NewNop(), rec)

	sup := orchestrator.NewSupervisor(nil, zap.NewNop(), sharedconfig.LLMConfig{Model: "gpt-4o"}, llmClient, evalPipeline, nil)

	req := &agentv1.AgentRequest{
		RequestId: "smoke-req-1",
		AgentId:   "smoke-agent",
		TenantId:  "tenant-smoke",
		Payload:   []byte(`{"messages":[{"role":"user","content":"Explain EU AI Act"}]}`),
	}
	resp, err := sup.Invoke(ctx, req)
	if err != nil {
		t.Fatalf("invoke failed: %v", err)
	}
	if resp.GetStatus() != agentv1.AgentResponse_STATUS_COMPLETED {
		t.Fatalf("unexpected status: %v msg=%q", resp.GetStatus(), resp.GetErrorMessage())
	}
	if !strings.Contains(string(resp.GetResult()), "EU AI Act") {
		t.Errorf("unexpected result: %q", resp.GetResult())
	}

	// Gather metrics directly from the registry (avoids text-parse validation quirks).
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	mf := metricFamilyMap(mfs)

	// ---- Inference metrics ----
	requireCounter(t, mf, "ai_inference_prompt_tokens_total", "tenant-smoke", "gpt-4o", 10)
	requireCounter(t, mf, "ai_inference_completion_tokens_total", "tenant-smoke", "gpt-4o", 4)
	requireHistogramSampleCount(t, mf, "ai_inference_ttft_seconds", "tenant-smoke", "gpt-4o", 1)
	requireHistogramSampleCount(t, mf, "ai_inference_tpot_seconds", "tenant-smoke", "gpt-4o", 1)
	requireHistogramSampleCount(t, mf, "ai_inference_total_duration_seconds", "tenant-smoke", "gpt-4o", 1)

	// ---- Evaluation metrics ----
	requireHistogramSampleCount(t, mf, "ai_hallucination_rate", "tenant-smoke", "gpt-4o", 1)
	requireHistogramSampleCount(t, mf, "ai_bias_score", "tenant-smoke", "gpt-4o", 1)
	requireHistogramSampleCount(t, mf, "ai_toxicity_score", "tenant-smoke", "gpt-4o", 1)
}

func metricFamilyMap(mfs []*dto.MetricFamily) map[string]*dto.MetricFamily {
	m := make(map[string]*dto.MetricFamily, len(mfs))
	for _, mf := range mfs {
		m[mf.GetName()] = mf
	}
	return m
}

func requireCounter(t *testing.T, mf map[string]*dto.MetricFamily, name, tenantID, model string, wantValue float64) {
	t.Helper()
	family, ok := mf[name]
	if !ok {
		t.Fatalf("metric %q not found", name)
	}
	for _, m := range family.GetMetric() {
		labels := labelMap(m.GetLabel())
		if labels["tenant_id"] == tenantID && labels["model"] == model {
			if m.GetCounter() == nil {
				t.Fatalf("metric %q is not a counter", name)
			}
			if m.GetCounter().GetValue() != wantValue {
				t.Errorf("metric %q[%s,%s]=%v want %v", name, tenantID, model, m.GetCounter().GetValue(), wantValue)
			}
			return
		}
	}
	t.Fatalf("metric %q not found with tenant_id=%s model=%s", name, tenantID, model)
}

func requireHistogramSampleCount(t *testing.T, mf map[string]*dto.MetricFamily, name, tenantID, model string, wantCount uint64) {
	t.Helper()
	family, ok := mf[name]
	if !ok {
		t.Fatalf("metric %q not found", name)
	}
	for _, m := range family.GetMetric() {
		labels := labelMap(m.GetLabel())
		if labels["tenant_id"] == tenantID && labels["model"] == model {
			if m.GetHistogram() == nil {
				t.Fatalf("metric %q is not a histogram", name)
			}
			if m.GetHistogram().GetSampleCount() != wantCount {
				t.Errorf("metric %q[%s,%s] count=%d want %d", name, tenantID, model, m.GetHistogram().GetSampleCount(), wantCount)
			}
			return
		}
	}
	t.Fatalf("metric %q not found with tenant_id=%s model=%s", name, tenantID, model)
}

func labelMap(labels []*dto.LabelPair) map[string]string {
	m := make(map[string]string, len(labels))
	for _, l := range labels {
		m[l.GetName()] = l.GetValue()
	}
	return m
}
