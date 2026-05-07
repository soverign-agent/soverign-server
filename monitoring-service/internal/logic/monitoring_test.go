package logic

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sovereign-ai-compliance/shared/tenant"
)

// fakeQuerier is a hand-rolled fake — keeps tests free of network mocks.
type fakeQuerier struct {
	instant      map[string]model.Value
	rng          map[string]model.Value
	instantErr   error
	rangeErr     error
	instantCalls []string
	rangeCalls   []string
}

func (f *fakeQuerier) QueryInstant(_ context.Context, query string, _ time.Time) (model.Value, error) {
	f.instantCalls = append(f.instantCalls, query)
	if f.instantErr != nil {
		return nil, f.instantErr
	}
	if v, ok := f.instant[query]; ok {
		return v, nil
	}
	return model.Vector{}, nil
}

func (f *fakeQuerier) QueryRange(_ context.Context, query string, _, _ time.Time, _ time.Duration) (model.Value, error) {
	f.rangeCalls = append(f.rangeCalls, query)
	if f.rangeErr != nil {
		return nil, f.rangeErr
	}
	if v, ok := f.rng[query]; ok {
		return v, nil
	}
	return model.Matrix{}, nil
}

func vec(v float64) model.Vector {
	return model.Vector{&model.Sample{Value: model.SampleValue(v), Timestamp: 0}}
}

func mat(points map[int64]float64) model.Matrix {
	pairs := make([]model.SamplePair, 0, len(points))
	// Iterate map keys in a deterministic order — Prometheus returns sorted.
	keys := make([]int64, 0, len(points))
	for k := range points {
		keys = append(keys, k)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	for _, k := range keys {
		pairs = append(pairs, model.SamplePair{Timestamp: model.Time(k), Value: model.SampleValue(points[k])})
	}
	return model.Matrix{&model.SampleStream{Values: pairs}}
}

func tenantCtx(id string) context.Context {
	return tenant.WithContext(context.Background(), id)
}

func TestNewService_NilLogger(t *testing.T) {
	s := NewService(&fakeQuerier{}, nil)
	require.NotNil(t, s)
	require.NotNil(t, s.logger)
}

func TestGetOverview_MissingTenant(t *testing.T) {
	s := NewService(&fakeQuerier{}, nil)
	ov, err := s.GetOverview(context.Background())
	require.Error(t, err)
	assert.Nil(t, ov)
	assert.Contains(t, err.Error(), "tenant context required")
}

func TestGetOverview_HappyPath(t *testing.T) {
	tenantID := "tenant-1"
	q := &fakeQuerier{
		instant: map[string]model.Value{
			fmtq(overviewHallucinationRateQuery, tenantID): vec(0.023), // 2.3%
			fmtq(overviewAvgTTFTQuery, tenantID):           vec(0.120), // 120ms
			fmtq(overviewAvgTPOTQuery, tenantID):           vec(0.045), // 45ms
		},
	}
	s := NewService(q, nil)

	ov, err := s.GetOverview(tenantCtx(tenantID))
	require.NoError(t, err)
	require.NotNil(t, ov)
	assert.Equal(t, "2.3%", ov.HallucinationRate)
	assert.Equal(t, "120ms", ov.AvgTTFT)
	assert.Equal(t, "45ms", ov.AvgTPOT)
	assert.False(t, ov.Alert)
}

func TestGetOverview_AlertThresholds(t *testing.T) {
	tests := []struct {
		name          string
		hallRate      float64
		ttft          float64
		tpot          float64
		expectedAlert bool
	}{
		{"all green", 0.02, 0.10, 0.04, false},
		{"hallucination too high", 0.05, 0.10, 0.04, true},
		{"ttft too high", 0.02, 0.20, 0.04, true},
		{"tpot too high", 0.02, 0.10, 0.07, true},
		{"all red", 0.10, 0.30, 0.10, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tenantID := "t"
			q := &fakeQuerier{instant: map[string]model.Value{
				fmtq(overviewHallucinationRateQuery, tenantID): vec(tc.hallRate),
				fmtq(overviewAvgTTFTQuery, tenantID):           vec(tc.ttft),
				fmtq(overviewAvgTPOTQuery, tenantID):           vec(tc.tpot),
			}}
			s := NewService(q, nil)
			ov, err := s.GetOverview(tenantCtx(tenantID))
			require.NoError(t, err)
			assert.Equal(t, tc.expectedAlert, ov.Alert)
		})
	}
}

