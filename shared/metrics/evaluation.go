package metrics

import "github.com/prometheus/client_golang/prometheus"

var scoreBuckets = []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0}

func newHallucinationHistogram() *prometheus.HistogramVec {
	return prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ai_hallucination_rate",
		Help:    "Hallucination rate of an LLM response, normalized to 0..1.",
		Buckets: scoreBuckets,
	}, []string{LabelTenantID, LabelModel, LabelEvalType})
}

func newBiasHistogram() *prometheus.HistogramVec {
	return prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ai_bias_score",
		Help:    "Bias score of an LLM response, normalized to 0..1.",
		Buckets: scoreBuckets,
	}, []string{LabelTenantID, LabelModel})
}

func newToxicityHistogram() *prometheus.HistogramVec {
	return prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ai_toxicity_score",
		Help:    "Toxicity score of an LLM response, normalized to 0..1.",
		Buckets: scoreBuckets,
	}, []string{LabelTenantID, LabelModel})
}

func newSafetyViolationCounter() *prometheus.CounterVec {
	return prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ai_safety_violations_total",
		Help: "Total number of safety violations detected, partitioned by category.",
	}, []string{LabelTenantID, LabelModel, LabelCategory})
}

func (r *Recorder) ObserveHallucinationRate(tenantID, model, evalType string, rate float64) {
	r.hallucinationRate.WithLabelValues(tenantID, model, evalType).Observe(rate)
}

func (r *Recorder) ObserveBiasScore(tenantID, model string, score float64) {
	r.biasScore.WithLabelValues(tenantID, model).Observe(score)
}

func (r *Recorder) ObserveToxicityScore(tenantID, model string, score float64) {
	r.toxicityScore.WithLabelValues(tenantID, model).Observe(score)
}

func (r *Recorder) IncSafetyViolation(tenantID, model, category string) {
	r.safetyViolations.WithLabelValues(tenantID, model, category).Inc()
}
