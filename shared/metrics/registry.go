package metrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

// NewRegistry constructs an isolated *prometheus.Registry. The default global
// registerer is intentionally not used so multiple services and tests can
// coexist without conflict.
func NewRegistry() *prometheus.Registry {
	return prometheus.NewRegistry()
}

type Recorder struct {
	ttftSeconds          *prometheus.HistogramVec
	tpotSeconds          *prometheus.HistogramVec
	totalDurationSeconds *prometheus.HistogramVec
	promptTokens         *prometheus.CounterVec
	completionTokens     *prometheus.CounterVec

	hallucinationRate *prometheus.HistogramVec
	biasScore         *prometheus.HistogramVec
	toxicityScore     *prometheus.HistogramVec
	safetyViolations  *prometheus.CounterVec
}

// NewRecorder builds every metric handle and registers it with reg. An error is
// returned only if registration fails (i.e., a duplicate registry).
func NewRecorder(reg *prometheus.Registry) (*Recorder, error) {
	if reg == nil {
		return nil, fmt.Errorf("metrics: registry must not be nil")
	}

	r := &Recorder{
		ttftSeconds:          newTTFTHistogram(),
		tpotSeconds:          newTPOTHistogram(),
		totalDurationSeconds: newTotalDurationHistogram(),
		promptTokens:         newPromptTokenCounter(),
		completionTokens:     newCompletionTokenCounter(),
		hallucinationRate:    newHallucinationHistogram(),
		biasScore:            newBiasHistogram(),
		toxicityScore:        newToxicityHistogram(),
		safetyViolations:     newSafetyViolationCounter(),
	}

	collectors := []prometheus.Collector{
		r.ttftSeconds,
		r.tpotSeconds,
		r.totalDurationSeconds,
		r.promptTokens,
		r.completionTokens,
		r.hallucinationRate,
		r.biasScore,
		r.toxicityScore,
		r.safetyViolations,
	}
	for _, c := range collectors {
		if err := reg.Register(c); err != nil {
			return nil, fmt.Errorf("metrics: register collector: %w", err)
		}
	}
	return r, nil
}
