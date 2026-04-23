// Package config holds org-service configuration.
package config

// Config defines org-service configuration.
type Config struct {
	Database DatabaseConfig
	GRPC     GRPCConfig
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

// GRPCConfig holds gRPC server configuration.
type GRPCConfig struct {
	Port        int    `json:"port"`
	TLSCertFile string `json:"tls_cert_file,optional"`
	TLSKeyFile  string `json:"tls_key_file,optional"`
	Insecure    bool   `json:"insecure"`
}
