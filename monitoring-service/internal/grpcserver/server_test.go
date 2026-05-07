package grpcserver

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"

	"sovereign-ai-compliance/monitoring-service/internal/logic"
	monitoringv1 "sovereign-ai-compliance/shared/proto/monitoring/v1"
	"sovereign-ai-compliance/shared/tenant"
)

// fakeQuerier mirrors the one in the logic package — kept local so the two
// packages can drift independently if the wire interface ever changes.
type fakeQuerier struct {
	instantValue model.Value
	rangeValue   model.Value
}

func (f *fakeQuerier) QueryInstant(_ context.Context, _ string, _ time.Time) (model.Value, error) {
	if f.instantValue == nil {
		return model.Vector{&model.Sample{Value: 0}}, nil
	}
	return f.instantValue, nil
}

func (f *fakeQuerier) QueryRange(_ context.Context, _ string, _, _ time.Time, _ time.Duration) (model.Value, error) {
	if f.rangeValue == nil {
		return model.Matrix{&model.SampleStream{
			Values: []model.SamplePair{
				{Timestamp: model.Time(1000), Value: 0.1},
				{Timestamp: model.Time(2000), Value: 0.2},
			},
		}}, nil
	}
	return f.rangeValue, nil
}

func newServer() *Server {
	return NewServer(logic.NewService(&fakeQuerier{}, nil))
}

func TestWithTenant(t *testing.T) {
	md := metadata.New(map[string]string{"x-tenant-id": "tenant-abc"})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	result := withTenant(ctx)
	tenantID, ok := tenant.FromContext(result)
	require.True(t, ok)
	assert.Equal(t, "tenant-abc", tenantID)

	ctx2 := context.Background()
	result2 := withTenant(ctx2)
	_, ok2 := tenant.FromContext(result2)
	assert.False(t, ok2)

	md3 := metadata.New(map[string]string{"x-tenant-id": ""})
	ctx3 := metadata.NewIncomingContext(context.Background(), md3)
	result3 := withTenant(ctx3)
	_, ok3 := tenant.FromContext(result3)
	assert.False(t, ok3)
}

func tenantMD() context.Context {
	md := metadata.New(map[string]string{"x-tenant-id": "tenant-123"})
	return metadata.NewIncomingContext(context.Background(), md)
}

func TestGetOverview(t *testing.T) {
	server := newServer()

	resp, err := server.GetOverview(context.Background(), &monitoringv1.GetOverviewRequest{})
	require.Error(t, err)
	assert.Nil(t, resp)

	resp, err = server.GetOverview(tenantMD(), &monitoringv1.GetOverviewRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Overview)
	assert.Contains(t, resp.Overview.HallucinationRate, "%")
	assert.Contains(t, resp.Overview.AvgTtft, "ms")
	assert.Contains(t, resp.Overview.AvgTpot, "ms")
}

func TestGetHallucinationMetrics(t *testing.T) {
	server := newServer()

	resp, err := server.GetHallucinationMetrics(context.Background(), &monitoringv1.GetHallucinationMetricsRequest{})
	require.Error(t, err)
	assert.Nil(t, resp)

	resp, err = server.GetHallucinationMetrics(tenantMD(), &monitoringv1.GetHallucinationMetricsRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotEmpty(t, resp.Metrics)
	for _, m := range resp.Metrics {
		assert.NotEmpty(t, m.Time)
	}
}

func TestGetTokenPerformance(t *testing.T) {
	server := newServer()

	resp, err := server.GetTokenPerformance(context.Background(), &monitoringv1.GetTokenPerformanceRequest{})
	require.Error(t, err)
	assert.Nil(t, resp)

	resp, err = server.GetTokenPerformance(tenantMD(), &monitoringv1.GetTokenPerformanceRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotEmpty(t, resp.Metrics)
}

func TestGetHallucinationMetrics_WithTimeRange(t *testing.T) {
	server := newServer()
	start := time.Now().Add(-6 * time.Hour)
	end := time.Now()

	resp, err := server.GetHallucinationMetrics(tenantMD(), &monitoringv1.GetHallucinationMetricsRequest{
		StartTime: timestamppb.New(start),
		EndTime:   timestamppb.New(end),
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.Metrics)
}

func TestGetTokenPerformance_WithTimeRange(t *testing.T) {
	server := newServer()
	start := time.Now().Add(-6 * time.Hour)
	end := time.Now()

	resp, err := server.GetTokenPerformance(tenantMD(), &monitoringv1.GetTokenPerformanceRequest{
		StartTime: timestamppb.New(start),
		EndTime:   timestamppb.New(end),
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.Metrics)
}
