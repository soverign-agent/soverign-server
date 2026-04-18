// Package handler registers auth service routes.
package handler

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest"
)

// RegisterRoutes configures all auth service HTTP routes.
func RegisterRoutes(server *rest.Server, handler *AuthHandler) {
	server.AddRoutes(
		[]rest.Route{
			{Method: http.MethodPost, Path: "/api/v1/auth/login", Handler: handler.Login},
			{Method: http.MethodPost, Path: "/api/v1/auth/refresh", Handler: handler.Refresh},
			{Method: http.MethodPost, Path: "/api/v1/auth/logout", Handler: handler.Logout},
			{Method: http.MethodPost, Path: "/api/v1/auth/password-reset/request", Handler: handler.RequestPasswordReset},
			{Method: http.MethodPost, Path: "/api/v1/auth/password-reset/confirm", Handler: handler.ResetPassword},
			{Method: http.MethodGet, Path: "/api/v1/auth/me", Handler: handler.Me},
		},
	)
}
