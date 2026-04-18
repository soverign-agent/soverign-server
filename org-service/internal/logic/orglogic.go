// Package logic implements the org-service business logic.
package logic

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"

	"sovereign-ai-compliance/org-service/model"
	"sovereign-ai-compliance/org-service/repo"
)

// Validation constants.
const (
	maxEmailLength     = 254
	maxNameLength      = 255
	maxDescriptionLength = 1000
	maxDomainLength    = 255
	maxSettingsLength  = 10000
	maxMetadataLength  = 10000
	maxRulesLength     = 50000
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// Valid roles.
var validRoles = map[string]bool{
	"admin":    true,
	"auditor":  true,
	"reviewer": true,
}

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
	// Validate input
	if name != "" && len(name) > maxNameLength {
		return nil, fmt.Errorf("name exceeds maximum length of %d characters", maxNameLength)
	}
	if domain != "" && len(domain) > maxDomainLength {
		return nil, fmt.Errorf("domain exceeds maximum length of %d characters", maxDomainLength)
	}
	if settings != "" && len(settings) > maxSettingsLength {
		return nil, fmt.Errorf("settings exceeds maximum length of %d characters", maxSettingsLength)
	}

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
	// Validate input
	if len(email) > maxEmailLength {
		return nil, fmt.Errorf("email exceeds maximum length of %d characters", maxEmailLength)
	}
	if !emailRegex.MatchString(email) {
		return nil, fmt.Errorf("invalid email format")
	}
	if !validRoles[role] {
		return nil, fmt.Errorf("invalid role: must be one of [admin, auditor, reviewer]")
	}

	// In a real implementation, send invitation email and set a temporary password
	user := &model.User{
		ID:           uuid.Must(uuid.NewRandom()),
		TenantID:     tenantID,
		Email:        email,
		PasswordHash: "", // Will be set when user accepts invitation - Verify handles this securely
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
	// Validate input
	if !validRoles[role] {
		return nil, fmt.Errorf("invalid role: must be one of [admin, auditor, reviewer]")
	}

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
	// Validate input
	if len(name) > maxNameLength {
		return nil, fmt.Errorf("name exceeds maximum length of %d characters", maxNameLength)
	}
	if len(description) > maxDescriptionLength {
		return nil, fmt.Errorf("description exceeds maximum length of %d characters", maxDescriptionLength)
	}
	if len(metadata) > maxMetadataLength {
		return nil, fmt.Errorf("metadata exceeds maximum length of %d characters", maxMetadataLength)
	}

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
	// Validate input
	if name != "" && len(name) > maxNameLength {
		return nil, fmt.Errorf("name exceeds maximum length of %d characters", maxNameLength)
	}
	if description != "" && len(description) > maxDescriptionLength {
		return nil, fmt.Errorf("description exceeds maximum length of %d characters", maxDescriptionLength)
	}
	if metadata != "" && len(metadata) > maxMetadataLength {
		return nil, fmt.Errorf("metadata exceeds maximum length of %d characters", maxMetadataLength)
	}

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
	// Validate input
	if len(name) > maxNameLength {
		return nil, fmt.Errorf("name exceeds maximum length of %d characters", maxNameLength)
	}
	if len(rules) > maxRulesLength {
		return nil, fmt.Errorf("rules exceeds maximum length of %d characters", maxRulesLength)
	}

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
