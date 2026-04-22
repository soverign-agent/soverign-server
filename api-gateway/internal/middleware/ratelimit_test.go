package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"sovereign-ai-compliance/shared/tenant"
)

func TestRateLimit_AllowsRequestsUnderLimit(t *testing.T) {
	mw := RateLimit(RateLimiterConfig{RequestsPerSecond: 10, Burst: 2})

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("expected handler to be called")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestRateLimit_RejectsWhenExceeded(t *testing.T) {
	// Very tight limit: 1 rps, burst of 1
	mw := RateLimit(RateLimiterConfig{RequestsPerSecond: 1, Burst: 1})

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request should pass
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected first request to pass, got %d", rec1.Code)
	}

	// Second request immediately should be rate limited
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rec2.Code)
	}
}

func TestRateLimit_PerTenantIsolation(t *testing.T) {
	mw := RateLimit(RateLimiterConfig{RequestsPerSecond: 1, Burst: 1})

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust limit for tenant-a
	ctxA := tenant.WithContext(context.Background(), "tenant-a")
	req1 := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctxA)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected first request for tenant-a to pass, got %d", rec1.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctxA)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected tenant-a to be rate limited, got %d", rec2.Code)
	}

	// tenant-b should still be allowed
	ctxB := tenant.WithContext(context.Background(), "tenant-b")
	req3 := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctxB)
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Errorf("expected tenant-b request to pass, got %d", rec3.Code)
	}
}
