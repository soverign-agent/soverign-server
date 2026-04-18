// Package rbac provides role-based access control.
package rbac

import "fmt"

// Role represents a user role.
type Role string

const (
	RoleAdmin    Role = "admin"
	RoleAuditor  Role = "auditor"
	RoleReviewer Role = "reviewer"
)

// ResourceAction pairs a resource with an action.
type ResourceAction struct {
	Resource string
	Action   string
}

// permissionMap defines which roles can perform which actions on which resources.
var permissionMap = map[Role]map[ResourceAction]bool{
	RoleAdmin: {
		{"user", "create"}:    true,
		{"user", "read"}:      true,
		{"user", "update"}:    true,
		{"user", "delete"}:    true,
		{"tenant", "create"}:  true,
		{"tenant", "read"}:    true,
		{"tenant", "update"}:  true,
		{"tenant", "delete"}:  true,
		{"audit", "create"}:   true,
		{"audit", "read"}:     true,
		{"audit", "update"}:   true,
		{"audit", "delete"}:   true,
		{"document", "create"}: true,
		{"document", "read"}:   true,
		{"document", "update"}: true,
		{"document", "delete"}: true,
		{"approval", "create"}: true,
		{"approval", "read"}:   true,
		{"approval", "update"}: true,
		{"approval", "delete"}: true,
		{"system", "manage"}:   true,
	},
	RoleAuditor: {
		{"audit", "create"}:  true,
		{"audit", "read"}:    true,
		{"audit", "update"}:  true,
		{"document", "read"}: true,
		{"approval", "read"}: true,
		{"approval", "update"}: true,
	},
	RoleReviewer: {
		{"approval", "read"}:   true,
		{"approval", "update"}: true,
		{"document", "read"}:   true,
		{"audit", "read"}:      true,
	},
}

// HasPermission checks if a role has permission for a resource action.
func HasPermission(role string, resource, action string) bool {
	actions, ok := permissionMap[Role(role)]
	if !ok {
		return false
	}
	return actions[ResourceAction{Resource: resource, Action: action}]
}

// ValidRole checks if a role string is valid.
func ValidRole(role string) bool {
	switch Role(role) {
	case RoleAdmin, RoleAuditor, RoleReviewer:
		return true
	}
	return false
}

// RolePermissions returns all permissions for a role.
func RolePermissions(role string) []ResourceAction {
	actions, ok := permissionMap[Role(role)]
	if !ok {
		return nil
	}
	result := make([]ResourceAction, 0, len(actions))
	for ra := range actions {
		result = append(result, ra)
	}
	return result
}

// AllRoles returns all defined role names.
func AllRoles() []string {
	return []string{string(RoleAdmin), string(RoleAuditor), string(RoleReviewer)}
}

// ValidateRole returns an error if the role is not recognized.
func ValidateRole(role string) error {
	if !ValidRole(role) {
		return fmt.Errorf("invalid role: %s", role)
	}
	return nil
}
