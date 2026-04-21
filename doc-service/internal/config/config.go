// Package config defines the configuration structure for doc-service.
package config

import (
	sharedconfig "sovereign-ai-compliance/shared/config"

	"github.com/zeromicro/go-zero/rest"
)

// Config holds the application configuration for doc-service.
type Config struct {
	rest.RestConf
	Database sharedconfig.DatabaseConfig
	LLM      sharedconfig.LLMConfig
	Export   ExportConfig
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
