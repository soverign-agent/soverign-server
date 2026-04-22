// Package grpcserver provides the gRPC server implementation for repo-service.
package grpcserver

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"sovereign-ai-compliance/repo-service/internal/logic"
	"sovereign-ai-compliance/repo-service/internal/types"
	"sovereign-ai-compliance/shared/proto/repo/v1"
	"sovereign-ai-compliance/shared/tenant"
)

// Server implements repov1.RepoServiceServer.
type Server struct {
	repov1.UnimplementedRepoServiceServer

	logic *logic.RepositoryLogic
}

// NewServer creates a new gRPC server for repo-service.
func NewServer(logic *logic.RepositoryLogic) *Server {
	return &Server{
		logic: logic,
	}
}

// Register registers the server on the provided gRPC server.
func (s *Server) Register(grpcServer *grpc.Server) {
	repov1.RegisterRepoServiceServer(grpcServer, s)
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

// ListRepositories lists all repositories for a tenant.
func (s *Server) ListRepositories(ctx context.Context, req *repov1.ListRepositoriesRequest) (*repov1.ListRepositoriesResponse, error) {
	ctx = withTenant(ctx)

	tenantIDStr, ok := tenant.FromContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "tenant context required")
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid tenant_id: %v", err)
	}

	repos, err := s.logic.List(ctx, tenantID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list repositories: %v", err)
	}

	protoRepos := make([]*repov1.SafeRepository, len(repos))
	for i, r := range repos {
		protoRepos[i] = toProtoSafeRepository(r)
	}

	return &repov1.ListRepositoriesResponse{
		Repositories: protoRepos,
	}, nil
}

// GetRepository gets a single repository by ID.
func (s *Server) GetRepository(ctx context.Context, req *repov1.GetRepositoryRequest) (*repov1.SafeRepository, error) {
	ctx = withTenant(ctx)

	tenantIDStr, ok := tenant.FromContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "tenant context required")
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid tenant_id: %v", err)
	}

	repoID, err := uuid.Parse(req.RepositoryId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid repository_id: %v", err)
	}

	repo, err := s.logic.GetByID(ctx, tenantID, repoID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get repository: %v", err)
	}
	if repo == nil {
		return nil, status.Errorf(codes.NotFound, "repository not found")
	}

	safe := repo.ToSafe()
	return toProtoSafeRepository(safe), nil
}

// CreateRepository creates a new repository connection.
func (s *Server) CreateRepository(ctx context.Context, req *repov1.CreateRepositoryRequest) (*repov1.CreateRepositoryResponse, error) {
	ctx = withTenant(ctx)

	tenantIDStr, ok := tenant.FromContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "tenant context required")
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid tenant_id: %v", err)
	}

	if req.Name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "name is required")
	}
	if req.Url == "" {
		return nil, status.Errorf(codes.InvalidArgument, "url is required")
	}

	createReq := &types.CreateRepositoryRequest{
		Name:          req.Name,
		URL:           req.Url,
		Provider:      types.Provider(string(providerFromProto(req.Provider))),
		Username:      strPtr(req.Username),
		Token:         strPtr(req.Token),
		WebhookSecret: strPtr(req.WebhookSecret),
		DefaultBranch: req.DefaultBranch,
	}

	repo, err := s.logic.Create(ctx, tenantID, createReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create repository: %v", err)
	}

	return &repov1.CreateRepositoryResponse{
		Repository: toProtoSafeRepository(*repo),
	}, nil
}

// DeleteRepository deletes a repository connection.
func (s *Server) DeleteRepository(ctx context.Context, req *repov1.DeleteRepositoryRequest) (*repov1.DeleteRepositoryResponse, error) {
	ctx = withTenant(ctx)

	tenantIDStr, ok := tenant.FromContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "tenant context required")
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid tenant_id: %v", err)
	}

	repoID, err := uuid.Parse(req.RepositoryId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid repository_id: %v", err)
	}

	success, err := s.logic.Delete(ctx, tenantID, repoID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "delete repository: %v", err)
	}

	return &repov1.DeleteRepositoryResponse{
		Success: success,
	}, nil
}

// TestConnection tests a repository connection.
func (s *Server) TestConnection(ctx context.Context, req *repov1.TestConnectionRequest) (*repov1.TestConnectionResponse, error) {
	ctx = withTenant(ctx)

	tenantIDStr, ok := tenant.FromContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "tenant context required")
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid tenant_id: %v", err)
	}

	repoID, err := uuid.Parse(req.RepositoryId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid repository_id: %v", err)
	}

	success, message, err := s.logic.TestConnection(ctx, tenantID, repoID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "test connection: %v", err)
	}

	return &repov1.TestConnectionResponse{
		Success: success,
		Message: message,
	}, nil
}

