// Package grpcserver provides the gRPC server implementation for monitoring-service.
package grpcserver

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"sovereign-ai-compliance/monitoring-service/internal/logic"
	"sovereign-ai-compliance/shared/proto/monitoring/v1"
	"sovereign-ai-compliance/shared/tenant"
)

// Server implements monitoringv1.MonitoringServiceServer.
type Server struct {
	monitoringv1.UnimplementedMonitoringServiceServer

	service *logic.Service
}

// NewServer creates a new gRPC server for monitoring-service.
func NewServer(service *logic.Service) *Server {
	return &Server{
		service: service,
	}
}

// Register registers the server on the provided gRPC server.
func (s *Server) Register(grpcServer *grpc.Server) {
	monitoringv1.RegisterMonitoringServiceServer(grpcServer, s)
}

// withTenant extracts tenant ID from gRPC metadata and injects it into context.
func withTenant(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	vals := md.Get("x-tenant-id")
	if len(vals) > 0 && vals[0] != "" {
		return tenant.WithContext(ctx, vals[0])
	}
	return ctx
}

// GetOverview returns the current monitoring overview.
func (s *Server) GetOverview(ctx context.Context, req *monitoringv1.GetOverviewRequest) (*monitoringv1.GetOverviewResponse, error) {
	ctx = withTenant(ctx)

	overview, err := s.service.GetOverview(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get overview: %v", err)
	}

	return &monitoringv1.GetOverviewResponse{
		Overview: &monitoringv1.MonitoringOverview{
			HallucinationRate: overview.HallucinationRate,
			AvgTtft:           overview.AvgTTFT,
			AvgTpot:           overview.AvgTPOT,
			Alert:             overview.Alert,
		},
	}, nil
}

// GetHallucinationMetrics returns a time-series of hallucination metrics.
func (s *Server) GetHallucinationMetrics(ctx context.Context, req *monitoringv1.GetHallucinationMetricsRequest) (*monitoringv1.GetHallucinationMetricsResponse, error) {
	ctx = withTenant(ctx)

	var startTime, endTime time.Time
	if req.StartTime != nil {
		startTime = req.StartTime.AsTime()
	}
	if req.EndTime != nil {
		endTime = req.EndTime.AsTime()
	}

	points, err := s.service.GetHallucinationMetrics(ctx, startTime, endTime)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get hallucination metrics: %v", err)
	}

	metrics := make([]*monitoringv1.HallucinationMetrics, len(points))
	for i, p := range points {
		metrics[i] = &monitoringv1.HallucinationMetrics{
			Time:     p.Time.Format(time.RFC3339),
			Rate:     p.Rate,
			Bias:     p.Bias,
			Toxicity: p.Toxicity,
		}
	}

	return &monitoringv1.GetHallucinationMetricsResponse{
		Metrics: metrics,
	}, nil
}

// GetTokenPerformance returns a time-series of token performance metrics.
func (s *Server) GetTokenPerformance(ctx context.Context, req *monitoringv1.GetTokenPerformanceRequest) (*monitoringv1.GetTokenPerformanceResponse, error) {
	ctx = withTenant(ctx)

	var startTime, endTime time.Time
	if req.StartTime != nil {
		startTime = req.StartTime.AsTime()
	}
	if req.EndTime != nil {
		endTime = req.EndTime.AsTime()
	}

	points, err := s.service.GetTokenPerformance(ctx, startTime, endTime)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get token performance: %v", err)
	}

	metrics := make([]*monitoringv1.TokenPerformance, len(points))
	for i, p := range points {
		metrics[i] = &monitoringv1.TokenPerformance{
			Time: p.Time.Format(time.RFC3339),
			Ttft: p.TTFT,
			Tpot: p.TPOT,
		}
	}

	return &monitoringv1.GetTokenPerformanceResponse{
		Metrics: metrics,
	}, nil
}
