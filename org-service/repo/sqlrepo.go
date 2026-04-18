// Package repo provides a concrete PostgreSQL implementation of Repository.
package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"sovereign-ai-compliance/org-service/model"
)

// sqlRepository implements Repository using database/sql.
type sqlRepository struct {
	db *sql.DB
}

// NewSQLRepository creates a new PostgreSQL-backed repository.
func NewSQLRepository(db *sql.DB) Repository {
	return &sqlRepository{db: db}
}

func (r *sqlRepository) GetTenant(ctx context.Context, tenantID uuid.UUID) (*model.Tenant, error) {
	var t model.Tenant
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, slug, domain, settings, created_at, updated_at FROM tenants WHERE id = $1`,
		tenantID,
	).Scan(&t.ID, &t.Name, &t.Slug, &t.Domain, &t.Settings, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("tenant not found")
		}
		return nil, fmt.Errorf("query tenant: %w", err)
	}
	return &t, nil
}

func (r *sqlRepository) CreateTenant(ctx context.Context, tenant *model.Tenant) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO tenants (id, name, slug, domain, settings, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		tenant.ID, tenant.Name, tenant.Slug, tenant.Domain, tenant.Settings, tenant.CreatedAt, tenant.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert tenant: %w", err)
	}
	return nil
}

func (r *sqlRepository) UpdateTenant(ctx context.Context, tenant *model.Tenant) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE tenants SET name = $1, slug = $2, domain = $3, settings = $4, updated_at = $5 WHERE id = $6`,
		tenant.Name, tenant.Slug, tenant.Domain, tenant.Settings, time.Now(), tenant.ID,
	)
	if err != nil {
		return fmt.Errorf("update tenant: %w", err)
	}
	return nil
}

func (r *sqlRepository) GetUserByID(ctx context.Context, tenantID, userID uuid.UUID) (*model.User, error) {
	var u model.User
	var lastLogin sql.NullTime
	err := r.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, email, password_hash, role, is_active, last_login, created_at, updated_at
		 FROM users WHERE tenant_id = $1 AND id = $2`,
		tenantID, userID,
	).Scan(&u.ID, &u.TenantID, &u.Email, &u.PasswordHash, &u.Role, &u.IsActive, &lastLogin, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("query user: %w", err)
	}
	u.LastLogin = lastLogin
	return &u, nil
}

func (r *sqlRepository) ListUsers(ctx context.Context, tenantID uuid.UUID) ([]model.User, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, tenant_id, email, password_hash, role, is_active, last_login, created_at, updated_at
		 FROM users WHERE tenant_id = $1 ORDER BY created_at DESC`,
		tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("query users: %w", err)
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		var u model.User
		var lastLogin sql.NullTime
		if err := rows.Scan(&u.ID, &u.TenantID, &u.Email, &u.PasswordHash, &u.Role, &u.IsActive, &lastLogin, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		u.LastLogin = lastLogin
		users = append(users, u)
	}
	return users, rows.Err()
}

func (r *sqlRepository) CreateUser(ctx context.Context, user *model.User) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO users (id, tenant_id, email, password_hash, role, is_active, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		user.ID, user.TenantID, user.Email, user.PasswordHash, user.Role, user.IsActive, user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}

func (r *sqlRepository) UpdateUser(ctx context.Context, user *model.User) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET email = $1, password_hash = $2, role = $3, is_active = $4, updated_at = $5
		 WHERE id = $6 AND tenant_id = $7`,
		user.Email, user.PasswordHash, user.Role, user.IsActive, time.Now(), user.ID, user.TenantID,
	)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}

func (r *sqlRepository) GetAISystem(ctx context.Context, tenantID, systemID uuid.UUID) (*model.AISystem, error) {
	var s model.AISystem
	var deletedAt sql.NullTime
	err := r.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, name, description, risk_classification, status, metadata, created_at, updated_at, deleted_at
		 FROM ai_systems WHERE tenant_id = $1 AND id = $2`,
		tenantID, systemID,
	).Scan(&s.ID, &s.TenantID, &s.Name, &s.Description, &s.RiskClassification, &s.Status, &s.Metadata, &s.CreatedAt, &s.UpdatedAt, &deletedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("ai system not found")
		}
		return nil, fmt.Errorf("query ai system: %w", err)
	}
	s.DeletedAt = deletedAt
	return &s, nil
}