func TestGetOverview_EmptyVector(t *testing.T) {
	// No data in Prometheus yet — must NOT error, return formatted zeros.
	q := &fakeQuerier{} // all queries return empty Vector
	s := NewService(q, nil)

	ov, err := s.GetOverview(tenantCtx("tenant-empty"))
	require.NoError(t, err)
	assert.Equal(t, "0.0%", ov.HallucinationRate)
	assert.Equal(t, "0ms", ov.AvgTTFT)
	assert.Equal(t, "0ms", ov.AvgTPOT)
	assert.False(t, ov.Alert)
}

func TestGetOverview_QuerierError(t *testing.T) {
	q := &fakeQuerier{instantErr: errors.New("boom")}
	s := NewService(q, nil)

	ov, err := s.GetOverview(tenantCtx("t"))
	require.Error(t, err)
	assert.Nil(t, ov)
	assert.Contains(t, err.Error(), "boom")
}

func TestGetOverview_NonVectorResult(t *testing.T) {
	tenantID := "t"
	q := &fakeQuerier{instant: map[string]model.Value{
		fmtq(overviewHallucinationRateQuery, tenantID): mat(map[int64]float64{1: 0.5}),
	}}
	s := NewService(q, nil)

	ov, err := s.GetOverview(tenantCtx(tenantID))
	require.Error(t, err)
	assert.Nil(t, ov)
	assert.Contains(t, err.Error(), "expected vector")
}

func TestGetHallucinationMetrics_MissingTenant(t *testing.T) {
	s := NewService(&fakeQuerier{}, nil)
	out, err := s.GetHallucinationMetrics(context.Background(), time.Time{}, time.Time{})
	require.Error(t, err)
	assert.Nil(t, out)
}

func TestGetHallucinationMetrics_AlignedSeries(t *testing.T) {
	tenantID := "t"
	q := &fakeQuerier{rng: map[string]model.Value{
		fmtq(hallucinationTimeSeriesQuery, tenantID): mat(map[int64]float64{1000: 0.02, 2000: 0.03, 3000: 0.04}),
		fmtq(biasTimeSeriesQuery, tenantID):          mat(map[int64]float64{1000: 0.10, 2000: 0.11, 3000: 0.12}),
		fmtq(toxicityTimeSeriesQuery, tenantID):      mat(map[int64]float64{1000: 0.01, 2000: 0.02, 3000: 0.03}),
	}}
	s := NewService(q, nil)

	pts, err := s.GetHallucinationMetrics(tenantCtx(tenantID), time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, pts, 3)
	assert.InDelta(t, 0.02, pts[0].Rate, 1e-9)
	assert.InDelta(t, 0.10, pts[0].Bias, 1e-9)
	assert.InDelta(t, 0.01, pts[0].Toxicity, 1e-9)
	assert.InDelta(t, 0.04, pts[2].Rate, 1e-9)
	assert.InDelta(t, 0.12, pts[2].Bias, 1e-9)
	assert.InDelta(t, 0.03, pts[2].Toxicity, 1e-9)
}

func TestGetHallucinationMetrics_MisalignedSeries(t *testing.T) {
	tenantID := "t"
	// Bias missing one timestamp (2000); toxicity missing two (1000, 3000).
	q := &fakeQuerier{rng: map[string]model.Value{
		fmtq(hallucinationTimeSeriesQuery, tenantID): mat(map[int64]float64{1000: 0.02, 2000: 0.03, 3000: 0.04}),
		fmtq(biasTimeSeriesQuery, tenantID):          mat(map[int64]float64{1000: 0.10, 3000: 0.12}),
		fmtq(toxicityTimeSeriesQuery, tenantID):      mat(map[int64]float64{2000: 0.02}),
	}}
	s := NewService(q, nil)

	pts, err := s.GetHallucinationMetrics(tenantCtx(tenantID), time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, pts, 3)
	// Bias missing at t=2000 → 0.0
	assert.InDelta(t, 0.0, pts[1].Bias, 1e-9)
	// Toxicity present only at t=2000
	assert.InDelta(t, 0.0, pts[0].Toxicity, 1e-9)
	assert.InDelta(t, 0.02, pts[1].Toxicity, 1e-9)
	assert.InDelta(t, 0.0, pts[2].Toxicity, 1e-9)
}

func TestGetHallucinationMetrics_EmptyMatrix(t *testing.T) {
	q := &fakeQuerier{} // all queries return empty Matrix
	s := NewService(q, nil)

	pts, err := s.GetHallucinationMetrics(tenantCtx("t"), time.Time{}, time.Time{})
	require.NoError(t, err)
	assert.Empty(t, pts)
}

