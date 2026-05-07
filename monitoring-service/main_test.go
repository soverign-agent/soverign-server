package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"sovereign-ai-compliance/monitoring-service/internal/config"
)

func TestConfigStructure(t *testing.T) {
	c := config.Config{
		GRPC: config.GRPCConfig{
			Port:     9088,
			Insecure: true,
		},
		Prometheus: config.PrometheusConfig{
			URL: "http://localhost:9090",
		},
	}

	assert.Equal(t, 9088, c.GRPC.Port)
	assert.True(t, c.GRPC.Insecure)
	assert.Empty(t, c.GRPC.TLSCertFile)
	assert.Empty(t, c.GRPC.TLSKeyFile)
	assert.Equal(t, "http://localhost:9090", c.Prometheus.URL)
}
