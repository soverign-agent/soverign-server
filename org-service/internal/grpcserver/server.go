// Package grpcserver provides the gRPC server implementation for org-service.
package grpcserver

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"sovereign-ai-compliance/org-service/internal/logic"
	"sovereign-ai-compliance/shared/proto/org/v1"
	"sovereign-ai-compliance/shared/tenant"
)

// Server implements orgv1.OrgServiceServer.
type Server struct {
	orgv1.UnimplementedOrgServiceServer

	logic *logic.Org
}

// NewServer creates a new gRPC server for org-service.
func NewServer(logic *logic.Org) *Server {
	return &Server{
		logic: logic,
	}
}

// Register registers the server on the provided gRPC server.
func (s *Server) Register(grpcServer *grpc.Server) {
	orgv1.RegisterOrgServiceServer(grpcServer, s)
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

// tenantIDFromContext extracts the tenant ID from context.
func tenantIDFromContext(ctx context.Context) (uuid.UUID, error) {
	tenantID, ok := tenant.FromContext(ctx)
	if !ok || tenantID == "" {
		return uuid.Nil, status.Errorf(codes.Unauthenticated, "tenant id required")
	}
	id, err := uuid.Parse(tenantID)
	if err != nil {
		return uuid.Nil, status.Errorf(codes.InvalidArgument, "invalid tenant id: %v", err)
	}
	return id, nil
}

// GetTenant returns tenant information.
func (s *Server) GetTenant(ctx context.Context, req *orgv1.GetTenantRequest) (*orgv1.Tenant, error) {
	ctx = withTenant(ctx)

	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	t, err := s.logic.GetTenant(ctx, tenantID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get tenant: %v", err)
	}
	if t == nil {
		return nil, status.Errorf(codes.NotFound, "tenant not found")
	}

	return toProtoTenant(t), nil
}

// UpdateTenant updates tenant information.
func (s *Server) UpdateTenant(ctx context.Context, req *orgv1.UpdateTenantRequest) (*orgv1.Tenant, error) {
	ctx = withTenant(ctx)

	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	t, err := s.logic.UpdateTenant(ctx, tenantID, req.Name, req.Domain, req.Settings)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "update tenant: %v", err)
	}

	return toProtoTenant(t), nil
}

// ListUsers returns all users in a tenant.
func (s *Server) ListUsers(ctx context.Context, req *orgv1.ListUsersRequest) (*orgv1.ListUsersResponse, error) {
	ctx = withTenant(ctx)

	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	users, err := s.logic.ListUsers(ctx, tenantID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list users: %v", err)
	}

	protoUsers := make([]*orgv1.SafeUser, len(users))
	for i, u := range users {
		protoUsers[i] = toProtoSafeUser(u)
	}

	return &orgv1.ListUsersResponse{Users: protoUsers}, nil
}

// InviteUser creates a new user in a tenant.
func (s *Server) InviteUser(ctx context.Context, req *orgv1.InviteUserRequest) (*orgv1.SafeUser, error) {
	ctx = withTenant(ctx)

	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if req.Email == "" {
		return nil, status.Errorf(codes.InvalidArgument, "email is required")
	}
	if req.Role == "" {
		return nil, status.Errorf(codes.InvalidArgument, "role is required")
	}

	user, err := s.logic.InviteUser(ctx, tenantID, req.Email, req.Role)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "invite user: %v", err)
	}

	return toProtoSafeUser(*user), nil
}

// UpdateUserRole updates a user's role.
func (s *Server) UpdateUserRole(ctx context.Context, req *orgv1.UpdateUserRoleRequest) (*orgv1.SafeUser, error) {
	ctx = withTenant(ctx)

	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id: %v", err)
	}
	if req.Role == "" {
		return nil, status.Errorf(codes.InvalidArgument, "role is required")
	}

	user, err := s.logic.UpdateUserRole(ctx, tenantID, userID, req.Role)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "update user role: %v", err)
	}

	return toProtoSafeUser(*user), nil
}

// ToggleUser activates or deactivates a user.
func (s *Server) ToggleUser(ctx context.Context, req *orgv1.ToggleUserRequest) (*orgv1.SafeUser, error) {
	ctx = withTenant(ctx)

	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id: %v", err)
	}

	user, err := s.logic.ToggleUserActive(ctx, tenantID, userID, req.IsActive)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "toggle user: %v", err)
	}

	return toProtoSafeUser(*user), nil
}