func TestGetHallucinationMetrics_QuerierError(t *testing.T) {
	q := &fakeQuerier{rangeErr: errors.New("prom down")}
	s := NewService(q, nil)

	pts, err := s.GetHallucinationMetrics(tenantCtx("t"), time.Time{}, time.Time{})
	require.Error(t, err)
	assert.Nil(t, pts)
	assert.Contains(t, err.Error(), "prom down")
}

func TestGetTokenPerformance_MissingTenant(t *testing.T) {
	s := NewService(&fakeQuerier{}, nil)
	out, err := s.GetTokenPerformance(context.Background(), time.Time{}, time.Time{})
	require.Error(t, err)
	assert.Nil(t, out)
}

func TestGetTokenPerformance_HappyPath(t *testing.T) {
	tenantID := "t"
	q := &fakeQuerier{rng: map[string]model.Value{
		// Values stored in seconds; service converts to ms.
		fmtq(ttftTimeSeriesQuery, tenantID): mat(map[int64]float64{1000: 0.120, 2000: 0.140}),
		fmtq(tpotTimeSeriesQuery, tenantID): mat(map[int64]float64{1000: 0.045, 2000: 0.050}),
	}}
	s := NewService(q, nil)

	pts, err := s.GetTokenPerformance(tenantCtx(tenantID), time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, pts, 2)
	assert.InDelta(t, 120.0, pts[0].TTFT, 1e-9)
	assert.InDelta(t, 45.0, pts[0].TPOT, 1e-9)
	assert.InDelta(t, 140.0, pts[1].TTFT, 1e-9)
	assert.InDelta(t, 50.0, pts[1].TPOT, 1e-9)
}

func TestGetTokenPerformance_MisalignedSeries(t *testing.T) {
	tenantID := "t"
	q := &fakeQuerier{rng: map[string]model.Value{
		fmtq(ttftTimeSeriesQuery, tenantID): mat(map[int64]float64{1000: 0.120, 2000: 0.140}),
		fmtq(tpotTimeSeriesQuery, tenantID): mat(map[int64]float64{1000: 0.045}), // missing 2000
	}}
	s := NewService(q, nil)

	pts, err := s.GetTokenPerformance(tenantCtx(tenantID), time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, pts, 2)
	assert.InDelta(t, 0.0, pts[1].TPOT, 1e-9)
}

func TestGetTokenPerformance_EmptyMatrix(t *testing.T) {
	s := NewService(&fakeQuerier{}, nil)
	pts, err := s.GetTokenPerformance(tenantCtx("t"), time.Time{}, time.Time{})
	require.NoError(t, err)
	assert.Empty(t, pts)
}

func TestGetTokenPerformance_QuerierError(t *testing.T) {
	q := &fakeQuerier{rangeErr: errors.New("prom down")}
	s := NewService(q, nil)

	pts, err := s.GetTokenPerformance(tenantCtx("t"), time.Time{}, time.Time{})
	require.Error(t, err)
	assert.Nil(t, pts)
}

func TestNormalizeRange_DefaultsBothZero(t *testing.T) {
	start, end, step := normalizeRange(time.Time{}, time.Time{})
	assert.False(t, start.IsZero())
	assert.False(t, end.IsZero())
	assert.Equal(t, time.Hour, step)
	assert.WithinDuration(t, end.Add(-24*time.Hour), start, time.Second)
}

func TestNormalizeRange_CustomEnd(t *testing.T) {
	end := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	start, gotEnd, _ := normalizeRange(time.Time{}, end)
	assert.Equal(t, end, gotEnd)
	assert.Equal(t, end.Add(-24*time.Hour), start)
}

func TestIndexByTimestamp_Empty(t *testing.T) {
	idx := indexByTimestamp(nil)
	assert.Empty(t, idx)
}

// fmtq is a tiny helper so we can build expected query strings without
// importing fmt directly into every test case. Mirrors the fmt.Sprintf in
// the service — keep in sync.
func fmtq(template, tenantID string) string {
	// Same %q semantics the service uses.
	const dq = `"`
	return replaceFirst(template, "%q", dq+tenantID+dq)
}

// replaceFirst is a minimal replacement helper for the test utility above.
// We avoid pulling in strings.Replace in this hot path for clarity.
func replaceFirst(s, old, new string) string {
	for i := 0; i+len(old) <= len(s); i++ {
		if s[i:i+len(old)] == old {
			return s[:i] + new + s[i+len(old):]
		}
	}
	return s
}
