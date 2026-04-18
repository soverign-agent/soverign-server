// Package middleware provides HTTP middleware for the API gateway.
package middleware

import (
	"net/http"

	"sovereign-ai-compliance/shared/tenant"
)

// Tenant extracts the tenant_id from JWT claims and injects it into the request context.
// This middleware must run after JWTAuth so that claims are present in the context.
func Tenant(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := ClaimsFromContext(r.Context())
		if claims == nil {
			next(w, r)
			return
		}

		// Extract tenant_id from claims. Support both "tenant_id" and "tid" claim keys.
		var tenantID string
		if v, ok := claims["tenant_id"].(string); ok && v != "" {
			tenantID = v
		} else if v, ok := claims["tid"].(string); ok && v != "" {
			tenantID = v
		}

		if tenantID != "" {
			ctx := tenant.WithContext(r.Context(), tenantID)
			r = r.WithContext(ctx)
		}

		next(w, r)
	}
}
