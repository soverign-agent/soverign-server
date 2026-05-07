// Package tests contains integration-style tests for the API gateway wiring.
package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"sovereign-ai-compliance/api-gateway/internal/config"
	"sovereign-ai-compliance/api-gateway/internal/gateway"
)

// TestShouldHandle_OrchestratorPaths verifies the orchestrator URL prefixes
// are routed to the grpc-gateway handler.
func TestShouldHandle_OrchestratorPaths(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/api/v1/orchestrator/invoke", true},
		{"/api/v1/orchestrator/agents/audit-agent", true},
		{"/api/v1/orchestrator/agents/", true},
		{"/api/v1/orchestrator/", true},
		// Negative cases: paths that should not match.
		{"/api/v1/orchestrator", false},   // missing trailing slash, exact match falls through
		{"/api/v1/orchestrators/", false}, // similar but distinct prefix
		{"/api/v1/audit-jobs/", true},     // sanity check existing prefix still matches
		{"/health", false},
		{"/", false},
	}

	for _, tc := range cases {
		got := gateway.ShouldHandle(tc.path)
		if got != tc.want {
			t.Errorf("ShouldHandle(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

// TestNew_RegistersOrchestratorHandler verifies that when the orchestrator
// upstream is configured, gateway.New initializes a connection (added to
// the close set) and registers REST handlers under /api/v1/orchestrator/.
func TestNew_RegistersOrchestratorHandler(t *testing.T) {
	cfg := config.Config{
		GRPCUpstream: config.GRPCUpstreamConfig{
			Orchestrator: "localhost:9088",
			Insecure:     true,
		},
	}

	mux, err := gateway.New(cfg)
	if err != nil {
		t.Fatalf("gateway.New returned error: %v", err)
	}
	if mux == nil {
		t.Fatal("expected non-nil mux")
	}
	defer func() {
		_ = mux.Close()
	}()

	// A POST to the registered Invoke endpoint must reach the gateway runtime.
	// Without a live upstream the gRPC dial cannot complete, so we expect a
	// non-404 status: gateway routes this to gRPC and returns a translated
	// upstream error rather than the default mux 404.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orchestrator/invoke", nil)
	rec := httptest.NewRecorder()
	mux.Handler().ServeHTTP(rec, req)

	if rec.Code == http.StatusNotFound {
		t.Errorf("orchestrator route not registered: got 404 for /api/v1/orchestrator/invoke")
	}
}

// TestNew_OrchestratorOptionalWhenUnset verifies the gateway still builds
// successfully when the orchestrator upstream is not configured.
func TestNew_OrchestratorOptionalWhenUnset(t *testing.T) {
	cfg := config.Config{
		GRPCUpstream: config.GRPCUpstreamConfig{
			Insecure: true,
		},
	}

	mux, err := gateway.New(cfg)
	if err != nil {
		t.Fatalf("gateway.New returned error with no upstreams: %v", err)
	}
	defer func() {
		_ = mux.Close()
	}()

	// With nothing registered, an orchestrator path must miss the runtime mux
	// and produce a 404. This guards against accidentally always registering
	// the handler regardless of configuration.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orchestrator/invoke", nil)
	rec := httptest.NewRecorder()
	mux.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 when orchestrator not configured, got %d", rec.Code)
	}
}
