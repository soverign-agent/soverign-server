package main

import (
	"testing"

	"sovereign-ai-compliance/monitoring-service/internal/config"

	"github.com/stretchr/testify/assert"
)

func TestConfigStructure(t *testing.T) {
	// Verify config can be instantiated
	c := config.Config{
		GRPC: config.GRPCConfig{
			Port:     9088,
			Insecure: true,
		},
	}

	assert.Equal(t, 9088, c.GRPC.Port)
	assert.True(t, c.GRPC.Insecure)
	assert.Empty(t, c.GRPC.TLSCertFile)
	assert.Empty(t, c.GRPC.TLSKeyFile)
}
