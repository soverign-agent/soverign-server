package logic

import (
	"context"
	"testing"
	"time"

	"sovereign-ai-compliance/shared/tenant"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGenerator(t *testing.T) {
	g := NewGenerator()
	require.NotNil(t, g)
	require.NotNil(t, g.rng)
}

func TestGetOverview_MissingTenant(t *testing.T) {
	g := NewGenerator()
	ctx := context.Background()

	overview, err := g.GetOverview(ctx)
	require.Error(t, err)
	assert.Nil(t, overview)
	assert.Contains(t, err.Error(), "tenant context required")
}

func TestGetOverview_Success(t *testing.T) {
	g := NewGenerator()
	ctx := tenant.WithContext(context.Background(), "tenant-123")

	overview, err := g.GetOverview(ctx)
	require.NoError(t, err)
	require.NotNil(t, overview)

	// Verify format
	assert.Contains(t, overview.HallucinationRate, "%")
	assert.Contains(t, overview.AvgTTFT, "ms")
	assert.Contains(t, overview.AvgTPOT, "ms")

	// Verify determinism: same tenant should return same values
	overview2, err := g.GetOverview(ctx)
	require.NoError(t, err)
	assert.Equal(t, overview.HallucinationRate, overview2.HallucinationRate)
	assert.Equal(t, overview.AvgTTFT, overview2.AvgTTFT)
	assert.Equal(t, overview.AvgTPOT, overview2.AvgTPOT)
	assert.Equal(t, overview.Alert, overview2.Alert)
}

func TestGetOverview_DifferentTenants(t *testing.T) {
	g := NewGenerator()
	ctx1 := tenant.WithContext(context.Background(), "tenant-a")
	ctx2 := tenant.WithContext(context.Background(), "tenant-b")

	overview1, err := g.GetOverview(ctx1)
	require.NoError(t, err)

	overview2, err := g.GetOverview(ctx2)
	require.NoError(t, err)

	// Different tenants should get different data (with very high probability)
	// We only check that they are not all identical
	allSame := overview1.HallucinationRate == overview2.HallucinationRate &&
		overview1.AvgTTFT == overview2.AvgTTFT &&
		overview1.AvgTPOT == overview2.AvgTPOT &&
		overview1.Alert == overview2.Alert
	assert.False(t, allSame, "different tenants should get different demo data")
}

func TestGetHallucinationMetrics_MissingTenant(t *testing.T) {
	g := NewGenerator()
	ctx := context.Background()

	metrics, err := g.GetHallucinationMetrics(ctx, time.Time{}, time.Time{})
	require.Error(t, err)
	assert.Nil(t, metrics)
}

func TestGetHallucinationMetrics_DefaultRange(t *testing.T) {
	g := NewGenerator()
	ctx := tenant.WithContext(context.Background(), "tenant-123")

	metrics, err := g.GetHallucinationMetrics(ctx, time.Time{}, time.Time{})
	require.NoError(t, err)
	require.NotEmpty(t, metrics)

	// Should generate ~25 points for 24 hours
	assert.GreaterOrEqual(t, len(metrics), 24)
	assert.LessOrEqual(t, len(metrics), 25)

	for _, m := range metrics {
		assert.False(t, m.Time.IsZero())
		assert.GreaterOrEqual(t, m.Rate, 0.0)
		assert.LessOrEqual(t, m.Rate, 1.0)
		assert.GreaterOrEqual(t, m.Bias, 0.0)
		assert.LessOrEqual(t, m.Bias, 1.0)
		assert.GreaterOrEqual(t, m.Toxicity, 0.0)
		assert.LessOrEqual(t, m.Toxicity, 1.0)
	}
}

func TestGetHallucinationMetrics_CustomRange(t *testing.T) {
	g := NewGenerator()
	ctx := tenant.WithContext(context.Background(), "tenant-123")

	start := time.Now().Add(-6 * time.Hour)
	end := time.Now()

	metrics, err := g.GetHallucinationMetrics(ctx, start, end)
	require.NoError(t, err)
	require.NotEmpty(t, metrics)

	// Should generate ~7 points for 6 hours
	assert.GreaterOrEqual(t, len(metrics), 6)
	assert.LessOrEqual(t, len(metrics), 7)
}

func TestGetHallucinationMetrics_Determinism(t *testing.T) {
	g := NewGenerator()
	ctx := tenant.WithContext(context.Background(), "tenant-abc")

	start := time.Now().Add(-12 * time.Hour)
	end := time.Now()

	m1, err := g.GetHallucinationMetrics(ctx, start, end)
	require.NoError(t, err)

	m2, err := g.GetHallucinationMetrics(ctx, start, end)
	require.NoError(t, err)

	require.Equal(t, len(m1), len(m2))
	for i := range m1 {
		assert.Equal(t, m1[i].Time, m2[i].Time)
		assert.Equal(t, m1[i].Rate, m2[i].Rate)
		assert.Equal(t, m1[i].Bias, m2[i].Bias)
		assert.Equal(t, m1[i].Toxicity, m2[i].Toxicity)
	}
}

func TestGetTokenPerformance_MissingTenant(t *testing.T) {
	g := NewGenerator()
	ctx := context.Background()

	metrics, err := g.GetTokenPerformance(ctx, time.Time{}, time.Time{})
	require.Error(t, err)
	assert.Nil(t, metrics)
}

func TestGetTokenPerformance_DefaultRange(t *testing.T) {
	g := NewGenerator()
	ctx := tenant.WithContext(context.Background(), "tenant-123")

	metrics, err := g.GetTokenPerformance(ctx, time.Time{}, time.Time{})
	require.NoError(t, err)
	require.NotEmpty(t, metrics)

	assert.GreaterOrEqual(t, len(metrics), 24)
	assert.LessOrEqual(t, len(metrics), 25)

	for _, m := range metrics {
		assert.False(t, m.Time.IsZero())
		assert.GreaterOrEqual(t, m.TTFT, 20.0)
		assert.LessOrEqual(t, m.TTFT, 500.0)
		assert.GreaterOrEqual(t, m.TPOT, 10.0)
		assert.LessOrEqual(t, m.TPOT, 120.0)
	}
}

func TestGetTokenPerformance_Determinism(t *testing.T) {
	g := NewGenerator()
	ctx := tenant.WithContext(context.Background(), "tenant-xyz")

	start := time.Now().Add(-8 * time.Hour)
	end := time.Now()

	m1, err := g.GetTokenPerformance(ctx, start, end)
	require.NoError(t, err)

	m2, err := g.GetTokenPerformance(ctx, start, end)
	require.NoError(t, err)

	require.Equal(t, len(m1), len(m2))
	for i := range m1 {
		assert.Equal(t, m1[i].Time, m2[i].Time)
		assert.Equal(t, m1[i].TTFT, m2[i].TTFT)
		assert.Equal(t, m1[i].TPOT, m2[i].TPOT)
	}
}

func TestClamp(t *testing.T) {
	assert.Equal(t, 5.0, clamp(3.0, 5.0, 10.0))
	assert.Equal(t, 10.0, clamp(15.0, 5.0, 10.0))
	assert.Equal(t, 7.0, clamp(7.0, 5.0, 10.0))
}

func TestHashString(t *testing.T) {
	h1 := hashString("abc")
	h2 := hashString("abc")
	h3 := hashString("def")

	assert.Equal(t, h1, h2)
	assert.NotEqual(t, h1, h3)
}

func TestGenerateHourlyPoints(t *testing.T) {
	start := time.Date(2024, 1, 1, 10, 30, 0, 0, time.UTC)
	end := time.Date(2024, 1, 1, 14, 45, 0, 0, time.UTC)

	points := generateHourlyPoints(start, end)
	require.Len(t, points, 5)

	// Verify truncation and hourly spacing
	assert.Equal(t, 10, points[0].Hour())
	assert.Equal(t, 11, points[1].Hour())
	assert.Equal(t, 12, points[2].Hour())
	assert.Equal(t, 13, points[3].Hour())
	assert.Equal(t, 14, points[4].Hour())

	for _, p := range points {
		assert.Equal(t, 0, p.Minute())
		assert.Equal(t, 0, p.Second())
	}
}
