// Package config defines the configuration structure for monitoring-service.
package config

// Config holds the application configuration for monitoring-service.
type Config struct {
	GRPC       GRPCConfig
	Prometheus PrometheusConfig
}

// GRPCConfig holds gRPC server configuration.
type GRPCConfig struct {
	Port        int    `json:"port"`                   // gRPC server port, e.g. 9088
	TLSCertFile string `json:"tls_cert_file,optional"` // Path to TLS certificate (production)
	TLSKeyFile  string `json:"tls_key_file,optional"`  // Path to TLS key (production)
	Insecure    bool   `json:"insecure"`               // Allow insecure connections (local dev only)
}

// PrometheusConfig holds the Prometheus HTTP API endpoint used by the
// monitoring service to read AI metrics.
type PrometheusConfig struct {
	URL string `json:"url"` // e.g. "http://localhost:9090"
}
