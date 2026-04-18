// Package handler provides REST handlers for the org service.
package handler

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest"
)

// RegisterRoutes registers all org service routes with the server.
func RegisterRoutes(server *rest.Server, handler *OrgHandler) {
	server.AddRoutes(
		[]rest.Route{
			// Tenant routes
			{Method: http.MethodGet, Path: "/api/org/tenant", Handler: handler.GetTenant},
			{Method: http.MethodPut, Path: "/api/org/tenant", Handler: handler.UpdateTenant},

			// User routes
			{Method: http.MethodGet, Path: "/api/org/users", Handler: handler.ListUsers},
			{Method: http.MethodPost, Path: "/api/org/users/invite", Handler: handler.InviteUser},
			{Method: http.MethodPut, Path: "/api/org/users/role", Handler: handler.UpdateUserRole},
			{Method: http.MethodPut, Path: "/api/org/users/toggle", Handler: handler.ToggleUser},

			// AI System routes
			{Method: http.MethodGet, Path: "/api/org/ai-systems", Handler: handler.ListAISystems},
			{Method: http.MethodPost, Path: "/api/org/ai-systems", Handler: handler.CreateAISystem},
			{Method: http.MethodPut, Path: "/api/org/ai-systems", Handler: handler.UpdateAISystem},
			{Method: http.MethodDelete, Path: "/api/org/ai-systems", Handler: handler.DeleteAISystem},

			// Compliance Policy routes
			{Method: http.MethodGet, Path: "/api/org/policy", Handler: handler.GetActivePolicy},
			{Method: http.MethodPut, Path: "/api/org/policy", Handler: handler.UpdatePolicy},
		},
	)
}
