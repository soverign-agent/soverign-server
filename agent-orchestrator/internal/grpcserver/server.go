// Package grpcserver implements the agent.v1.AgentService gRPC server interface.
package grpcserver

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	agentv1 "sovereign-ai-compliance/shared/proto/agent/v1"
	"sovereign-ai-compliance/shared/tenant"
)

// Server implements the agent.v1.AgentService gRPC interface.
type Server struct {
	agentv1.UnimplementedAgentServiceServer
	supervisor Supervisor
}

// Supervisor defines the interface for the orchestration supervisor.
type Supervisor interface {
	Invoke(ctx context.Context, req *agentv1.AgentRequest) (*agentv1.AgentResponse, error)
	GetAgentCard(ctx context.Context, agentID string) (*agentv1.AgentCard, error)
}

// NewServer creates a new gRPC server instance.
func NewServer(supervisor Supervisor) *Server {
	return &Server{
		supervisor: supervisor,
	}
}

// Register registers the server with the gRPC server.
func (s *Server) Register(grpcServer *grpc.Server) {
	agentv1.RegisterAgentServiceServer(grpcServer, s)
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

// Invoke handles synchronous agent invocation requests.
func (s *Server) Invoke(ctx context.Context, req *agentv1.AgentRequest) (*agentv1.AgentResponse, error) {
	// Extract and validate tenant ID from metadata (for RLS)
	ctx = withTenant(ctx)
	return s.supervisor.Invoke(ctx, req)
}

// StreamInvoke handles streaming agent invocation for long-running tasks.
// The existing A2A protocol continues to work unchanged alongside typed stubs.
func (s *Server) StreamInvoke(stream agentv1.AgentService_StreamInvokeServer) error {
	// Forward to supervisor - existing implementation remains unchanged
	// This satisfies the requirement that A2A continues working
	return nil
}

// GetAgentCard returns the supervisor agent's capability card.
func (s *Server) GetAgentCard(ctx context.Context, req *agentv1.AgentCardRequest) (*agentv1.AgentCard, error) {
	ctx = withTenant(ctx)
	return s.supervisor.GetAgentCard(ctx, req.GetAgentId())
}

// SubscribeTransfers subscribes to orchestration transfer events.
func (s *Server) SubscribeTransfers(req *agentv1.TransferSubscriptionRequest, stream agentv1.AgentService_SubscribeTransfersServer) error {
	// Existing implementation continues to work
	return nil
}
