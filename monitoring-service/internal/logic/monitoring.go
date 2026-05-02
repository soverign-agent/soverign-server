// Package logic provides monitoring business logic and synthetic data generation.
package logic

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	"sovereign-ai-compliance/shared/tenant"
)

// HallucinationPoint is a single time-series point for hallucination data.
type HallucinationPoint struct {
	Time      time.Time
	Rate      float64
	Bias      float64
	Toxicity  float64
}

// TokenPerfPoint is a single time-series point for token performance data.
type TokenPerfPoint struct {
	Time time.Time
	TTFT float64
	TPOT float64
}

// Overview provides a high-level snapshot of AI system health.
type Overview struct {
	HallucinationRate string
	AvgTTFT           string
	AvgTPOT           string
	Alert             bool
}

// Generator produces synthetic monitoring data.
type Generator struct {
	rng *rand.Rand
}

// NewGenerator creates a new synthetic data generator.
func NewGenerator() *Generator {
	return &Generator{
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// GetOverview returns the current monitoring overview for a tenant.
// The tenantID is used to scope the response (different tenants get different data).
func (g *Generator) GetOverview(ctx context.Context) (*Overview, error) {
	tenantID, ok := tenant.FromContext(ctx)
	if !ok || tenantID == "" {
		return nil, fmt.Errorf("tenant context required")
	}

	// Deterministic variation based on tenantID so the same tenant
	// always sees consistent demo data.
	seed := hashString(tenantID)
	localRng := rand.New(rand.NewSource(seed))

	hallucinationRate := 1.0 + localRng.Float64()*4.0 // 1.0% - 5.0%
	avgTTFT := 80.0 + localRng.Float64()*80.0         // 80ms - 160ms
	avgTPOT := 20.0 + localRng.Float64()*50.0         // 20ms - 70ms

	alert := hallucinationRate > 4.0 || avgTTFT > 140 || avgTPOT > 60

	return &Overview{
		HallucinationRate: fmt.Sprintf("%.1f%%", hallucinationRate),
		AvgTTFT:           fmt.Sprintf("%.0fms", avgTTFT),
		AvgTPOT:           fmt.Sprintf("%.0fms", avgTPOT),
		Alert:             alert,
	}, nil
}

// GetHallucinationMetrics returns 24 hours of synthetic hallucination metrics.
func (g *Generator) GetHallucinationMetrics(ctx context.Context, startTime, endTime time.Time) ([]HallucinationPoint, error) {
	tenantID, ok := tenant.FromContext(ctx)
	if !ok || tenantID == "" {
		return nil, fmt.Errorf("tenant context required")
	}

	if startTime.IsZero() {
		startTime = time.Now().Add(-24 * time.Hour)
	}
	if endTime.IsZero() {
		endTime = time.Now()
	}

	seed := hashString(tenantID)
	localRng := rand.New(rand.NewSource(seed))

	points := generateHourlyPoints(startTime, endTime)
	result := make([]HallucinationPoint, len(points))

	for i, t := range points {
		// Use a sine wave + noise for realistic-looking time-series data
		hour := float64(t.Hour())
		baseRate := 0.02 + 0.015*math.Sin(hour*math.Pi/12.0)
		noise := (localRng.Float64() - 0.5) * 0.02
		rate := clamp(baseRate+noise, 0.0, 1.0)

		baseBias := 0.1 + 0.05*math.Sin(hour*math.Pi/12.0+math.Pi/4.0)
		biasNoise := (localRng.Float64() - 0.5) * 0.05
		bias := clamp(baseBias+biasNoise, 0.0, 1.0)

		baseToxicity := 0.02 + 0.01*math.Sin(hour*math.Pi/12.0+math.Pi/2.0)
		toxNoise := (localRng.Float64() - 0.5) * 0.01
		toxicity := clamp(baseToxicity+toxNoise, 0.0, 1.0)

		result[i] = HallucinationPoint{
			Time:     t,
			Rate:     rate,
			Bias:     bias,
			Toxicity: toxicity,
		}
	}

	return result, nil
}

// GetTokenPerformance returns 24 hours of synthetic token performance metrics.
func (g *Generator) GetTokenPerformance(ctx context.Context, startTime, endTime time.Time) ([]TokenPerfPoint, error) {
	tenantID, ok := tenant.FromContext(ctx)
	if !ok || tenantID == "" {
		return nil, fmt.Errorf("tenant context required")
	}

	if startTime.IsZero() {
		startTime = time.Now().Add(-24 * time.Hour)
	}
	if endTime.IsZero() {
		endTime = time.Now()
	}

	seed := hashString(tenantID)
	localRng := rand.New(rand.NewSource(seed))

	points := generateHourlyPoints(startTime, endTime)
	result := make([]TokenPerfPoint, len(points))

	for i, t := range points {
		hour := float64(t.Hour())
		// TTFT is higher during peak hours (9-17)
		baseTTFT := 100.0 + 40.0*math.Sin((hour-9.0)*math.Pi/8.0)
		ttftNoise := (localRng.Float64() - 0.5) * 30.0
		ttft := clamp(baseTTFT+ttftNoise, 20.0, 500.0)

		baseTPOT := 35.0 + 15.0*math.Sin((hour-10.0)*math.Pi/8.0)
		tpotNoise := (localRng.Float64() - 0.5) * 10.0
		tpot := clamp(baseTPOT+tpotNoise, 10.0, 120.0)

		result[i] = TokenPerfPoint{
			Time: t,
			TTFT: ttft,
			TPOT: tpot,
		}
	}

	return result, nil
}

// generateHourlyPoints generates hourly time points between start and end.
func generateHourlyPoints(start, end time.Time) []time.Time {
	start = start.Truncate(time.Hour)
	end = end.Truncate(time.Hour)

	var points []time.Time
	for t := start; !t.After(end); t = t.Add(time.Hour) {
		points = append(points, t)
	}
	return points
}

// hashString creates a deterministic int64 hash from a string.
func hashString(s string) int64 {
	var h int64 = 5381
	for i := 0; i < len(s); i++ {
		h = ((h << 5) + h) + int64(s[i])
	}
	return h
}

// clamp restricts v to the range [min, max].
func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
