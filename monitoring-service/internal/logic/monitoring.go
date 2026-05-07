// Package logic provides the monitoring business layer. It wraps a PromQL
// Querier and maps results to the value types consumed by the gRPC server.
//
// All synthetic-data generation has been removed. If Prometheus is unreachable,
// queries return errors rather than silently fabricating data.
package logic

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/common/model"
	"go.uber.org/zap"

	"sovereign-ai-compliance/monitoring-service/internal/promql"
	"sovereign-ai-compliance/shared/tenant"
)

// HallucinationPoint is a single time-series point for hallucination metrics.
type HallucinationPoint struct {
	Time     time.Time
	Rate     float64
	Bias     float64
	Toxicity float64
}

// TokenPerfPoint is a single time-series point for token performance metrics.
type TokenPerfPoint struct {
	Time time.Time
	TTFT float64
	TPOT float64
}

// Overview is a high-level snapshot of AI system health for the active tenant.
type Overview struct {
	HallucinationRate string
	AvgTTFT           string
	AvgTPOT           string
	Alert             bool
}

// Service is the monitoring business layer backed by a PromQL Querier.
type Service struct {
	querier promql.Querier
	logger  *zap.Logger
}

// NewService constructs a Service. logger may be nil — a no-op logger is used.
func NewService(q promql.Querier, logger *zap.Logger) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{querier: q, logger: logger}
}

// GetOverview reads instant Prometheus values for the active tenant and formats
// them for display.
//
// Empty Vector results from Prometheus are treated as zero values (no traffic
// for the tenant in the recording window). This is the documented behavior:
// the overview returns formatted zeros and Alert=false rather than an error,
// because "no LLM activity yet" is a valid steady state, not a failure.
func (s *Service) GetOverview(ctx context.Context) (*Overview, error) {
	tenantID, ok := tenant.FromContext(ctx)
	if !ok || tenantID == "" {
		return nil, fmt.Errorf("tenant context required")
	}

	now := time.Now()

	hallRate, err := s.scalarFromInstant(ctx, fmt.Sprintf(overviewHallucinationRateQuery, tenantID), now)
	if err != nil {
		return nil, fmt.Errorf("query hallucination rate: %w", err)
	}
	ttft, err := s.scalarFromInstant(ctx, fmt.Sprintf(overviewAvgTTFTQuery, tenantID), now)
	if err != nil {
		return nil, fmt.Errorf("query avg ttft: %w", err)
	}
	tpot, err := s.scalarFromInstant(ctx, fmt.Sprintf(overviewAvgTPOTQuery, tenantID), now)
	if err != nil {
		return nil, fmt.Errorf("query avg tpot: %w", err)
	}

	alert := hallRate > alertHallucinationRateSeconds ||
		ttft > alertTTFTSeconds ||
		tpot > alertTPOTSeconds

	return &Overview{
		HallucinationRate: fmt.Sprintf("%.1f%%", hallRate*100),
		AvgTTFT:           fmt.Sprintf("%.0fms", ttft*1000),
		AvgTPOT:           fmt.Sprintf("%.0fms", tpot*1000),
		Alert:             alert,
	}, nil
}

// GetHallucinationMetrics issues three range queries (rate, bias, toxicity)
// and aligns them by timestamp using the rate series as the canonical axis.
// Missing values in bias or toxicity at a given timestamp default to 0.0.
func (s *Service) GetHallucinationMetrics(ctx context.Context, start, end time.Time) ([]HallucinationPoint, error) {
	tenantID, ok := tenant.FromContext(ctx)
	if !ok || tenantID == "" {
		return nil, fmt.Errorf("tenant context required")
	}

	start, end, step := normalizeRange(start, end)

	rateMatrix, err := s.matrixFromRange(ctx, fmt.Sprintf(hallucinationTimeSeriesQuery, tenantID), start, end, step)
	if err != nil {
		return nil, fmt.Errorf("query hallucination series: %w", err)
	}
	biasMatrix, err := s.matrixFromRange(ctx, fmt.Sprintf(biasTimeSeriesQuery, tenantID), start, end, step)
	if err != nil {
		return nil, fmt.Errorf("query bias series: %w", err)
	}
	toxMatrix, err := s.matrixFromRange(ctx, fmt.Sprintf(toxicityTimeSeriesQuery, tenantID), start, end, step)
	if err != nil {
		return nil, fmt.Errorf("query toxicity series: %w", err)
	}

	rateSeries := firstStream(rateMatrix)
	if rateSeries == nil {
		return []HallucinationPoint{}, nil
	}
	biasIdx := indexByTimestamp(firstStream(biasMatrix))
	toxIdx := indexByTimestamp(firstStream(toxMatrix))

	out := make([]HallucinationPoint, 0, len(rateSeries))
	for _, sp := range rateSeries {
		out = append(out, HallucinationPoint{
			Time:     sp.Timestamp.Time(),
			Rate:     float64(sp.Value),
			Bias:     biasIdx[sp.Timestamp],
			Toxicity: toxIdx[sp.Timestamp],
		})
	}
	return out, nil
}

