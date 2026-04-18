// Package handler provides REST handlers for the repository service.
package handler

import (
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/rest/httpx"

	"sovereign-ai-compliance/repo-service/internal/logic"
	"sovereign-ai-compliance/repo-service/internal/types"
	"sovereign-ai-compliance/shared/tenant"
)

// RepositoryHandler handles repository HTTP requests.
type RepositoryHandler struct {
	logic *logic.RepositoryLogic
}

// NewRepositoryHandler creates a new RepositoryHandler.
func NewRepositoryHandler(logic *logic.RepositoryLogic) *RepositoryHandler {
	return &RepositoryHandler{logic: logic}
}

// ListRepositories lists all repositories for the current tenant.
func (h *RepositoryHandler) ListRepositories(w http.ResponseWriter, r *http.Request) {
	tenantIDStr := tenant.MustFromContext(r.Context())
	tenantID, err := parseUUID(tenantIDStr)
	if err != nil {
		httpx.Error(w, err)
		return
	}

	repos, err := h.logic.List(r.Context(), tenantID)
	if err != nil {
		httpx.Error(w, err)
		return
	}

	httpx.OkJson(w, types.ListRepositoriesResponse{Repositories: repos})
}

// CreateRepository creates a new repository connection.
func (h *RepositoryHandler) CreateRepository(w http.ResponseWriter, r *http.Request) {
	var req types.CreateRepositoryRequest
	if err := httpx.Parse(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}

	tenantIDStr := tenant.MustFromContext(r.Context())
	authenticatedTenantID, err := parseUUID(tenantIDStr)
	if err != nil {
		httpx.Error(w, err)
		return
	}

	repo, err := h.logic.Create(r.Context(), authenticatedTenantID, &req)
	if err != nil {
		httpx.Error(w, err)
		return
	}

	httpx.OkJson(w, types.CreateRepositoryResponse{Repository: *repo})
}

// DeleteRepository deletes a repository connection.
func (h *RepositoryHandler) DeleteRepository(w http.ResponseWriter, r *http.Request) {
	var req types.DeleteRepositoryRequest
	if err := httpx.Parse(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}

	tenantIDStr := tenant.MustFromContext(r.Context())
	authenticatedTenantID, err := parseUUID(tenantIDStr)
	if err != nil {
		httpx.Error(w, err)
		return
	}

	success, err := h.logic.Delete(r.Context(), authenticatedTenantID, req.RepositoryID)
	if err != nil {
		httpx.Error(w, err)
		return
	}

	httpx.OkJson(w, types.DeleteRepositoryResponse{Success: success})
}

// TestConnection tests repository connection.
func (h *RepositoryHandler) TestConnection(w http.ResponseWriter, r *http.Request) {
	var req types.TestConnectionRequest
	if err := httpx.Parse(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}

	tenantIDStr := tenant.MustFromContext(r.Context())
	authenticatedTenantID, err := parseUUID(tenantIDStr)
	if err != nil {
		httpx.Error(w, err)
		return
	}

	success, message, err := h.logic.TestConnection(r.Context(), authenticatedTenantID, req.RepositoryID)
	if err != nil {
		httpx.Error(w, err)
		return
	}

	httpx.OkJson(w, types.TestConnectionResponse{
		Success: success,
		Message: message,
	})
}

// TriggerScan manually triggers a repository scan.
func (h *RepositoryHandler) TriggerScan(w http.ResponseWriter, r *http.Request) {
	var req types.TriggerScanRequest
	if err := httpx.Parse(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}

	tenantIDStr := tenant.MustFromContext(r.Context())
	_, err := parseUUID(tenantIDStr)
	if err != nil {
		httpx.Error(w, err)
		return
	}

	// TODO: actually run the scan async after implementing code pull
	// For now just return queued status
	httpx.OkJson(w, map[string]interface{}{
		"scan_id": uuid.New(),
		"status": "queued",
	})
}

// WebhookCallback handles incoming webhook from Git provider.
func (h *RepositoryHandler) WebhookCallback(w http.ResponseWriter, r *http.Request) {
	var pathReq struct {
		ID string `path:"id"`
	}
	if err := httpx.Parse(r, &pathReq); err != nil {
		httpx.Error(w, err)
		return
	}

	repoID, err := parseUUID(pathReq.ID)
	if err != nil {
		httpx.Error(w, err)
		return
	}

	// Read request body for signature verification
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		httpx.Error(w, err)
		return
	}

	// Get signature from header
	signature := r.Header.Get("X-Hub-Signature-256")
	if signature == "" {
		signature = r.Header.Get("X-Gitlab-Token")
	}

	// TODO: In the future we'll need to get tenant ID from the repository since webhook doesn't have it
	// For now, we expect the API gateway has already set tenant context
	// Wait - actually repository is already tenant-isolated via RLS, so we need to get tenant from context
	tenantIDStr := tenant.MustFromContext(r.Context())
	tenantID, err := parseUUID(tenantIDStr)
	if err != nil {
		httpx.Error(w, err)
		return
	}

	// Get repository from database
	repository, err := h.logic.GetByID(r.Context(), tenantID, repoID)
	if err != nil {
		httpx.Error(w, err)
		return
	}

	// Verify webhook signature
	valid, err := h.logic.VerifyWebhookSignature(r.Context(), repository, bodyBytes, signature, repository.Provider)
	if err != nil {
		httpx.Error(w, err)
		return
	}

	if !valid {
		http.Error(w, "invalid webhook signature", http.StatusUnauthorized)
		return
	}

	// TODO: When we implement code pulling and audit triggering, we need to trigger an audit job here
	// For now just accept the webhook
	httpx.OkJson(w, map[string]string{
		"status": "accepted",
		"message": "webhook received, audit will be triggered shortly",
	})
}

// GetScanResult gets a specific scan result.
func (h *RepositoryHandler) GetScanResult(w http.ResponseWriter, r *http.Request) {
	// For now - placeholder
	httpx.OkJson(w, map[string]interface{}{})
}

// ListScanResults lists all scan results for a repository.
func (h *RepositoryHandler) ListScanResults(w http.ResponseWriter, r *http.Request) {
	// For now - placeholder
	httpx.OkJson(w, map[string]interface{}{})
}

func parseUUID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid UUID: %w", err)
	}
	return id, nil
}
