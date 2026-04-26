// Package config defines the configuration structure for audit-service.
package config

import sharedconfig "sovereign-ai-compliance/shared/config"

// Config holds the application configuration for audit-service.
type Config struct {
	Database     sharedconfig.DatabaseConfig
	Temporal     sharedconfig.TemporalConfig
	Risk         RiskConfig
	GRPC         GRPCConfig
	Notification NotificationConfig
	Repo         RepoConfig
}

// NotificationConfig holds notification-service gRPC client settings.
type NotificationConfig struct {
	Addr        string `json:"addr"`
	TLSCertFile string `json:"tls_cert_file,optional"`
	Insecure    bool   `json:"insecure"`
}

// RepoConfig holds repo-service gRPC client settings.
type RepoConfig struct {
	Addr        string `json:"addr"`
	TLSCertFile string `json:"tls_cert_file,optional"`
	Insecure    bool   `json:"insecure"`
}

// RiskConfig holds risk scoring thresholds configuration.
type RiskConfig struct {
	CriticalThreshold int `json:"critical_threshold"` // Minimum score for critical severity
	HighThreshold     int `json:"high_threshold"`     // Minimum score for high severity
	MediumThreshold   int `json:"medium_threshold"`   // Minimum score for medium severity
	// Low is everything below Medium
}

// GRPCConfig holds gRPC server configuration.
type GRPCConfig struct {
	Port        int    `json:"port"`                   // gRPC server port, e.g. 9086
	TLSCertFile string `json:"tls_cert_file,optional"` // Path to TLS certificate (production)
	TLSKeyFile  string `json:"tls_key_file,optional"`  // Path to TLS key (production)
	Insecure    bool   `json:"insecure"`               // Allow insecure connections (local dev only)
}

// DefaultRiskConfig returns the default risk configuration.
func DefaultRiskConfig() RiskConfig {
	return RiskConfig{
		CriticalThreshold: 70,
		HighThreshold:     40,
		MediumThreshold:   20,
	}
}
