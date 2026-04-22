// Package grpcserver provides the gRPC server implementation for auth-service.
package grpcserver

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"sovereign-ai-compliance/auth-service/internal/logic"
	authv1 "sovereign-ai-compliance/shared/proto/auth/v1"
)

// Server implements authv1.AuthServiceServer.
type Server struct {
	authv1.UnimplementedAuthServiceServer

	logic *logic.Auth
}

// NewServer creates a new gRPC server for auth-service.
func NewServer(logic *logic.Auth) *Server {
	return &Server{
		logic: logic,
	}
}

// Register registers the server on the provided gRPC server.
func (s *Server) Register(grpcServer *grpc.Server) {
	authv1.RegisterAuthServiceServer(grpcServer, s)
}

// Login authenticates a user and returns tokens.
func (s *Server) Login(ctx context.Context, req *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	if req.Email == "" {
		return nil, status.Errorf(codes.InvalidArgument, "email is required")
	}
	if req.Password == "" {
		return nil, status.Errorf(codes.InvalidArgument, "password is required")
	}

	var tenantID uuid.UUID
	var err error
	if req.TenantId != "" {
		tenantID, err = uuid.Parse(req.TenantId)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid tenant_id: %v", err)
		}
	}

	pair, user, err := s.logic.Login(ctx, tenantID, req.Email, req.Password)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "login failed: %v", err)
	}

	return &authv1.LoginResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		User:         toProtoSafeUser(*user),
	}, nil
}

// RefreshToken exchanges a refresh token for a new token pair.
func (s *Server) RefreshToken(ctx context.Context, req *authv1.RefreshRequest) (*authv1.TokenResponse, error) {
	if req.RefreshToken == "" {
		return nil, status.Errorf(codes.InvalidArgument, "refresh_token is required")
	}

	pair, err := s.logic.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "refresh failed: %v", err)
	}

	return &authv1.TokenResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
	}, nil
}

// Logout revokes all refresh tokens for a user.
func (s *Server) Logout(ctx context.Context, req *authv1.LogoutRequest) (*authv1.LogoutResponse, error) {
	if req.UserId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "user_id is required")
	}

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id: %v", err)
	}

	if err := s.logic.Logout(ctx, userID); err != nil {
		return nil, status.Errorf(codes.Internal, "logout failed: %v", err)
	}

	return &authv1.LogoutResponse{Message: "logged out successfully"}, nil
}

// RequestPasswordReset initiates a password reset flow.
func (s *Server) RequestPasswordReset(ctx context.Context, req *authv1.RequestPasswordResetRequest) (*authv1.RequestPasswordResetResponse, error) {
	if req.Email == "" {
		return nil, status.Errorf(codes.InvalidArgument, "email is required")
	}

	var tenantID uuid.UUID
	var err error
	if req.TenantId != "" {
		tenantID, err = uuid.Parse(req.TenantId)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid tenant_id: %v", err)
		}
	}

	if err := s.logic.RequestPasswordReset(ctx, tenantID, req.Email); err != nil {
		return nil, status.Errorf(codes.Internal, "request password reset failed: %v", err)
	}

	return &authv1.RequestPasswordResetResponse{Message: "if the email exists, a reset link has been sent"}, nil
}

// ConfirmPasswordReset completes a password reset.
func (s *Server) ConfirmPasswordReset(ctx context.Context, req *authv1.ConfirmPasswordResetRequest) (*authv1.ConfirmPasswordResetResponse, error) {
	if req.Token == "" {
		return nil, status.Errorf(codes.InvalidArgument, "token is required")
	}
	if req.NewPassword == "" {
		return nil, status.Errorf(codes.InvalidArgument, "new_password is required")
	}

	if err := s.logic.ResetPassword(ctx, req.Token, req.NewPassword); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "confirm password reset failed: %v", err)
	}

	return &authv1.ConfirmPasswordResetResponse{Message: "password reset successful"}, nil
}

// GetMe returns the current authenticated user.
func (s *Server) GetMe(ctx context.Context, req *authv1.GetMeRequest) (*authv1.SafeUser, error) {
	if req.TenantId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "tenant_id is required")
	}
	if req.UserId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "user_id is required")
	}

	tenantID, err := uuid.Parse(req.TenantId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid tenant_id: %v", err)
	}

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id: %v", err)
	}

	user, err := s.logic.Me(ctx, tenantID, userID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "user not found: %v", err)
	}

	return toProtoSafeUser(*user), nil
}

// ValidateToken checks whether an access token is valid and returns its claims.
func (s *Server) ValidateToken(ctx context.Context, req *authv1.ValidateTokenRequest) (*authv1.ValidateTokenResponse, error) {
	if req.AccessToken == "" {
		return nil, status.Errorf(codes.InvalidArgument, "access_token is required")
	}

	userID, tenantID, role, err := s.logic.ValidateToken(req.AccessToken)
	if err != nil {
		return &authv1.ValidateTokenResponse{Valid: false}, nil
	}

	return &authv1.ValidateTokenResponse{
		UserId:   userID,
		TenantId: tenantID,
		Role:     role,
		Valid:    true,
	}, nil
}

// HasPermission checks whether a role has permission for a resource action.
func (s *Server) HasPermission(ctx context.Context, req *authv1.HasPermissionRequest) (*authv1.HasPermissionResponse, error) {
	if req.Role == "" {
		return nil, status.Errorf(codes.InvalidArgument, "role is required")
	}
	if req.Resource == "" {
		return nil, status.Errorf(codes.InvalidArgument, "resource is required")
	}
	if req.Action == "" {
		return nil, status.Errorf(codes.InvalidArgument, "action is required")
	}

	granted := s.logic.HasPermission(req.Role, req.Resource, req.Action)
	return &authv1.HasPermissionResponse{Granted: granted}, nil
}
