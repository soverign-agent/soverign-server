// Package handler provides REST handlers for the org service.
package handler

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/rest/httpx"

	"sovereign-ai-compliance/org-service/internal/logic"
	"sovereign-ai-compliance/org-service/internal/types"
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
	var req struct {
		TenantID string `form:"tenant_id"`
	}
	if err := httpx.Parse(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	tenant, err := h.org.GetTenant(r.Context(), parseUUID(req.TenantID))
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
	tenant, err := h.org.UpdateTenant(r.Context(), req.TenantID, req.Name, req.Domain, req.Settings)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.OkJson(w, types.TenantResponse{Tenant: *tenant})
}

// ListUsers returns all users in a tenant.
func (h *OrgHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TenantID string `form:"tenant_id"`
	}
	if err := httpx.Parse(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	users, err := h.org.ListUsers(r.Context(), parseUUID(req.TenantID))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.OkJson(w, types.UsersResponse{Users: users})
}

// InviteUser invites a new user to a tenant.
func (h *OrgHandler) InviteUser(w http.ResponseWriter, r *http.Request) {
	var req types.InviteUserRequest
	if err := httpx.Parse(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	user, err := h.org.InviteUser(r.Context(), req.TenantID, req.Email, req.Role)
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
	user, err := h.org.UpdateUserRole(r.Context(), req.TenantID, req.UserID, req.Role)
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
	user, err := h.org.ToggleUserActive(r.Context(), req.TenantID, req.UserID, req.IsActive)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.OkJson(w, user)
}

// ListAISystems returns all AI systems for a tenant.
func (h *OrgHandler) ListAISystems(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TenantID string `form:"tenant_id"`
	}
	if err := httpx.Parse(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	systems, err := h.org.ListAISystems(r.Context(), parseUUID(req.TenantID))
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
	system, err := h.org.CreateAISystem(r.Context(), req.TenantID, req.Name, req.Description, req.RiskClassification, req.Status, req.Metadata)
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
	system, err := h.org.UpdateAISystem(r.Context(), req.TenantID, req.SystemID, req.Name, req.Description, req.RiskClassification, req.Status, req.Metadata)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.OkJson(w, system)
}

// DeleteAISystem soft deletes an AI system.
func (h *OrgHandler) DeleteAISystem(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TenantID string `form:"tenant_id"`
		SystemID string `form:"system_id"`
	}
	if err := httpx.Parse(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	if err := h.org.DeleteAISystem(r.Context(), parseUUID(req.TenantID), parseUUID(req.SystemID)); err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.OkJson(w, map[string]string{"message": "deleted"})
}

// GetActivePolicy returns the current active compliance policy.
func (h *OrgHandler) GetActivePolicy(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TenantID string `form:"tenant_id"`
	}
	if err := httpx.Parse(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	policy, err := h.org.GetActivePolicy(r.Context(), parseUUID(req.TenantID))
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
	policy, err := h.org.UpdatePolicy(r.Context(), req.TenantID, req.Name, req.PolicyType, req.Rules)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.OkJson(w, types.PolicyResponse{CompliancePolicy: *policy})
}

func parseUUID(s string) uuid.UUID {
	id, _ := uuid.Parse(s)
	return id
}
