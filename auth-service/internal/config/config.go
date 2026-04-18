// Package config holds auth-service configuration.
package config

import (
	"time"

	"github.com/zeromicro/go-zero/rest"
)

// Config defines auth-service configuration.
type Config struct {
	rest.RestConf
	Auth     AuthConfig
	Database DatabaseConfig
}

// AuthConfig holds JWT and security settings.
type AuthConfig struct {
	PrivateKeyPath string
	AccessExpiry   time.Duration
	RefreshExpiry  time.Duration
	Issuer         string
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
