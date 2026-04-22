// Package config defines the configuration structure for rag-service.
package config

import "sovereign-ai-compliance/shared/llm"

// Config holds the application configuration for rag-service.
type Config struct {
	Database struct {
		Host     string
		Port     int
		User     string
		Password string
		Database string
		SSLMode  string
	}
	LLM struct {
		Provider       llm.Provider
		APIKey         string
		BaseURL        string
		EmbeddingModel string
		Timeout        int
		MaxTokens      int
		Temperature    float64
	}
	Chunking struct {
		DefaultChunkSize    int
		DefaultChunkOverlap int
	}
	GRPC GRPCConfig
}

// GRPCConfig holds gRPC server configuration.
type GRPCConfig struct {
	Port        int    `json:"port"`
	TLSCertFile string `json:"tls_cert_file"`
	TLSKeyFile  string `json:"tls_key_file"`
	Insecure    bool   `json:"insecure"`
}
