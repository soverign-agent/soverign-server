package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	ttftBuckets          = []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 30}
	tpotBuckets          = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5}
	totalDurationBuckets = []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120}
)

func newTTFTHistogram() *prometheus.HistogramVec {
	return prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ai_inference_ttft_seconds",
		Help:    "Time to first token for streaming LLM inference, in seconds.",
		Buckets: ttftBuckets,
	}, []string{LabelTenantID, LabelModel, LabelProvider})
}

func newTPOTHistogram() *prometheus.HistogramVec {
	return prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ai_inference_tpot_seconds",
		Help:    "Time per output token for streaming LLM inference, in seconds.",
		Buckets: tpotBuckets,
	}, []string{LabelTenantID, LabelModel, LabelProvider})
}

func newTotalDurationHistogram() *prometheus.HistogramVec {
	return prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ai_inference_total_duration_seconds",
		Help:    "Total wall-clock duration of an LLM inference call, in seconds.",
		Buckets: totalDurationBuckets,
	}, []string{LabelTenantID, LabelModel, LabelProvider, LabelStatus})
}

func newPromptTokenCounter() *prometheus.CounterVec {
	return prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ai_inference_prompt_tokens_total",
		Help: "Total number of prompt tokens consumed by LLM inference.",
	}, []string{LabelTenantID, LabelModel, LabelProvider})
}

func newCompletionTokenCounter() *prometheus.CounterVec {
	return prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ai_inference_completion_tokens_total",
		Help: "Total number of completion tokens produced by LLM inference.",
	}, []string{LabelTenantID, LabelModel, LabelProvider})
}

func (r *Recorder) ObserveTTFT(tenantID, model, provider string, seconds float64) {
	r.ttftSeconds.WithLabelValues(tenantID, model, provider).Observe(seconds)
}

func (r *Recorder) ObserveTPOT(tenantID, model, provider string, seconds float64) {
	r.tpotSeconds.WithLabelValues(tenantID, model, provider).Observe(seconds)
}

func (r *Recorder) ObserveTotalDuration(tenantID, model, provider, status string, seconds float64) {
	r.totalDurationSeconds.WithLabelValues(tenantID, model, provider, status).Observe(seconds)
}

func (r *Recorder) AddPromptTokens(tenantID, model, provider string, tokens int) {
	r.promptTokens.WithLabelValues(tenantID, model, provider).Add(float64(tokens))
}

func (r *Recorder) AddCompletionTokens(tenantID, model, provider string, tokens int) {
	r.completionTokens.WithLabelValues(tenantID, model, provider).Add(float64(tokens))
}
