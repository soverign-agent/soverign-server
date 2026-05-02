package grpcserver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	"sovereign-ai-compliance/monitoring-service/internal/logic"
	"sovereign-ai-compliance/shared/proto/monitoring/v1"
	"sovereign-ai-compliance/shared/tenant"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestWithTenant(t *testing.T) {
	// Test with x-tenant-id metadata
	md := metadata.New(map[string]string{"x-tenant-id": "tenant-abc"})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	result := withTenant(ctx)
	tenantID, ok := tenant.FromContext(result)
	require.True(t, ok)
	assert.Equal(t, "tenant-abc", tenantID)

	// Test without metadata
	ctx2 := context.Background()
	result2 := withTenant(ctx2)
	_, ok2 := tenant.FromContext(result2)
	assert.False(t, ok2)

	// Test with empty tenant-id
	md3 := metadata.New(map[string]string{"x-tenant-id": ""})
	ctx3 := metadata.NewIncomingContext(context.Background(), md3)
	result3 := withTenant(ctx3)
	_, ok3 := tenant.FromContext(result3)
	assert.False(t, ok3)
}

func TestGetOverview(t *testing.T) {
	generator := logic.NewGenerator()
	server := NewServer(generator)

	// Missing tenant should fail
	ctx := context.Background()
	resp, err := server.GetOverview(ctx, &monitoringv1.GetOverviewRequest{})
	require.Error(t, err)
	assert.Nil(t, resp)

	// With tenant should succeed
	md := metadata.New(map[string]string{"x-tenant-id": "tenant-123"})
	ctx = metadata.NewIncomingContext(context.Background(), md)

	resp, err = server.GetOverview(ctx, &monitoringv1.GetOverviewRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Overview)

	assert.Contains(t, resp.Overview.HallucinationRate, "%")
	assert.Contains(t, resp.Overview.AvgTtft, "ms")
	assert.Contains(t, resp.Overview.AvgTpot, "ms")
}

func TestGetHallucinationMetrics(t *testing.T) {
	generator := logic.NewGenerator()
	server := NewServer(generator)

	// Missing tenant should fail
	ctx := context.Background()
	resp, err := server.GetHallucinationMetrics(ctx, &monitoringv1.GetHallucinationMetricsRequest{})
	require.Error(t, err)
	assert.Nil(t, resp)

	// With tenant should succeed
	md := metadata.New(map[string]string{"x-tenant-id": "tenant-123"})
	ctx = metadata.NewIncomingContext(context.Background(), md)

	resp, err = server.GetHallucinationMetrics(ctx, &monitoringv1.GetHallucinationMetricsRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotEmpty(t, resp.Metrics)

	for _, m := range resp.Metrics {
		assert.NotEmpty(t, m.Time)
		assert.GreaterOrEqual(t, m.Rate, 0.0)
		assert.LessOrEqual(t, m.Rate, 1.0)
	}
}

func TestGetTokenPerformance(t *testing.T) {
	generator := logic.NewGenerator()
	server := NewServer(generator)

	// Missing tenant should fail
	ctx := context.Background()
	resp, err := server.GetTokenPerformance(ctx, &monitoringv1.GetTokenPerformanceRequest{})
	require.Error(t, err)
	assert.Nil(t, resp)

	// With tenant should succeed
	md := metadata.New(map[string]string{"x-tenant-id": "tenant-123"})
	ctx = metadata.NewIncomingContext(context.Background(), md)

	resp, err = server.GetTokenPerformance(ctx, &monitoringv1.GetTokenPerformanceRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotEmpty(t, resp.Metrics)

	for _, m := range resp.Metrics {
		assert.NotEmpty(t, m.Time)
		assert.GreaterOrEqual(t, m.Ttft, 20.0)
		assert.LessOrEqual(t, m.Ttft, 500.0)
	}
}

func TestGetHallucinationMetrics_WithTimeRange(t *testing.T) {
	generator := logic.NewGenerator()
	server := NewServer(generator)

	md := metadata.New(map[string]string{"x-tenant-id": "tenant-123"})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	start := time.Now().Add(-6 * time.Hour)
	end := time.Now()

	resp, err := server.GetHallucinationMetrics(ctx, &monitoringv1.GetHallucinationMetricsRequest{
		StartTime: mustTimestamp(start),
		EndTime:   mustTimestamp(end),
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.GreaterOrEqual(t, len(resp.Metrics), 6)
}

func TestGetTokenPerformance_WithTimeRange(t *testing.T) {
	generator := logic.NewGenerator()
	server := NewServer(generator)

	md := metadata.New(map[string]string{"x-tenant-id": "tenant-123"})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	start := time.Now().Add(-6 * time.Hour)
	end := time.Now()

	resp, err := server.GetTokenPerformance(ctx, &monitoringv1.GetTokenPerformanceRequest{
		StartTime: mustTimestamp(start),
		EndTime:   mustTimestamp(end),
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.GreaterOrEqual(t, len(resp.Metrics), 6)
}

func mustTimestamp(t time.Time) *timestamppb.Timestamp {
	return timestamppb.New(t)
}

