// Package types defines request and response types for the org service.
package types

import (
	"github.com/google/uuid"

	"sovereign-ai-compliance/org-service/model"
)

// Tenant request/response types
type TenantResponse struct {
	model.Tenant
}

type UpdateTenantRequest struct {
	TenantID uuid.UUID `json:"tenant_id" form:"tenant_id"`
	Name     string    `json:"name" form:"name"`
	Domain   string    `json:"domain" form:"domain"`
	Settings string    `json:"settings" form:"settings"`
}

// User request/response types
type InviteUserRequest struct {
	TenantID uuid.UUID `json:"tenant_id" form:"tenant_id"`
	Email    string    `json:"email" form:"email"`
	Role     string    `json:"role" form:"role"`
}

type UpdateUserRoleRequest struct {
	TenantID uuid.UUID `json:"tenant_id" form:"tenant_id"`
	UserID   uuid.UUID `json:"user_id" form:"user_id"`
	Role     string    `json:"role" form:"role"`
}

type ToggleUserRequest struct {
	TenantID uuid.UUID `json:"tenant_id" form:"tenant_id"`
	UserID   uuid.UUID `json:"user_id" form:"user_id"`
	IsActive bool      `json:"is_active" form:"is_active"`
}

type UsersResponse struct {
	Users []model.SafeUser `json:"users"`
}

// AI System request/response types
type CreateAISystemRequest struct {
	TenantID           uuid.UUID `json:"tenant_id" form:"tenant_id"`
	Name               string    `json:"name" form:"name"`
	Description        string    `json:"description" form:"description"`
	RiskClassification string    `json:"risk_classification" form:"risk_classification"`
	Status             string    `json:"status" form:"status"`
	Metadata           string    `json:"metadata" form:"metadata"`
}

type UpdateAISystemRequest struct {
	TenantID           uuid.UUID `json:"tenant_id" form:"tenant_id"`
	SystemID           uuid.UUID `json:"system_id" form:"system_id"`
	Name               string    `json:"name" form:"name"`
	Description        string    `json:"description" form:"description"`
	RiskClassification string    `json:"risk_classification" form:"risk_classification"`
	Status             string    `json:"status" form:"status"`
	Metadata           string    `json:"metadata" form:"metadata"`
}

type AISystemsResponse struct {
	Systems []model.AISystem `json:"systems"`
}

// Compliance Policy request/response types
type PolicyRequest struct {
	TenantID   uuid.UUID `json:"tenant_id" form:"tenant_id"`
	Name       string    `json:"name" form:"name"`
	PolicyType string    `json:"policy_type" form:"policy_type"`
	Rules      string    `json:"rules" form:"rules"`
}

type PolicyResponse struct {
	model.CompliancePolicy
}
