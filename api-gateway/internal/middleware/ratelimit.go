// Package middleware provides HTTP middleware for the API gateway.
package middleware

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
	"sovereign-ai-compliance/shared/tenant"
)

// RateLimiterConfig holds per-tenant rate limiting configuration.
type RateLimiterConfig struct {
	RequestsPerSecond float64
	Burst             int
}

// DefaultRateLimiterConfig returns a sensible default: 100 rps with burst of 150.
func DefaultRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		RequestsPerSecond: 100,
		Burst:             150,
	}
}

// tenantLimiter holds rate limiters per tenant with last-access tracking for cleanup.
type tenantLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimit returns HTTP middleware that enforces per-tenant token-bucket rate limiting.
// Requests without a tenant are rate-limited by a shared "anonymous" bucket.
func RateLimit(cfg RateLimiterConfig) func(http.Handler) http.Handler {
	mu := &sync.RWMutex{}
	limiters := make(map[string]*tenantLimiter)

	// Background cleanup of stale limiters every 5 minutes.
	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		for range ticker.C {
			mu.Lock()
			for id, tl := range limiters {
				if time.Since(tl.lastSeen) > 10*time.Minute {
					delete(limiters, id)
				}
			}
			mu.Unlock()
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := "anonymous"
			if id, ok := tenant.FromContext(r.Context()); ok && id != "" {
				tenantID = id
			}

			mu.RLock()
			tl, ok := limiters[tenantID]
			mu.RUnlock()

			if !ok {
				mu.Lock()
				tl, ok = limiters[tenantID]
				if !ok {
					tl = &tenantLimiter{
						limiter: rate.NewLimiter(rate.Limit(cfg.RequestsPerSecond), cfg.Burst),
					}
					limiters[tenantID] = tl
				}
				mu.Unlock()
			}

			tl.lastSeen = time.Now()
			if !tl.limiter.Allow() {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"code":    429,
					"message": "rate limit exceeded",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
