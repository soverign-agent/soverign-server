// Package config holds repo-service configuration.
package config

// Config defines repo-service configuration.
type Config struct {
	Database DatabaseConfig
	// EncryptionKey is the 32-byte AES-256 key used for encrypting credentials at rest
	EncryptionKey string
	// TempDir is the directory where we clone repositories for analysis
	TempDir string `json:",default=/tmp/repo-scans"`
	GRPC    GRPCConfig
}

// GRPCConfig holds gRPC server configuration.
type GRPCConfig struct {
	Port        int    `json:"port"`
	TLSCertFile string `json:"tls_cert_file"`
	TLSKeyFile  string `json:"tls_key_file"`
	Insecure    bool   `json:"insecure"`
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
