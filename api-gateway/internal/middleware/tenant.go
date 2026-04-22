// Package middleware provides HTTP middleware for the API gateway.
package middleware

import (
	"encoding/json"
	"net/http"

	"sovereign-ai-compliance/shared/tenant"
)

// Tenant extracts the tenant_id from JWT claims and injects it into the request context.
// This middleware must run after JWTAuth so that claims are present in the context.
// If claims exist but contain no tenant_id, it returns 400 Bad Request.
func Tenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := ClaimsFromContext(r.Context())
		if claims == nil {
			next.ServeHTTP(w, r)
			return
		}

		// Extract tenant_id from claims. Support both "tenant_id" and "tid" claim keys.
		var tenantID string
		if v, ok := claims["tenant_id"].(string); ok && v != "" {
			tenantID = v
		} else if v, ok := claims["tid"].(string); ok && v != "" {
			tenantID = v
		}

		if tenantID == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code":    400,
				"message": "missing tenant_id in token claims",
			})
			return
		}

		ctx := tenant.WithContext(r.Context(), tenantID)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}
