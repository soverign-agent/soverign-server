// Package config defines the configuration structure for audit-service.
package config

import (
	"github.com/zeromicro/go-zero/rest"
	sharedconfig "sovereign-ai-compliance/shared/config"
)

// Config holds the application configuration for audit-service.
type Config struct {
	rest.RestConf
	Database sharedconfig.DatabaseConfig
	Temporal sharedconfig.TemporalConfig
	Risk     RiskConfig
}

// RiskConfig holds risk scoring thresholds configuration.
type RiskConfig struct {
	CriticalThreshold int `json:"critical_threshold"` // Minimum score for critical severity
	HighThreshold     int `json:"high_threshold"`     // Minimum score for high severity
	MediumThreshold   int `json:"medium_threshold"`   // Minimum score for medium severity
	// Low is everything below Medium
}

// DefaultRiskConfig returns the default risk configuration.
func DefaultRiskConfig() RiskConfig {
	return RiskConfig{
		CriticalThreshold: 70,
		HighThreshold:     40,
		MediumThreshold:   20,
	}
}