func (r *sqlRepository) ListAISystems(ctx context.Context, tenantID uuid.UUID) ([]model.AISystem, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, tenant_id, name, description, risk_classification, status, metadata, created_at, updated_at, deleted_at
		 FROM ai_systems WHERE tenant_id = $1 AND deleted_at IS NULL ORDER BY created_at DESC`,
		tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("query ai systems: %w", err)
	}
	defer rows.Close()

	var systems []model.AISystem
	for rows.Next() {
		var s model.AISystem
		var deletedAt sql.NullTime
		if err := rows.Scan(&s.ID, &s.TenantID, &s.Name, &s.Description, &s.RiskClassification, &s.Status, &s.Metadata, &s.CreatedAt, &s.UpdatedAt, &deletedAt); err != nil {
			return nil, fmt.Errorf("scan ai system: %w", err)
		}
		s.DeletedAt = deletedAt
		systems = append(systems, s)
	}
	return systems, rows.Err()
}

func (r *sqlRepository) CreateAISystem(ctx context.Context, system *model.AISystem) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO ai_systems (id, tenant_id, name, description, risk_classification, status, metadata, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		system.ID, system.TenantID, system.Name, system.Description, system.RiskClassification, system.Status, system.Metadata, system.CreatedAt, system.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert ai system: %w", err)
	}
	return nil
}

func (r *sqlRepository) UpdateAISystem(ctx context.Context, system *model.AISystem) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE ai_systems SET name = $1, description = $2, risk_classification = $3, status = $4, metadata = $5, updated_at = $6
		 WHERE id = $7 AND tenant_id = $8`,
		system.Name, system.Description, system.RiskClassification, system.Status, system.Metadata, time.Now(), system.ID, system.TenantID,
	)
	if err != nil {
		return fmt.Errorf("update ai system: %w", err)
	}
	return nil
}

func (r *sqlRepository) SoftDeleteAISystem(ctx context.Context, tenantID, systemID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE ai_systems SET deleted_at = $1, updated_at = $2 WHERE id = $3 AND tenant_id = $4`,
		time.Now(), time.Now(), systemID, tenantID,
	)
	if err != nil {
		return fmt.Errorf("soft delete ai system: %w", err)
	}
	return nil
}

func (r *sqlRepository) GetActivePolicy(ctx context.Context, tenantID uuid.UUID) (*model.CompliancePolicy, error) {
	var p model.CompliancePolicy
	err := r.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, name, policy_type, rules, is_active, created_at, updated_at
		 FROM compliance_policies WHERE tenant_id = $1 AND is_active = true ORDER BY updated_at DESC LIMIT 1`,
		tenantID,
	).Scan(&p.ID, &p.TenantID, &p.Name, &p.PolicyType, &p.Rules, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no active policy found")
		}
		return nil, fmt.Errorf("query active policy: %w", err)
	}
	return &p, nil
}

func (r *sqlRepository) GetPolicyByID(ctx context.Context, tenantID, policyID uuid.UUID) (*model.CompliancePolicy, error) {
	var p model.CompliancePolicy
	err := r.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, name, policy_type, rules, is_active, created_at, updated_at
		 FROM compliance_policies WHERE tenant_id = $1 AND id = $2`,
		tenantID, policyID,
	).Scan(&p.ID, &p.TenantID, &p.Name, &p.PolicyType, &p.Rules, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("policy not found")
		}
		return nil, fmt.Errorf("query policy: %w", err)
	}
	return &p, nil
}

func (r *sqlRepository) CreatePolicy(ctx context.Context, policy *model.CompliancePolicy) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO compliance_policies (id, tenant_id, name, policy_type, rules, is_active, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		policy.ID, policy.TenantID, policy.Name, policy.PolicyType, policy.Rules, policy.IsActive, policy.CreatedAt, policy.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert policy: %w", err)
	}
	return nil
}

func (r *sqlRepository) UpdatePolicy(ctx context.Context, policy *model.CompliancePolicy) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE compliance_policies SET name = $1, policy_type = $2, rules = $3, is_active = $4, updated_at = $5
		 WHERE id = $6 AND tenant_id = $7`,
		policy.Name, policy.PolicyType, policy.Rules, policy.IsActive, time.Now(), policy.ID, policy.TenantID,
	)
	if err != nil {
		return fmt.Errorf("update policy: %w", err)
	}
	return nil
}

func (r *sqlRepository) DeactivateOldPolicies(ctx context.Context, tenantID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE compliance_policies SET is_active = false WHERE tenant_id = $1`,
		tenantID,
	)
	if err != nil {
		return fmt.Errorf("deactivate old policies: %w", err)
	}
	return nil
}
