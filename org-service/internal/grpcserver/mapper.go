package grpcserver

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"sovereign-ai-compliance/org-service/model"
	"sovereign-ai-compliance/shared/proto/org/v1"
)

// toProtoTenant converts model.Tenant to proto.
func toProtoTenant(t *model.Tenant) *orgv1.Tenant {
	if t == nil {
		return nil
	}
	return &orgv1.Tenant{
		Id:        t.ID.String(),
		Name:      t.Name,
		Slug:      t.Slug,
		Domain:    t.Domain,
		Settings:  t.Settings,
		CreatedAt: timestamppb.New(t.CreatedAt),
		UpdatedAt: timestamppb.New(t.UpdatedAt),
	}
}

// toProtoSafeUser converts model.SafeUser to proto.
func toProtoSafeUser(u model.SafeUser) *orgv1.SafeUser {
	var lastLogin *timestamppb.Timestamp
	if u.LastLogin.Valid {
		lastLogin = timestamppb.New(u.LastLogin.Time)
	}
	return &orgv1.SafeUser{
		Id:        u.ID.String(),
		TenantId:  u.TenantID.String(),
		Email:     u.Email,
		Role:      u.Role,
		IsActive:  u.IsActive,
		LastLogin: lastLogin,
		CreatedAt: timestamppb.New(u.CreatedAt),
		UpdatedAt: timestamppb.New(u.UpdatedAt),
	}
}

// toProtoAISystem converts model.AISystem to proto.
func toProtoAISystem(s *model.AISystem) *orgv1.AISystem {
	if s == nil {
		return nil
	}
	return &orgv1.AISystem{
		Id:                 s.ID.String(),
		TenantId:           s.TenantID.String(),
		Name:               s.Name,
		Description:        s.Description,
		RiskClassification: s.RiskClassification,
		Status:             s.Status,
		Metadata:           s.Metadata,
		CreatedAt:          timestamppb.New(s.CreatedAt),
		UpdatedAt:          timestamppb.New(s.UpdatedAt),
	}
}

// toProtoCompliancePolicy converts model.CompliancePolicy to proto.
func toProtoCompliancePolicy(p *model.CompliancePolicy) *orgv1.CompliancePolicy {
	if p == nil {
		return nil
	}
	return &orgv1.CompliancePolicy{
		Id:         p.ID.String(),
		TenantId:   p.TenantID.String(),
		Name:       p.Name,
		PolicyType: p.PolicyType,
		Rules:      p.Rules,
		IsActive:   p.IsActive,
		CreatedAt:  timestamppb.New(p.CreatedAt),
		UpdatedAt:  timestamppb.New(p.UpdatedAt),
	}
}

// timeToProto converts *time.Time to proto timestamp.
func timeToProto(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}
