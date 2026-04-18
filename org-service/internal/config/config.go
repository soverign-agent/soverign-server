// Package config holds org-service configuration.
package config

import "github.com/zeromicro/go-zero/rest"

// Config defines org-service configuration.
type Config struct {
	rest.RestConf
	Database DatabaseConfig
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
