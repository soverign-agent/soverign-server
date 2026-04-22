// Package config holds auth-service configuration.
package config

import "time"

// Config defines auth-service configuration.
type Config struct {
	Auth     AuthConfig
	Database DatabaseConfig
	GRPC     GRPCConfig
}

// GRPCConfig holds gRPC server configuration.
type GRPCConfig struct {
	Port        int    `json:"port"`
	TLSCertFile string `json:"tls_cert_file"`
	TLSKeyFile  string `json:"tls_key_file"`
	Insecure    bool   `json:"insecure"`
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
