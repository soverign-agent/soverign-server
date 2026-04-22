// Package config defines the configuration structure for agent-orchestrator.
package config

import (
	"sovereign-ai-compliance/agent-orchestrator/internal/client"
	sharedconfig "sovereign-ai-compliance/shared/config"
)

// Config holds the application configuration for agent-orchestrator.
type Config struct {
	Database sharedconfig.DatabaseConfig
	Temporal sharedconfig.TemporalConfig
	LLM      sharedconfig.LLMConfig
	GRPC     GRPCConfig
	Clients  client.Config
}

// GRPCConfig holds gRPC server configuration.
type GRPCConfig struct {
	Port        int    `json:"port"`
	TLSCertFile string `json:"tls_cert_file"`
	TLSKeyFile  string `json:"tls_key_file"`
	Insecure    bool   `json:"insecure"`
}
