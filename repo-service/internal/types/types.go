// Package types defines request and response types for the repository service.
package types

import (
	"github.com/google/uuid"

	"sovereign-ai-compliance/repo-service/model"
)

// Provider is the repository provider type.
type Provider string

const (
	ProviderGitHub     Provider = "github"
	ProviderGitLab     Provider = "gitlab"
	ProviderSelfHosted  Provider = "self_hosted"
)

// CreateRepositoryRequest is the request to create a new repository connection.
type CreateRepositoryRequest struct {
	Name          string  `json:"name" validate:"required"`
	URL           string  `json:"url" validate:"required,url"`
	Provider      Provider `json:"provider" validate:"required,oneof=github gitlab self_hosted"`
	Username      *string `json:"username"` // Username for Git authentication
	Token         *string `json:"token"`    // Personal access token
	WebhookSecret *string `json:"webhook_secret"` // Webhook secret
	DefaultBranch string  `json:"default_branch" default:"main"`
}

// CreateRepositoryResponse is the response after creating a repository connection.
type CreateRepositoryResponse struct {
	Repository model.SafeRepository `json:"repository"`
}

// ListRepositoriesResponse is the response listing all repositories for a tenant.
type ListRepositoriesResponse struct {
	Repositories []model.SafeRepository `json:"repositories"`
}

// DeleteRepositoryRequest is the request to delete a repository connection.
type DeleteRepositoryRequest struct {
	RepositoryID uuid.UUID `json:"repository_id" validate:"required"`
}

// DeleteRepositoryResponse is the response after deleting a repository.
type DeleteRepositoryResponse struct {
	Success bool `json:"success"`
}

// TestConnectionRequest is the request to test a repository connection.
type TestConnectionRequest struct {
	RepositoryID uuid.UUID `json:"repository_id" validate:"required"`
}

// TestConnectionResponse is the response after testing the connection.
type TestConnectionResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// TriggerScanRequest is the request to manually trigger a code scan.
type TriggerScanRequest struct {
	RepositoryID uuid.UUID `json:"repository_id" validate:"required"`
	Branch       *string   `json:"branch"` // If omitted, uses default branch
}

// TriggerScanResponse is the response after triggering a scan.
type TriggerScanResponse struct {
	ScanID    uuid.UUID `json:"scan_id"`
	Status    string    `json:"status"`
}

// WebhookCallbackRequest is the incoming webhook callback request.
type WebhookCallbackRequest struct {
	RepositoryID uuid.UUID `json:"repository_id" validate:"required"`
}

// ScanResultResponse is the response containing scan results.
type ScanResultResponse struct {
	ScanResult model.ScanResult `json:"scan_result"`
}

// ListScanResultsResponse lists scan results for a repository.
type ListScanResultsResponse struct {
	ScanResults []model.ScanResult `json:"scan_results"`
}
