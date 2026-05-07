// Package config provides centralized configuration loading from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all application configuration with sensible defaults.
type Config struct {
	App      AppConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Temporal TemporalConfig
	JWT      JWTConfig
	LLM      LLMConfig
	Server   ServerConfig
}

// AppConfig holds application-level settings.
type AppConfig struct {
	Name    string
	Env     string
	Version string
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

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

// TemporalConfig holds Temporal workflow engine settings.
type TemporalConfig struct {
	HostPort  string
	Namespace string
}

// JWTConfig holds JWT signing and validation settings.
type JWTConfig struct {
	Secret        string
	AccessExpiry  time.Duration
	RefreshExpiry time.Duration
}

// LLMConfig holds LLM provider settings.
type LLMConfig struct {
	Provider    string
	APIKey      string
	BaseURL     string
	Model       string
	Timeout     time.Duration
	MaxTokens   int
	Temperature float64
}

// ServerConfig holds HTTP/gRPC server settings.
type ServerConfig struct {
	HTTPPort int
	GRPCPort int
}

// Load reads configuration from environment variables and returns a populated Config.
// Sensible defaults are used when environment variables are not set.
func Load() (*Config, error) {
	cfg := &Config{
		App: AppConfig{
			Name:    getEnv("APP_NAME", "sovereign-ai-compliance"),
			Env:     getEnv("APP_ENV", "development"),
			Version: getEnv("APP_VERSION", "0.1.0"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvInt("DB_PORT", 5432),
			User:     getEnv("DB_USER", "sovereign"),
			Password: getEnv("DB_PASSWORD", "sovereign-dev-password"),
			Database: getEnv("DB_NAME", "sovereign"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnvInt("REDIS_PORT", 6379),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		Temporal: TemporalConfig{
			HostPort:  getEnv("TEMPORAL_HOST_PORT", "localhost:7233"),
			Namespace: getEnv("TEMPORAL_NAMESPACE", "default"),
		},
		JWT: JWTConfig{
			Secret:        getEnv("JWT_SECRET", ""),
			AccessExpiry:  getEnvDuration("JWT_ACCESS_EXPIRY", 15*time.Minute),
			RefreshExpiry: getEnvDuration("JWT_REFRESH_EXPIRY", 7*24*time.Hour),
		},
		LLM: LLMConfig{
			Provider:    getEnv("LLM_PROVIDER", "openai"),
			APIKey:      getEnv("LLM_API_KEY", ""),
			BaseURL:     getEnv("LLM_BASE_URL", ""),
			Model:       getEnv("LLM_MODEL", "gpt-4o"),
			Timeout:     getEnvDuration("LLM_TIMEOUT", 60*time.Second),
			MaxTokens:   getEnvInt("LLM_MAX_TOKENS", 4096),
			Temperature: getEnvFloat("LLM_TEMPERATURE", 0.7),
		},
		Server: ServerConfig{
			HTTPPort: getEnvInt("HTTP_PORT", 8080),
			GRPCPort: getEnvInt("GRPC_PORT", 9091),
		},
	}

	if cfg.JWT.Secret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	return cfg, nil
}

// DSN returns the PostgreSQL connection string.
func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode,
	)
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return defaultValue
	}
	return i
}

func getEnvFloat(key string, defaultValue float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return defaultValue
	}
	return f
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue
	}
	if strings.HasSuffix(v, "s") || strings.HasSuffix(v, "m") || strings.HasSuffix(v, "h") {
		d, err := time.ParseDuration(v)
		if err != nil {
			return defaultValue
		}
		return d
	}
	// If no unit suffix, assume seconds
	i, err := strconv.Atoi(v)
	if err != nil {
		return defaultValue
	}
	return time.Duration(i) * time.Second
}
