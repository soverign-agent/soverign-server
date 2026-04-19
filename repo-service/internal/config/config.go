// Package config holds repo-service configuration.
package config

import (
	"github.com/zeromicro/go-zero/rest"
)

// Config defines repo-service configuration.
type Config struct {
	rest.RestConf
	Database DatabaseConfig
	// EncryptionKey is the 32-byte AES-256 key used for encrypting credentials at rest
	EncryptionKey string
	// TempDir is the directory where we clone repositories for analysis
	TempDir string `json:",default=/tmp/repo-scans"`
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string
}
