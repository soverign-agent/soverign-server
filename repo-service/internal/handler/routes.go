// Package handler provides REST handlers for the repository service.
package handler

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest"
)

// RegisterRoutes registers all repository service routes with the server.
func RegisterRoutes(server *rest.Server, handler *RepositoryHandler) {
	server.AddRoutes(
		[]rest.Route{
			// Repository routes
			{Method: http.MethodGet, Path: "/api/v1/repositories", Handler: handler.ListRepositories},
			{Method: http.MethodPost, Path: "/api/v1/repositories", Handler: handler.CreateRepository},
			{Method: http.MethodDelete, Path: "/api/v1/repositories/:id", Handler: handler.DeleteRepository},
			{Method: http.MethodPost, Path: "/api/v1/repositories/:id/test", Handler: handler.TestConnection},
			{Method: http.MethodPost, Path: "/api/v1/repositories/:id/scan", Handler: handler.TriggerScan},
			{Method: http.MethodPost, Path: "/api/v1/repository-webhooks/:id", Handler: handler.WebhookCallback},

			// Scan result routes
			{Method: http.MethodGet, Path: "/api/v1/repositories/:id/scans", Handler: handler.ListScanResults},
			{Method: http.MethodGet, Path: "/api/v1/repositories/scans/:scanId", Handler: handler.GetScanResult},
		},
	)
}
