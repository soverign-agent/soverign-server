package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret")
	defer os.Unsetenv("JWT_SECRET")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if cfg.Database.Host != "localhost" {
		t.Errorf("expected DB host localhost, got %s", cfg.Database.Host)
	}
	if cfg.Database.Port != 5432 {
		t.Errorf("expected DB port 5432, got %d", cfg.Database.Port)
	}
	if cfg.Redis.Port != 6379 {
		t.Errorf("expected Redis port 6379, got %d", cfg.Redis.Port)
	}
	if cfg.Server.HTTPPort != 8080 {
		t.Errorf("expected HTTP port 8080, got %d", cfg.Server.HTTPPort)
	}
}

func TestLoad_MissingJWTSecret(t *testing.T) {
	os.Unsetenv("JWT_SECRET")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error when JWT_SECRET is missing")
	}
}

func TestLoad_EnvOverrides(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret")
	os.Setenv("DB_HOST", "postgres.example.com")
	os.Setenv("DB_PORT", "5555")
	os.Setenv("HTTP_PORT", "9090")
	defer func() {
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("DB_HOST")
		os.Unsetenv("DB_PORT")
		os.Unsetenv("HTTP_PORT")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if cfg.Database.Host != "postgres.example.com" {
		t.Errorf("expected DB host override, got %s", cfg.Database.Host)
	}
	if cfg.Database.Port != 5555 {
		t.Errorf("expected DB port 5555, got %d", cfg.Database.Port)
	}
	if cfg.Server.HTTPPort != 9090 {
		t.Errorf("expected HTTP port 9090, got %d", cfg.Server.HTTPPort)
	}
}

func TestDatabaseDSN(t *testing.T) {
	cfg := DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "sovereign",
		Password: "secret",
		Database: "sovereign",
		SSLMode:  "disable",
	}
	expected := "host=localhost port=5432 user=sovereign password=secret dbname=sovereign sslmode=disable"
	if got := cfg.DSN(); got != expected {
		t.Errorf("DSN mismatch:\n got: %s\nwant: %s", got, expected)
	}
}

func TestGetEnvDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
	}{
		{"30s", 30 * time.Second},
		{"5m", 5 * time.Minute},
		{"2h", 2 * time.Hour},
		{"60", 60 * time.Second},
	}

	for _, tt := range tests {
		os.Setenv("TEST_DURATION", tt.input)
		got := getEnvDuration("TEST_DURATION", 0)
		if got != tt.expected {
			t.Errorf("getEnvDuration(%q) = %v, want %v", tt.input, got, tt.expected)
		}
		os.Unsetenv("TEST_DURATION")
	}
}
