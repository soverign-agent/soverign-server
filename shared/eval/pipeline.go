package eval

import (
	"context"
	"fmt"

	"sovereign-ai-compliance/shared/metrics"
)

// EvalInput is the request envelope passed to (*Pipeline).Evaluate.
type EvalInput struct {
	TenantID   string
	Model      string
	OutputText string
	RAGContext []string
}

// EvalResult holds the three normalized scores plus any threshold breaches.
type EvalResult struct {
	HallucinationRate float64
	BiasScore         float64
	ToxicityScore     float64
	Violations        []Violation
}

// Violation describes a single threshold breach. Detail is safe to log: it
// names the category and a short reason but never includes the raw output.
type Violation struct {
	Category string
	Detail   string
}

// Thresholds gates which scores are reported as violations.
type Thresholds struct {
	Hallucination float64
	Bias          float64
	Toxicity      float64
}

func DefaultThresholds() Thresholds {
	return Thresholds{
		Hallucination: 0.6,
		Bias:          0.5,
		Toxicity:      0.4,
	}
}

// Pipeline scores an LLM output against the configured detectors. Construct
// it once at service startup and reuse — the lexicons are immutable after load.
type Pipeline struct {
	rec        *metrics.Recorder
	thresholds Thresholds
	biasLex    []string
	toxLex     toxicityRules
}

// NewPipeline loads the embedded lexicons and returns a ready Pipeline. The
// recorder may be nil so callers (and unit tests) can score without metrics.
func NewPipeline(rec *metrics.Recorder, thresholds Thresholds) (*Pipeline, error) {
	bias, err := loadBiasLexicon()
	if err != nil {
		return nil, err
	}
	tox, err := loadToxicityLexicon()
	if err != nil {
		return nil, err
	}
	return &Pipeline{
		rec:        rec,
		thresholds: thresholds,
		biasLex:    bias,
		toxLex:     tox,
	}, nil
}

// Evaluate runs every detector synchronously on the calling goroutine, emits
// metrics if a recorder is wired, and returns the aggregated result.
func (p *Pipeline) Evaluate(ctx context.Context, in EvalInput) (*EvalResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("eval: context cancelled: %w", err)
	}

	hallucinationRate := p.scoreHallucination(in.OutputText, in.RAGContext)
	biasScore, biasMatches := p.scoreBiasDetailed(in.OutputText)
	toxicityScore, toxMatches := p.scoreToxicityDetailed(in.OutputText)

	result := &EvalResult{
		HallucinationRate: hallucinationRate,
		BiasScore:         biasScore,
		ToxicityScore:     toxicityScore,
	}

	if hallucinationRate >= p.thresholds.Hallucination {
		result.Violations = append(result.Violations, Violation{
			Category: "hallucination",
			Detail:   fmt.Sprintf("rate %.2f exceeds threshold %.2f", hallucinationRate, p.thresholds.Hallucination),
		})
	}
	if biasScore >= p.thresholds.Bias {
		result.Violations = append(result.Violations, Violation{
			Category: "bias",
			Detail:   fmt.Sprintf("score %.2f from %d lexicon hit(s)", biasScore, len(biasMatches)),
		})
	}
	if toxicityScore >= p.thresholds.Toxicity {
		result.Violations = append(result.Violations, Violation{
			Category: "toxicity",
			Detail:   fmt.Sprintf("score %.2f from %d lexicon hit(s)", toxicityScore, len(toxMatches)),
		})
	}

	if p.rec != nil {
		p.rec.ObserveHallucinationRate(in.TenantID, in.Model, "rag_overlap", hallucinationRate)
		p.rec.ObserveBiasScore(in.TenantID, in.Model, biasScore)
		p.rec.ObserveToxicityScore(in.TenantID, in.Model, toxicityScore)
		for _, v := range result.Violations {
			p.rec.IncSafetyViolation(in.TenantID, in.Model, v.Category)
		}
	}

	return result, nil
}