// GetTokenPerformance issues two range queries (TTFT, TPOT) and aligns them
// by timestamp using the TTFT series as the canonical axis. Values are
// converted from seconds (Prometheus native) to milliseconds for display.
func (s *Service) GetTokenPerformance(ctx context.Context, start, end time.Time) ([]TokenPerfPoint, error) {
	tenantID, ok := tenant.FromContext(ctx)
	if !ok || tenantID == "" {
		return nil, fmt.Errorf("tenant context required")
	}

	start, end, step := normalizeRange(start, end)

	ttftMatrix, err := s.matrixFromRange(ctx, fmt.Sprintf(ttftTimeSeriesQuery, tenantID), start, end, step)
	if err != nil {
		return nil, fmt.Errorf("query ttft series: %w", err)
	}
	tpotMatrix, err := s.matrixFromRange(ctx, fmt.Sprintf(tpotTimeSeriesQuery, tenantID), start, end, step)
	if err != nil {
		return nil, fmt.Errorf("query tpot series: %w", err)
	}

	ttftSeries := firstStream(ttftMatrix)
	if ttftSeries == nil {
		return []TokenPerfPoint{}, nil
	}
	tpotIdx := indexByTimestamp(firstStream(tpotMatrix))

	out := make([]TokenPerfPoint, 0, len(ttftSeries))
	for _, sp := range ttftSeries {
		out = append(out, TokenPerfPoint{
			Time: sp.Timestamp.Time(),
			TTFT: float64(sp.Value) * 1000,
			TPOT: tpotIdx[sp.Timestamp] * 1000,
		})
	}
	return out, nil
}

// scalarFromInstant runs an instant query and reduces a Vector to a single
// float. Empty vectors → 0.0 (interpreted as "no data yet"). Non-vector
// result types yield an error since they indicate a query template bug.
func (s *Service) scalarFromInstant(ctx context.Context, query string, ts time.Time) (float64, error) {
	val, err := s.querier.QueryInstant(ctx, query, ts)
	if err != nil {
		return 0, err
	}
	vec, ok := val.(model.Vector)
	if !ok {
		return 0, fmt.Errorf("expected vector result, got %T", val)
	}
	if len(vec) == 0 {
		return 0, nil
	}
	return float64(vec[0].Value), nil
}

// matrixFromRange runs a range query and returns the resulting Matrix. Non-
// matrix result types yield an error.
func (s *Service) matrixFromRange(ctx context.Context, query string, start, end time.Time, step time.Duration) (model.Matrix, error) {
	val, err := s.querier.QueryRange(ctx, query, start, end, step)
	if err != nil {
		return nil, err
	}
	mat, ok := val.(model.Matrix)
	if !ok {
		return nil, fmt.Errorf("expected matrix result, got %T", val)
	}
	return mat, nil
}

// firstStream returns the first SampleStream in mat, or nil if mat is empty.
// Our queries aggregate to a single series per tenant, so position 0 is
// always the canonical series when present.
func firstStream(mat model.Matrix) []model.SamplePair {
	if len(mat) == 0 {
		return nil
	}
	return mat[0].Values
}

// indexByTimestamp builds a map from sample timestamp to value, used to align
// secondary series to a canonical timestamp axis.
func indexByTimestamp(series []model.SamplePair) map[model.Time]float64 {
	idx := make(map[model.Time]float64, len(series))
	for _, sp := range series {
		idx[sp.Timestamp] = float64(sp.Value)
	}
	return idx
}

// normalizeRange fills in defaults for zero start/end and picks a step. The
// 1h step matches the existing UI granularity of the synthetic generator we
// replaced.
func normalizeRange(start, end time.Time) (time.Time, time.Time, time.Duration) {
	if end.IsZero() {
		end = time.Now()
	}
	if start.IsZero() {
		start = end.Add(-24 * time.Hour)
	}
	return start, end, time.Hour
}
