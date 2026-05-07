// Package eval provides a synchronous, in-process evaluation pipeline that
// scores an LLM completion for hallucination, bias, and toxicity, and emits
// the resulting Prometheus metrics through shared/metrics.
//
// The pipeline is deliberately rule-based and dependency-free: it relies on
// stdlib plus the shared metrics package only. Detection strategies are:
//
//   - Hallucination: bigram + unigram overlap between OutputText and the
//     provided RAGContext chunks. When no RAG context is supplied, the rate
//     is reported as 0 (we cannot disprove an answer without a ground truth).
//   - Bias: lexicon-based phrase matching with a 5-token negation window.
//   - Toxicity: lexicon term + threat regex matching.
//
// # Public API
//
//	type EvalInput struct{ TenantID, Model, OutputText string; RAGContext []string }
//	type EvalResult struct{ HallucinationRate, BiasScore, ToxicityScore float64; Violations []Violation }
//	type Violation  struct{ Category, Detail string }
//	type Thresholds struct{ Hallucination, Bias, Toxicity float64 }
//
//	func DefaultThresholds() Thresholds
//	func NewPipeline(rec *metrics.Recorder, thresholds Thresholds) (*Pipeline, error)
//	func (p *Pipeline) Evaluate(ctx context.Context, in EvalInput) (*EvalResult, error)
//
// The *metrics.Recorder argument may be nil so that unit tests can exercise
// the scoring logic without a Prometheus registry.
//
// # Limitations
//
// English-only, lexicon-driven detection. The lexicons are intentionally
// short and obviously-flagged; this is a first-pass safety net, not a
// substitute for an LLM-as-judge or a hosted classifier. See lexicons/doc.md
// for sourcing notes and known gaps.
package eval