// ListAISystems returns all active AI systems for a tenant.
func (s *Server) ListAISystems(ctx context.Context, req *orgv1.ListAISystemsRequest) (*orgv1.ListAISystemsResponse, error) {
	ctx = withTenant(ctx)

	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	systems, err := s.logic.ListAISystems(ctx, tenantID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list ai systems: %v", err)
	}

	protoSystems := make([]*orgv1.AISystem, len(systems))
	for i, sys := range systems {
		protoSystems[i] = toProtoAISystem(&sys)
	}

	return &orgv1.ListAISystemsResponse{Systems: protoSystems}, nil
}

// GetAISystem returns a single AI system by ID.
func (s *Server) GetAISystem(ctx context.Context, req *orgv1.GetAISystemRequest) (*orgv1.AISystem, error) {
	ctx = withTenant(ctx)

	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	systemID, err := uuid.Parse(req.SystemId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid system_id: %v", err)
	}

	system, err := s.logic.GetAISystem(ctx, tenantID, systemID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get ai system: %v", err)
	}
	if system == nil {
		return nil, status.Errorf(codes.NotFound, "ai system not found")
	}

	return toProtoAISystem(system), nil
}

// buildMetadataJSON constructs a valid JSON metadata string from repository_url and existing metadata.
func buildMetadataJSON(existingMeta, repositoryURL string) string {
	meta := make(map[string]interface{})
	if existingMeta != "" {
		_ = json.Unmarshal([]byte(existingMeta), &meta)
	}
	if repositoryURL != "" {
		meta["repository_url"] = repositoryURL
	}
	b, _ := json.Marshal(meta)
	return string(b)
}

// CreateAISystem creates a new AI system.
func (s *Server) CreateAISystem(ctx context.Context, req *orgv1.CreateAISystemRequest) (*orgv1.AISystem, error) {
	ctx = withTenant(ctx)

	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if req.Name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "name is required")
	}

	riskClass := req.RiskClassification
	if riskClass == "" {
		riskClass = "limited"
	}
	statusStr := req.Status
	if statusStr == "" {
		statusStr = "draft"
	}
	metadata := buildMetadataJSON(req.Metadata, req.RepositoryUrl)

	system, err := s.logic.CreateAISystem(ctx, tenantID, req.Name, req.Description, riskClass, statusStr, metadata)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create ai system: %v", err)
	}

	return toProtoAISystem(system), nil
}

// UpdateAISystem updates an AI system.
func (s *Server) UpdateAISystem(ctx context.Context, req *orgv1.UpdateAISystemRequest) (*orgv1.AISystem, error) {
	ctx = withTenant(ctx)

	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	systemID, err := uuid.Parse(req.SystemId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid system_id: %v", err)
	}

	metadata := buildMetadataJSON(req.Metadata, req.RepositoryUrl)

	system, err := s.logic.UpdateAISystem(ctx, tenantID, systemID, req.Name, req.Description, req.RiskClassification, req.Status, metadata)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "update ai system: %v", err)
	}

	return toProtoAISystem(system), nil
}

// DeleteAISystem soft deletes an AI system.
func (s *Server) DeleteAISystem(ctx context.Context, req *orgv1.DeleteAISystemRequest) (*orgv1.DeleteAISystemResponse, error) {
	ctx = withTenant(ctx)

	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	systemID, err := uuid.Parse(req.SystemId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid system_id: %v", err)
	}

	if err := s.logic.DeleteAISystem(ctx, tenantID, systemID); err != nil {
		return nil, status.Errorf(codes.Internal, "delete ai system: %v", err)
	}

	return &orgv1.DeleteAISystemResponse{Success: true}, nil
}

// GetActivePolicy returns the current active compliance policy.
func (s *Server) GetActivePolicy(ctx context.Context, req *orgv1.GetActivePolicyRequest) (*orgv1.CompliancePolicy, error) {
	ctx = withTenant(ctx)

	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	policy, err := s.logic.GetActivePolicy(ctx, tenantID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get active policy: %v", err)
	}
	if policy == nil {
		return nil, status.Errorf(codes.NotFound, "active policy not found")
	}

	return toProtoCompliancePolicy(policy), nil
}

// UpdatePolicy creates a new active policy and deactivates the old one.
func (s *Server) UpdatePolicy(ctx context.Context, req *orgv1.UpdatePolicyRequest) (*orgv1.CompliancePolicy, error) {
	ctx = withTenant(ctx)

	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if req.Name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "name is required")
	}

	policy, err := s.logic.UpdatePolicy(ctx, tenantID, req.Name, req.PolicyType, req.Rules)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "update policy: %v", err)
	}

	return toProtoCompliancePolicy(policy), nil
}
