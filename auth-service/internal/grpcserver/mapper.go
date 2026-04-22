// Package grpcserver provides the gRPC server implementation for auth-service.
package grpcserver

import (
	"database/sql"

	"google.golang.org/protobuf/types/known/timestamppb"

	"sovereign-ai-compliance/auth-service/model"
	authv1 "sovereign-ai-compliance/shared/proto/auth/v1"
)

// toProtoSafeUser converts model.SafeUser to proto SafeUser.
func toProtoSafeUser(u model.SafeUser) *authv1.SafeUser {
	return &authv1.SafeUser{
		Id:        u.ID.String(),
		TenantId:  u.TenantID.String(),
		Email:     u.Email,
		Role:      u.Role,
		IsActive:  u.IsActive,
		LastLogin: nullTimeToProto(u.LastLogin),
		CreatedAt: timestamppb.New(u.CreatedAt),
		UpdatedAt: timestamppb.New(u.UpdatedAt),
	}
}

// nullTimeToProto converts sql.NullTime to *timestamppb.Timestamp.
func nullTimeToProto(t sql.NullTime) *timestamppb.Timestamp {
	if !t.Valid {
		return nil
	}
	return timestamppb.New(t.Time)
}
