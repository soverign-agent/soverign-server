// Package repo defines the organization repository interface.
package repo

import (
	"context"

	"github.com/google/uuid"

	"sovereign-ai-compliance/org-service/model"
)

// Repository defines the data access interface for org operations.
type Repository interface {
	// Tenant operations
	GetTenant(ctx context.Context, tenantID uuid.UUID) (*model.Tenant, error)
	CreateTenant(ctx context.Context, tenant *model.Tenant) error
	UpdateTenant(ctx context.Context, tenant *model.Tenant) error

	// User operations
	GetUserByID(ctx context.Context, tenantID, userID uuid.UUID) (*model.User, error)
	ListUsers(ctx context.Context, tenantID uuid.UUID) ([]model.User, error)
	CreateUser(ctx context.Context, user *model.User) error
	UpdateUser(ctx context.Context, user *model.User) error

	// AI System operations
	GetAISystem(ctx context.Context, tenantID, systemID uuid.UUID) (*model.AISystem, error)
	ListAISystems(ctx context.Context, tenantID uuid.UUID) ([]model.AISystem, error)
	CreateAISystem(ctx context.Context, system *model.AISystem) error
	UpdateAISystem(ctx context.Context, system *model.AISystem) error
	SoftDeleteAISystem(ctx context.Context, tenantID, systemID uuid.UUID) error

	// Compliance Policy operations
	GetActivePolicy(ctx context.Context, tenantID uuid.UUID) (*model.CompliancePolicy, error)
	GetPolicyByID(ctx context.Context, tenantID, policyID uuid.UUID) (*model.CompliancePolicy, error)
	CreatePolicy(ctx context.Context, policy *model.CompliancePolicy) error
	UpdatePolicy(ctx context.Context, policy *model.CompliancePolicy) error
	DeactivateOldPolicies(ctx context.Context, tenantID uuid.UUID) error
}