// TriggerScan triggers a code scan for a repository.
func (s *Server) TriggerScan(ctx context.Context, req *repov1.TriggerScanRequest) (*repov1.TriggerScanResponse, error) {
	ctx = withTenant(ctx)

	tenantIDStr, ok := tenant.FromContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "tenant context required")
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid tenant_id: %v", err)
	}

	repoID, err := uuid.Parse(req.RepositoryId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid repository_id: %v", err)
	}

	branch := req.Branch
	if branch == "" {
		branch = ""
	}

	scan, err := s.logic.FullScan(ctx, tenantID, repoID, branch)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "trigger scan: %v", err)
	}

	return &repov1.TriggerScanResponse{
		ScanId: scan.ID.String(),
		Status: "completed",
	}, nil
}

// ProcessWebhook handles incoming webhook from Git provider.
func (s *Server) ProcessWebhook(ctx context.Context, req *repov1.ProcessWebhookRequest) (*repov1.ProcessWebhookResponse, error) {
	repoID, err := uuid.Parse(req.RepositoryId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid repository_id: %v", err)
	}

	// Extract signature from metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "missing metadata")
	}

	signature := ""
	if vals := md.Get("x-hub-signature-256"); len(vals) > 0 {
		signature = vals[0]
	} else if vals := md.Get("x-gitlab-token"); len(vals) > 0 {
		signature = vals[0]
	}

	if signature == "" {
		return nil, status.Errorf(codes.Unauthenticated, "missing webhook signature")
	}

	// Resolve tenant ownership from the repository record
	repository, err := s.logic.GetByIDAnyTenant(ctx, repoID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get repository: %v", err)
	}
	if repository == nil {
		return nil, status.Errorf(codes.NotFound, "repository not found")
	}
	if len(repository.WebhookSecret) == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "webhook secret required")
	}

	// Verify webhook signature
	valid, err := s.logic.VerifyWebhookSignature(ctx, repository, req.Payload, signature, repository.Provider)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "verify signature: %v", err)
	}
	if !valid {
		return nil, status.Errorf(codes.Unauthenticated, "invalid webhook signature")
	}

	// Trigger scan
	branch := branchFromWebhookPayload(req.Payload)
	scan, err := s.logic.FullScan(ctx, repository.TenantID, repoID, branch)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "trigger scan: %v", err)
	}

	return &repov1.ProcessWebhookResponse{
		Success: true,
		Message: scan.ID.String(),
	}, nil
}

// ListScanResults lists all scan results for a repository.
func (s *Server) ListScanResults(ctx context.Context, req *repov1.ListScanResultsRequest) (*repov1.ListScanResultsResponse, error) {
	repoID, err := uuid.Parse(req.RepositoryId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid repository_id: %v", err)
	}

	results, err := s.logic.ListScanResults(ctx, repoID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list scan results: %v", err)
	}

	protoResults := make([]*repov1.ScanResult, len(results))
	for i, r := range results {
		protoResults[i] = toProtoScanResult(r)
	}

	return &repov1.ListScanResultsResponse{
		ScanResults: protoResults,
	}, nil
}

// GetScanResult gets a specific scan result.
func (s *Server) GetScanResult(ctx context.Context, req *repov1.GetScanResultRequest) (*repov1.ScanResult, error) {
	ctx = withTenant(ctx)

	tenantIDStr, ok := tenant.FromContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "tenant context required")
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid tenant_id: %v", err)
	}

	scanID, err := uuid.Parse(req.ScanId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid scan_id: %v", err)
	}

	scan, err := s.logic.GetScanResult(ctx, tenantID, scanID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get scan result: %v", err)
	}
	if scan == nil {
		return nil, status.Errorf(codes.NotFound, "scan result not found")
	}

	return toProtoScanResult(*scan), nil
}

// branchFromWebhookPayload extracts the branch name from a webhook payload.
func branchFromWebhookPayload(payload []byte) string {
	var event struct {
		Ref string `json:"ref"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return ""
	}

	const headsPrefix = "refs/heads/"
	if strings.HasPrefix(event.Ref, headsPrefix) {
		return strings.TrimPrefix(event.Ref, headsPrefix)
	}
	return event.Ref
}
