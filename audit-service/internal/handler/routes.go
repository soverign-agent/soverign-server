// Package handler provides HTTP handlers for audit-service.
package handler

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest"
)

// RegisterRoutes registers all the audit service routes.
func RegisterRoutes(
	server *rest.Server,
	auditHandler *AuditHandler,
	sseHandler *SSEHandler,
) {
	// Audit management routes
	server.AddRoute(rest.Route{
		Method:  http.MethodGet,
		Path:    "/api/v1/audit-jobs",
		Handler: auditHandler.List,
	})

	server.AddRoute(rest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/audit-jobs/trigger",
		Handler: auditHandler.Trigger,
	})

	server.AddRoute(rest.Route{
		Method:  http.MethodGet,
		Path:    "/api/v1/audit-jobs/:id",
		Handler: auditHandler.Get,
	})

	server.AddRoute(rest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/audit-jobs/:id/pause",
		Handler: auditHandler.Pause,
	})

	server.AddRoute(rest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/audit-jobs/:id/resume",
		Handler: auditHandler.Resume,
	})

	server.AddRoute(rest.Route{
		Method:  http.MethodGet,
		Path:    "/api/v1/audit-jobs/:id/report",
		Handler: auditHandler.GetReport,
	})

	// SSE progress stream
	server.AddRoute(rest.Route{
		Method:  http.MethodGet,
		Path:    "/api/v1/audit-jobs/:id/status",
		Handler: sseHandler.StreamProgress,
	})
}
