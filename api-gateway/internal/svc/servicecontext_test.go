package svc

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"sovereign-ai-compliance/api-gateway/internal/config"
)

func TestNewServiceContext_ProxyUnavailable(t *testing.T) {
	c := config.Config{
		Auth: config.AuthConfig{PublicKeyPath: "../../certs/jwt-public.pem"},
		Upstream: config.UpstreamConfig{
			Auth: "http://localhost:8001",
		},
		PublicEndpoints: []string{"/public"},
	}

	ctx := NewServiceContext(c)
	if ctx == nil {
		t.Fatal("expected service context")
	}

	// Test proxy for missing service
	handler := ctx.Proxy("nonexistent")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 for missing upstream, got %d", rec.Code)
	}
}

func TestNewServiceContext_BreakersInitialized(t *testing.T) {
	c := config.Config{
		Auth: config.AuthConfig{PublicKeyPath: "../../certs/jwt-public.pem"},
		Upstream: config.UpstreamConfig{
			Auth: "http://localhost:8001",
			Org:  "http://localhost:8002",
		},
		PublicEndpoints: []string{},
	}

	ctx := NewServiceContext(c)
	if len(ctx.Breakers) != 2 {
		t.Errorf("expected 2 breakers, got %d", len(ctx.Breakers))
	}
}
