package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"sovereign-ai-compliance/shared/tenant"
)

func TestTenant_ExtractsFromClaims(t *testing.T) {
	claims := jwt.MapClaims{"tenant_id": "tenant-xyz"}
	ctx := context.WithValue(context.Background(), ck, claims)

	var extractedTenant string
	handler := Tenant(func(w http.ResponseWriter, r *http.Request) {
		id, ok := tenant.FromContext(r.Context())
		if ok {
			extractedTenant = id
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if extractedTenant != "tenant-xyz" {
		t.Errorf("expected tenant-xyz, got %s", extractedTenant)
	}
}

func TestTenant_SkipsWhenNoClaims(t *testing.T) {
	called := false
	handler := Tenant(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if !called {
		t.Error("expected handler to be called even without claims")
	}
}

func TestTenant_FallsBackToTidClaim(t *testing.T) {
	claims := jwt.MapClaims{"tid": "tenant-fallback"}
	ctx := context.WithValue(context.Background(), ck, claims)

	var extractedTenant string
	handler := Tenant(func(w http.ResponseWriter, r *http.Request) {
		id, ok := tenant.FromContext(r.Context())
		if ok {
			extractedTenant = id
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if extractedTenant != "tenant-fallback" {
		t.Errorf("expected tenant-fallback, got %s", extractedTenant)
	}
}
