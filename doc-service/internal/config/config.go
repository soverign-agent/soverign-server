// Package config defines the configuration structure for doc-service.
package config

import (
	"sovereign-ai-compliance/doc-service/internal/client"
	sharedconfig "sovereign-ai-compliance/shared/config"
)

// Config holds the application configuration for doc-service.
type Config struct {
	Database sharedconfig.DatabaseConfig
	LLM      sharedconfig.LLMConfig
	Export   ExportConfig
	GRPC     GRPCConfig
	Clients  client.Config
}

// ExportConfig holds document export settings.
type ExportConfig struct {
	OutputDir     string `json:"output_dir"`       // Directory to store exported files
	DefaultFormat string `json:"default_format"`   // Default export format (pdf, docx)
	MaxFileSizeMB int    `json:"max_file_size_mb"` // Maximum export file size in MB
}

// DefaultExportConfig returns the default export configuration.
func DefaultExportConfig() ExportConfig {
	return ExportConfig{
		OutputDir:     "./exports",
		DefaultFormat: "pdf",
		MaxFileSizeMB: 50,
	}
}

// GRPCConfig holds gRPC server configuration.
type GRPCConfig struct {
	Port        int    `json:"port"`
	TLSCertFile string `json:"tls_cert_file"`
	TLSKeyFile  string `json:"tls_key_file"`
	Insecure    bool   `json:"insecure"`
}
