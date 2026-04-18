package rbac

import (
	"testing"
)

func TestHasPermission_Admin(t *testing.T) {
	if !HasPermission("admin", "user", "create") {
		t.Error("expected admin to have user:create")
	}
	if !HasPermission("admin", "system", "manage") {
		t.Error("expected admin to have system:manage")
	}
	if !HasPermission("admin", "audit", "delete") {
		t.Error("expected admin to have audit:delete")
	}
}

func TestHasPermission_Auditor(t *testing.T) {
	if !HasPermission("auditor", "audit", "create") {
		t.Error("expected auditor to have audit:create")
	}
	if !HasPermission("auditor", "document", "read") {
		t.Error("expected auditor to have document:read")
	}
	if HasPermission("auditor", "user", "delete") {
		t.Error("expected auditor NOT to have user:delete")
	}
	if HasPermission("auditor", "system", "manage") {
		t.Error("expected auditor NOT to have system:manage")
	}
}

func TestHasPermission_Reviewer(t *testing.T) {
	if !HasPermission("reviewer", "approval", "read") {
		t.Error("expected reviewer to have approval:read")
	}
	if !HasPermission("reviewer", "approval", "update") {
		t.Error("expected reviewer to have approval:update")
	}
	if HasPermission("reviewer", "audit", "create") {
		t.Error("expected reviewer NOT to have audit:create")
	}
}

func TestHasPermission_InvalidRole(t *testing.T) {
	if HasPermission("unknown", "user", "read") {
		t.Error("expected unknown role to have no permissions")
	}
}

func TestValidRole(t *testing.T) {
	if !ValidRole("admin") {
		t.Error("expected admin to be valid")
	}
	if !ValidRole("auditor") {
		t.Error("expected auditor to be valid")
	}
	if !ValidRole("reviewer") {
		t.Error("expected reviewer to be valid")
	}
	if ValidRole("superuser") {
		t.Error("expected superuser to be invalid")
	}
}

func TestAllRoles(t *testing.T) {
	roles := AllRoles()
	if len(roles) != 3 {
		t.Errorf("expected 3 roles, got %d", len(roles))
	}
	roleSet := make(map[string]bool)
	for _, r := range roles {
		roleSet[r] = true
	}
	if !roleSet["admin"] || !roleSet["auditor"] || !roleSet["reviewer"] {
		t.Error("expected all three roles in list")
	}
}

func TestValidateRole(t *testing.T) {
	if err := ValidateRole("admin"); err != nil {
		t.Errorf("expected no error for admin: %v", err)
	}
	if err := ValidateRole("invalid"); err == nil {
		t.Error("expected error for invalid role")
	}
}

func TestRolePermissions(t *testing.T) {
	adminPerms := RolePermissions("admin")
	if len(adminPerms) == 0 {
		t.Error("expected admin to have permissions")
	}

	invalidPerms := RolePermissions("invalid")
	if invalidPerms != nil {
		t.Error("expected nil for invalid role")
	}
}
