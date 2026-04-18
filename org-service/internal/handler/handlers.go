// Package handler provides REST handlers for the org service.
package handler

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/rest/httpx"

	"sovereign-ai-compliance/org-service/internal/logic"
	"sovereign-ai-compliance/org-service/internal/types"
	"sovereign-ai-compliance/shared/tenant"
)

// OrgHandler holds org business logic.
type OrgHandler struct {
	org *logic.Org
}

// NewOrgHandler creates a new OrgHandler.
func NewOrgHandler(org *logic.Org) *OrgHandler {
	return &OrgHandler{org: org}
}

// GetTenant returns current tenant information.
func (h *OrgHandler) GetTenant(w http.ResponseWriter, r *http.Request) {
	// Get authenticated tenant from context (set by API Gateway after JWT validation)
	tenantIDStr := tenant.MustFromContext(r.Context())
	tenantID, err := parseUUID(tenantIDStr)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	tenant, err := h.org.GetTenant(r.Context(), tenantID)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.OkJson(w, types.TenantResponse{Tenant: *tenant})
}

// UpdateTenant updates tenant information.
func (h *OrgHandler) UpdateTenant(w http.ResponseWriter, r *http.Request) {
	var req types.UpdateTenantRequest
	if err := httpx.Parse(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	// Get authenticated tenant from context (set by API Gateway after JWT validation)
	// Ignore tenant_id from request for security - can only modify your own tenant
	tenantIDStr := tenant.MustFromContext(r.Context())
	authenticatedTenantID, err := parseUUID(tenantIDStr)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	tenant, err := h.org.UpdateTenant(r.Context(), authenticatedTenantID, req.Name, req.Domain, req.Settings)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.OkJson(w, types.TenantResponse{Tenant: *tenant})
}

// ListUsers returns all users in the current tenant.
func (h *OrgHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	// Get authenticated tenant from context (set by API Gateway after JWT validation)
	tenantIDStr := tenant.MustFromContext(r.Context())
	tenantID, err := parseUUID(tenantIDStr)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	users, err := h.org.ListUsers(r.Context(), tenantID)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.OkJson(w, types.UsersResponse{Users: users})
}

// InviteUser invites a new user to the current tenant.
func (h *OrgHandler) InviteUser(w http.ResponseWriter, r *http.Request) {
	var req types.InviteUserRequest
	if err := httpx.Parse(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	// Get authenticated tenant from context (set by API Gateway after JWT validation)
	// Ignore tenant_id from request for security - can only invite to your own tenant
	tenantIDStr := tenant.MustFromContext(r.Context())
	authenticatedTenantID, err := parseUUID(tenantIDStr)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	user, err := h.org.InviteUser(r.Context(), authenticatedTenantID, req.Email, req.Role)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.OkJson(w, user)
}

// UpdateUserRole updates a user's role.
func (h *OrgHandler) UpdateUserRole(w http.ResponseWriter, r *http.Request) {
	var req types.UpdateUserRoleRequest
	if err := httpx.Parse(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	// Get authenticated tenant from context (set by API Gateway after JWT validation)
	// Ignore tenant_id from request for security - can only modify users in your own tenant
	tenantIDStr := tenant.MustFromContext(r.Context())
	authenticatedTenantID, err := parseUUID(tenantIDStr)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	user, err := h.org.UpdateUserRole(r.Context(), authenticatedTenantID, req.UserID, req.Role)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.OkJson(w, user)
}

// ToggleUser enables or disables a user.
func (h *OrgHandler) ToggleUser(w http.ResponseWriter, r *http.Request) {
	var req types.ToggleUserRequest
	if err := httpx.Parse(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	// Get authenticated tenant from context (set by API Gateway after JWT validation)
	// Ignore tenant_id from request for security - can only toggle users in your own tenant
	tenantIDStr := tenant.MustFromContext(r.Context())
	authenticatedTenantID, err := parseUUID(tenantIDStr)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	user, err := h.org.ToggleUserActive(r.Context(), authenticatedTenantID, req.UserID, req.IsActive)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.OkJson(w, user)
}

// ListAISystems returns all AI systems for the current tenant.
func (h *OrgHandler) ListAISystems(w http.ResponseWriter, r *http.Request) {
	// Get authenticated tenant from context (set by API Gateway after JWT validation)
	tenantIDStr := tenant.MustFromContext(r.Context())
	tenantID, err := parseUUID(tenantIDStr)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	systems, err := h.org.ListAISystems(r.Context(), tenantID)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.OkJson(w, types.AISystemsResponse{Systems: systems})
}

// CreateAISystem creates a new AI system.
func (h *OrgHandler) CreateAISystem(w http.ResponseWriter, r *http.Request) {
	var req types.CreateAISystemRequest
	if err := httpx.Parse(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	// Get authenticated tenant from context (set by API Gateway after JWT validation)
	// Ignore tenant_id from request for security - can only create in your own tenant
	tenantIDStr := tenant.MustFromContext(r.Context())
	authenticatedTenantID, err := parseUUID(tenantIDStr)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	system, err := h.org.CreateAISystem(r.Context(), authenticatedTenantID, req.Name, req.Description, req.RiskClassification, req.Status, req.Metadata)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.OkJson(w, system)
}

// UpdateAISystem updates an AI system.
func (h *OrgHandler) UpdateAISystem(w http.ResponseWriter, r *http.Request) {
	var req types.UpdateAISystemRequest
	if err := httpx.Parse(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	// Get authenticated tenant from context (set by API Gateway after JWT validation)
	// Ignore tenant_id from request for security - can only update in your own tenant
	tenantIDStr := tenant.MustFromContext(r.Context())
	authenticatedTenantID, err := parseUUID(tenantIDStr)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	system, err := h.org.UpdateAISystem(r.Context(), authenticatedTenantID, req.SystemID, req.Name, req.Description, req.RiskClassification, req.Status, req.Metadata)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.OkJson(w, system)
}

// DeleteAISystem soft deletes an AI system.
func (h *OrgHandler) DeleteAISystem(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SystemID string `form:"system_id"`
	}
	if err := httpx.Parse(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	// Get authenticated tenant from context (set by API Gateway after JWT validation)
	tenantIDStr := tenant.MustFromContext(r.Context())
	tenantID, err := parseUUID(tenantIDStr)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	systemID, err := parseUUID(req.SystemID)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	if err := h.org.DeleteAISystem(r.Context(), tenantID, systemID); err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.OkJson(w, map[string]string{"message": "deleted"})
}

// GetActivePolicy returns the current active compliance policy for the current tenant.
func (h *OrgHandler) GetActivePolicy(w http.ResponseWriter, r *http.Request) {
	// Get authenticated tenant from context (set by API Gateway after JWT validation)
	tenantIDStr := tenant.MustFromContext(r.Context())
	tenantID, err := parseUUID(tenantIDStr)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	policy, err := h.org.GetActivePolicy(r.Context(), tenantID)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.OkJson(w, types.PolicyResponse{CompliancePolicy: *policy})
}

// UpdatePolicy updates the compliance policy.
func (h *OrgHandler) UpdatePolicy(w http.ResponseWriter, r *http.Request) {
	var req types.PolicyRequest
	if err := httpx.Parse(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	// Get authenticated tenant from context (set by API Gateway after JWT validation)
	// Ignore tenant_id from request for security - can only update policy in your own tenant
	tenantIDStr := tenant.MustFromContext(r.Context())
	authenticatedTenantID, err := parseUUID(tenantIDStr)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	policy, err := h.org.UpdatePolicy(r.Context(), authenticatedTenantID, req.Name, req.PolicyType, req.Rules)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.OkJson(w, types.PolicyResponse{CompliancePolicy: *policy})
}

func parseUUID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid UUID: %w", err)
	}
	return id, nil
}
