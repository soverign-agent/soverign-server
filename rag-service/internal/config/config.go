// Package config defines the configuration structure for rag-service.
package config

import (
	"github.com/zeromicro/go-zero/rest"
	"sovereign-ai-compliance/shared/llm"
)

// Config holds the application configuration for rag-service.
type Config struct {
	rest.RestConf
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
}
