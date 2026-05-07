// Package metrics is the shared Prometheus instrumentation surface for the
// Sovereign AI Compliance platform.
//
// It exposes a single concrete type, *Recorder, that bundles every AI-specific
// metric (inference latency, token consumption, evaluation scores, safety
// violations) defined by milestone M18. All metric handles are registered to a
// caller-supplied *prometheus.Registry so services stay isolated for tests and
// can be wired together with explicit dependency injection.
//
// # Public API
//
//	func NewRegistry() *prometheus.Registry
//	func NewRecorder(reg *prometheus.Registry) (*Recorder, error)
//	func NewHandler(reg *prometheus.Registry) http.Handler
//
//	type Recorder struct{ /* unexported */ }
//
//	// Inference metrics — call from the streaming LLM client.
//	func (r *Recorder) ObserveTTFT(tenantID, model, provider string, seconds float64)
//	func (r *Recorder) ObserveTPOT(tenantID, model, provider string, seconds float64)
//	func (r *Recorder) ObserveTotalDuration(tenantID, model, provider, status string, seconds float64)
//	func (r *Recorder) AddPromptTokens(tenantID, model, provider string, tokens int)
//	func (r *Recorder) AddCompletionTokens(tenantID, model, provider string, tokens int)
//
//	// Evaluation metrics — call from the eval pipeline.
//	func (r *Recorder) ObserveHallucinationRate(tenantID, model, evalType string, rate float64)
//	func (r *Recorder) ObserveBiasScore(tenantID, model string, score float64)
//	func (r *Recorder) ObserveToxicityScore(tenantID, model string, score float64)
//	func (r *Recorder) IncSafetyViolation(tenantID, model, category string)
//
// # Metric catalogue
//
//	| Name                                   | Type      | Labels                              |
//	|----------------------------------------|-----------|-------------------------------------|
//	| ai_inference_ttft_seconds              | Histogram | tenant_id, model, provider          |
//	| ai_inference_tpot_seconds              | Histogram | tenant_id, model, provider          |
//	| ai_inference_total_duration_seconds    | Histogram | tenant_id, model, provider, status  |
//	| ai_inference_prompt_tokens_total       | Counter   | tenant_id, model, provider          |
//	| ai_inference_completion_tokens_total   | Counter   | tenant_id, model, provider          |
//	| ai_hallucination_rate                  | Histogram | tenant_id, model, eval_type         |
//	| ai_bias_score                          | Histogram | tenant_id, model                    |
//	| ai_toxicity_score                      | Histogram | tenant_id, model                    |
//	| ai_safety_violations_total             | Counter   | tenant_id, model, category          |
//
// Histogram buckets are tuned for LLM latency (TTFT, TPOT, total duration) and
// for normalized 0..1 quality scores (hallucination, bias, toxicity).
//
// # Wiring
//
//	reg := metrics.NewRegistry()
//	rec, err := metrics.NewRecorder(reg)
//	if err != nil {
//	    return fmt.Errorf("init metrics: %w", err)
//	}
//	mux := http.NewServeMux()
//	mux.Handle("/metrics", metrics.NewHandler(reg))
//	// pass rec to llm.NewStreamingClient(..., rec) and eval.NewPipeline(..., rec)
//
// Do NOT add a global instance. Always inject *Recorder via constructors so that
// tests can create an isolated registry per case and run with -race.
package metrics
