// Package handler provides REST handlers for the auth service.
package handler

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"

	"sovereign-ai-compliance/auth-service/internal/logic"
	"sovereign-ai-compliance/auth-service/internal/types"
)

// AuthHandler holds auth business logic.
type AuthHandler struct {
	auth *logic.Auth
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(auth *logic.Auth) *AuthHandler {
	return &AuthHandler{auth: auth}
}

// Login handles user login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req types.LoginRequest
	if err := httpx.Parse(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}

	pair, user, err := h.auth.Login(r.Context(), req.TenantID, req.Email, req.Password)
	if err != nil {
		httpx.Error(w, err)
		return
	}

	httpx.OkJson(w, types.LoginResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		User:         *user,
	})
}

// Refresh handles token refresh.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req types.RefreshRequest
	if err := httpx.Parse(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}

	pair, err := h.auth.RefreshToken(r.Context(), req.RefreshToken)
	if err != nil {
		httpx.Error(w, err)
		return
	}

	httpx.OkJson(w, types.TokenResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
	})
}

// Logout handles user logout.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req types.LogoutRequest
	if err := httpx.Parse(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}

	if err := h.auth.Logout(r.Context(), req.UserID); err != nil {
		httpx.Error(w, err)
		return
	}

	httpx.OkJson(w, map[string]string{"message": "logged out"})
}

// RequestPasswordReset handles password reset requests.
func (h *AuthHandler) RequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var req types.PasswordResetRequest
	if err := httpx.Parse(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}

	if err := h.auth.RequestPasswordReset(r.Context(), req.TenantID, req.Email); err != nil {
		httpx.Error(w, err)
		return
	}

	httpx.OkJson(w, map[string]string{"message": "if the email exists, a reset link has been sent"})
}

// ResetPassword handles password reset confirmation.
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req types.ResetPasswordConfirmRequest
	if err := httpx.Parse(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}

	if err := h.auth.ResetPassword(r.Context(), req.Token, req.NewPassword); err != nil {
		httpx.Error(w, err)
		return
	}

	httpx.OkJson(w, map[string]string{"message": "password reset successful"})
}

// Me returns the current user information.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	var req types.MeRequest
	if err := httpx.Parse(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}

	user, err := h.auth.Me(r.Context(), req.TenantID, req.UserID)
	if err != nil {
		httpx.Error(w, err)
		return
	}

	httpx.OkJson(w, user)
}
