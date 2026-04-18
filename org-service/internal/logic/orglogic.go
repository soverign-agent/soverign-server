// Package logic implements the org-service business logic.
package logic

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"sovereign-ai-compliance/org-service/model"
	"sovereign-ai-compliance/org-service/repo"
)

// Org handles organization business logic.
type Org struct {
	repo repo.Repository
}

// NewOrg creates a new Org logic instance.
func NewOrg(r repo.Repository) *Org {
	return &Org{repo: r}
}

// --- Tenant Management ---

// GetTenant returns tenant information.
func (o *Org) GetTenant(ctx context.Context, tenantID uuid.UUID) (*model.Tenant, error) {
	return o.repo.GetTenant(ctx, tenantID)
}

// UpdateTenant updates tenant information.
func (o *Org) UpdateTenant(ctx context.Context, tenantID uuid.UUID, name, domain, settings string) (*model.Tenant, error) {
	tenant, err := o.repo.GetTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if name != "" {
		tenant.Name = name
	}
	if domain != "" {
		tenant.Domain = domain
	}
	if settings != "" {
		tenant.Settings = settings
	}
	tenant.UpdatedAt = time.Now()
	if err := o.repo.UpdateTenant(ctx, tenant); err != nil {
		return nil, fmt.Errorf("failed to update tenant: %w", err)
	}
	return tenant, nil
}

// --- User Management ---

// InviteUser creates a new user in a tenant.
func (o *Org) InviteUser(ctx context.Context, tenantID uuid.UUID, email, role string) (*model.SafeUser, error) {
	// In a real implementation, send invitation email and set a temporary password
	user := &model.User{
		ID:           uuid.Must(uuid.NewRandom()),
		TenantID:     tenantID,
		Email:        email,
		PasswordHash: "", // Will be set when user accepts invitation
		Role:         role,
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := o.repo.CreateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to invite user: %w", err)
	}
	safe := user.ToSafeUser()
	return &safe, nil
}

// ListUsers returns all users in a tenant.
func (o *Org) ListUsers(ctx context.Context, tenantID uuid.UUID) ([]model.SafeUser, error) {
	users, err := o.repo.ListUsers(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	var safeUsers []model.SafeUser
	for _, u := range users {
		safeUsers = append(safeUsers, u.ToSafeUser())
	}
	return safeUsers, nil
}

// UpdateUserRole updates a user's role.
func (o *Org) UpdateUserRole(ctx context.Context, tenantID, userID uuid.UUID, role string) (*model.SafeUser, error) {
	user, err := o.repo.GetUserByID(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}
	user.Role = role
	user.UpdatedAt = time.Now()
	if err := o.repo.UpdateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to update user role: %w", err)
	}
	safe := user.ToSafeUser()
	return &safe, nil
}

// ToggleUserActive enables or disables a user.
func (o *Org) ToggleUserActive(ctx context.Context, tenantID, userID uuid.UUID, isActive bool) (*model.SafeUser, error) {
	user, err := o.repo.GetUserByID(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}
	user.IsActive = isActive
	user.UpdatedAt = time.Now()
	if err := o.repo.UpdateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to update user status: %w", err)
	}
	safe := user.ToSafeUser()
	return &safe, nil
}

// --- AI System Management ---

// CreateAISystem creates a new AI system.
func (o *Org) CreateAISystem(ctx context.Context, tenantID uuid.UUID, name, description, riskClassification, status, metadata string) (*model.AISystem, error) {
	system := &model.AISystem{
		ID:                 uuid.Must(uuid.NewRandom()),
		TenantID:           tenantID,
		Name:               name,
		Description:        description,
		RiskClassification: riskClassification,
		Status:             status,
		Metadata:           metadata,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	if err := o.repo.CreateAISystem(ctx, system); err != nil {
		return nil, fmt.Errorf("failed to create ai system: %w", err)
	}
	return system, nil
}

// GetAISystem returns an AI system by ID.
func (o *Org) GetAISystem(ctx context.Context, tenantID, systemID uuid.UUID) (*model.AISystem, error) {
	return o.repo.GetAISystem(ctx, tenantID, systemID)
}

// ListAISystems returns all active AI systems for a tenant.
func (o *Org) ListAISystems(ctx context.Context, tenantID uuid.UUID) ([]model.AISystem, error) {
	return o.repo.ListAISystems(ctx, tenantID)
}

// UpdateAISystem updates an AI system.
func (o *Org) UpdateAISystem(ctx context.Context, tenantID, systemID uuid.UUID, name, description, riskClassification, status, metadata string) (*model.AISystem, error) {
	system, err := o.repo.GetAISystem(ctx, tenantID, systemID)
	if err != nil {
		return nil, err
	}
	if name != "" {
		system.Name = name
	}
	if description != "" {
		system.Description = description
	}
	if riskClassification != "" {
		system.RiskClassification = riskClassification
	}
	if status != "" {
		system.Status = status
	}
	if metadata != "" {
		system.Metadata = metadata
	}
	system.UpdatedAt = time.Now()
	if err := o.repo.UpdateAISystem(ctx, system); err != nil {
		return nil, fmt.Errorf("failed to update ai system: %w", err)
	}
	return system, nil
}

// DeleteAISystem soft deletes an AI system.
func (o *Org) DeleteAISystem(ctx context.Context, tenantID, systemID uuid.UUID) error {
	return o.repo.SoftDeleteAISystem(ctx, tenantID, systemID)
}

// --- Compliance Policy Management ---

// GetActivePolicy returns the current active compliance policy.
func (o *Org) GetActivePolicy(ctx context.Context, tenantID uuid.UUID) (*model.CompliancePolicy, error) {
	return o.repo.GetActivePolicy(ctx, tenantID)
}

// UpdatePolicy creates a new active policy and deactivates the old one.
func (o *Org) UpdatePolicy(ctx context.Context, tenantID uuid.UUID, name, policyType, rules string) (*model.CompliancePolicy, error) {
	// Deactivate old policies first
	if err := o.repo.DeactivateOldPolicies(ctx, tenantID); err != nil {
		return nil, fmt.Errorf("failed to deactivate old policies: %w", err)
	}

	policy := &model.CompliancePolicy{
		ID:         uuid.Must(uuid.NewRandom()),
		TenantID:   tenantID,
		Name:       name,
		PolicyType: policyType,
		Rules:      rules,
		IsActive:   true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := o.repo.CreatePolicy(ctx, policy); err != nil {
		return nil, fmt.Errorf("failed to create policy: %w", err)
	}
	return policy, nil
}
