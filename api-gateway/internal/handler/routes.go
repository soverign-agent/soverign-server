// Package handler registers API gateway routes.
package handler

import (
	"net/http"

	"sovereign-ai-compliance/api-gateway/internal/svc"

	"github.com/zeromicro/go-zero/rest"
)

// RegisterHandlers configures all API gateway routes with middleware.
func RegisterHandlers(server *rest.Server, serverCtx *svc.ServiceContext) {
	// Apply global middleware: JWT auth -> tenant extraction
	server.Use(serverCtx.JWTAuth)
	server.Use(serverCtx.Tenant)

	// Auth service routes (public endpoints handled by JWT middleware skip list)
	server.AddRoutes(
		[]rest.Route{
			{Method: http.MethodPost, Path: "/api/v1/auth/*", Handler: serverCtx.Proxy("auth")},
			{Method: http.MethodGet, Path: "/api/v1/auth/*", Handler: serverCtx.Proxy("auth")},
			{Method: http.MethodPut, Path: "/api/v1/auth/*", Handler: serverCtx.Proxy("auth")},
			{Method: http.MethodDelete, Path: "/api/v1/auth/*", Handler: serverCtx.Proxy("auth")},
		},
	)

	// Org service routes
	server.AddRoutes(
		[]rest.Route{
			{Method: http.MethodGet, Path: "/api/v1/ai-systems/*", Handler: serverCtx.Proxy("org")},
			{Method: http.MethodPost, Path: "/api/v1/ai-systems/*", Handler: serverCtx.Proxy("org")},
			{Method: http.MethodPut, Path: "/api/v1/ai-systems/*", Handler: serverCtx.Proxy("org")},
			{Method: http.MethodDelete, Path: "/api/v1/ai-systems/*", Handler: serverCtx.Proxy("org")},
			{Method: http.MethodGet, Path: "/api/v1/org/*", Handler: serverCtx.Proxy("org")},
			{Method: http.MethodPost, Path: "/api/v1/org/*", Handler: serverCtx.Proxy("org")},
			{Method: http.MethodPut, Path: "/api/v1/org/*", Handler: serverCtx.Proxy("org")},
			{Method: http.MethodDelete, Path: "/api/v1/org/*", Handler: serverCtx.Proxy("org")},
		},
	)

	// Repo service routes
	server.AddRoutes(
		[]rest.Route{
			{Method: http.MethodGet, Path: "/api/v1/repositories/*", Handler: serverCtx.Proxy("repo")},
			{Method: http.MethodPost, Path: "/api/v1/repositories/*", Handler: serverCtx.Proxy("repo")},
			{Method: http.MethodPut, Path: "/api/v1/repositories/*", Handler: serverCtx.Proxy("repo")},
			{Method: http.MethodDelete, Path: "/api/v1/repositories/*", Handler: serverCtx.Proxy("repo")},
			{Method: http.MethodPost, Path: "/api/v1/repository-webhooks/*", Handler: serverCtx.Proxy("repo")},
		},
	)

	// RAG service routes
	server.AddRoutes(
		[]rest.Route{
			{Method: http.MethodGet, Path: "/api/v1/documents/*", Handler: serverCtx.Proxy("rag")},
			{Method: http.MethodPost, Path: "/api/v1/documents/*", Handler: serverCtx.Proxy("rag")},
			{Method: http.MethodPut, Path: "/api/v1/documents/*", Handler: serverCtx.Proxy("rag")},
			{Method: http.MethodDelete, Path: "/api/v1/documents/*", Handler: serverCtx.Proxy("rag")},
			{Method: http.MethodGet, Path: "/api/v1/rag/*", Handler: serverCtx.Proxy("rag")},
			{Method: http.MethodPost, Path: "/api/v1/rag/*", Handler: serverCtx.Proxy("rag")},
		},
	)

	// Doc service routes
	server.AddRoutes(
		[]rest.Route{
			{Method: http.MethodGet, Path: "/api/v1/generated-documents/*", Handler: serverCtx.Proxy("doc")},
			{Method: http.MethodPost, Path: "/api/v1/generated-documents/*", Handler: serverCtx.Proxy("doc")},
			{Method: http.MethodPut, Path: "/api/v1/generated-documents/*", Handler: serverCtx.Proxy("doc")},
			{Method: http.MethodDelete, Path: "/api/v1/generated-documents/*", Handler: serverCtx.Proxy("doc")},
		},
	)

	// Audit service routes
	server.AddRoutes(
		[]rest.Route{
			{Method: http.MethodGet, Path: "/api/v1/audit-jobs/*", Handler: serverCtx.Proxy("audit")},
			{Method: http.MethodPost, Path: "/api/v1/audit-jobs/*", Handler: serverCtx.Proxy("audit")},
			{Method: http.MethodPut, Path: "/api/v1/audit-jobs/*", Handler: serverCtx.Proxy("audit")},
			{Method: http.MethodDelete, Path: "/api/v1/audit-jobs/*", Handler: serverCtx.Proxy("audit")},
		},
	)

	// Agent orchestrator routes
	server.AddRoutes(
		[]rest.Route{
			{Method: http.MethodGet, Path: "/api/v1/approvals/*", Handler: serverCtx.Proxy("agent")},
			{Method: http.MethodPost, Path: "/api/v1/approvals/*", Handler: serverCtx.Proxy("agent")},
			{Method: http.MethodPut, Path: "/api/v1/approvals/*", Handler: serverCtx.Proxy("agent")},
			{Method: http.MethodGet, Path: "/api/v1/agent/*", Handler: serverCtx.Proxy("agent")},
			{Method: http.MethodPost, Path: "/api/v1/agent/*", Handler: serverCtx.Proxy("agent")},
			{Method: http.MethodPut, Path: "/api/v1/agent/*", Handler: serverCtx.Proxy("agent")},
		},
	)

	// Notification service routes
	server.AddRoutes(
		[]rest.Route{
			{Method: http.MethodGet, Path: "/api/v1/notifications/*", Handler: serverCtx.Proxy("notification")},
			{Method: http.MethodPost, Path: "/api/v1/notifications/*", Handler: serverCtx.Proxy("notification")},
			{Method: http.MethodPut, Path: "/api/v1/notifications/*", Handler: serverCtx.Proxy("notification")},
			{Method: http.MethodDelete, Path: "/api/v1/notifications/*", Handler: serverCtx.Proxy("notification")},
		},
	)
}
