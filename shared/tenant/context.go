// Package tenant provides tenant context propagation for zero-trust multi-tenant isolation.
package tenant

import (
	"context"
	"fmt"
)

// key is an unexported type to prevent collisions with keys defined in other packages.
type key struct{}

// tenantKey is the context key used to store and retrieve tenant IDs.
var tenantKey = &key{}

// WithContext returns a new context with the tenant ID attached.
func WithContext(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantKey, tenantID)
}

// FromContext extracts the tenant ID from the context.
// Returns the tenant ID and true if present; empty string and false otherwise.
func FromContext(ctx context.Context) (string, bool) {
	v := ctx.Value(tenantKey)
	if v == nil {
		return "", false
	}
	tenantID, ok := v.(string)
	return tenantID, ok
}

// MustFromContext extracts the tenant ID from the context, panicking if it is missing.
// Use this only in code paths where a tenant must always be present.
func MustFromContext(ctx context.Context) string {
	tenantID, ok := FromContext(ctx)
	if !ok || tenantID == "" {
		panic(fmt.Errorf("tenant context required but not found"))
	}
	return tenantID
}
