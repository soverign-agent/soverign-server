// Package model defines database models for the repository service.
package model

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// Provider represents the Git repository provider.
type Provider string

const (
	ProviderGitHub   Provider = "github"
	ProviderGitLab   Provider = "gitlab"
	ProviderSelfHosted Provider = "self_hosted"
)

// Repository represents a code repository connection configuration.
type Repository struct {
	ID                uuid.UUID      `json:"id" db:"id"`
	TenantID          uuid.UUID      `json:"tenant_id" db:"tenant_id"`
	Name              string         `json:"name" db:"name"`
	URL               string         `json:"url" db:"url"`
	Provider          Provider       `json:"provider" db:"provider"`
	EncryptedCredentials []byte      `json:"-" db:"encrypted_credentials"` // Encrypted Git credentials (AES-256-GCM)
	WebhookSecret     []byte         `json:"-" db:"webhook_secret"`       // Encrypted webhook secret
	WebhookID         sql.NullString `json:"webhook_id" db:"webhook_id"`
	DefaultBranch     string         `json:"default_branch" db:"default_branch"`
	LastScannedAt     sql.NullTime   `json:"last_scanned_at" db:"last_scanned_at"`
	CreatedAt         time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at" db:"updated_at"`
}

// TableName returns the database table name.
func (Repository) TableName() string {
	return "repositories"
}

// SafeRepository returns a copy with sensitive fields removed for API responses.
type SafeRepository struct {
	ID            uuid.UUID      `json:"id"`
	TenantID      uuid.UUID      `json:"tenant_id"`
	Name          string         `json:"name"`
	URL           string         `json:"url"`
	Provider      Provider       `json:"provider"`
	WebhookID     sql.NullString `json:"webhook_id"`
	DefaultBranch string         `json:"default_branch"`
	LastScannedAt sql.NullTime   `json:"last_scanned_at"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// ToSafe converts a Repository to SafeRepository with sensitive fields removed.
func (r *Repository) ToSafe() SafeRepository {
	return SafeRepository{
		ID:            r.ID,
		TenantID:      r.TenantID,
		Name:          r.Name,
		URL:           r.URL,
		Provider:      r.Provider,
		WebhookID:     r.WebhookID,
		DefaultBranch: r.DefaultBranch,
		LastScannedAt: r.LastScannedAt,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
	}
}

// ScanResult represents the result of a static code analysis scan.
type ScanResult struct {
	ID            uuid.UUID `json:"id" db:"id"`
	RepositoryID  uuid.UUID `json:"repository_id" db:"repository_id"`
	TenantID      uuid.UUID `json:"tenant_id" db:"tenant_id"`
	Branch        string    `json:"branch" db:"branch"`
	CommitHash    string    `json:"commit_hash" db:"commit_hash"`
	AIUses        []byte    `json:"ai_uses" db:"ai_uses"` // JSON array of detected AI usages
	DataFlows     []byte    `json:"data_flows" db:"data_flows"` // JSON array of detected data flows
	SensitiveData []byte    `json:"sensitive_data" db:"sensitive_data"` // JSON array of detected sensitive data
	TotalFiles    int       `json:"total_files" db:"total_files"`
	ScannedFiles  int       `json:"scanned_files" db:"scanned_files"`
	StartedAt     time.Time `json:"started_at" db:"started_at"`
	CompletedAt   sql.NullTime `json:"completed_at" db:"completed_at"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// TableName returns the database table name.
func (ScanResult) TableName() string {
	return "repository_scan_results"
}
