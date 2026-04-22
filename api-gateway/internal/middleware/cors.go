// Package middleware provides HTTP middleware for the API gateway.
package middleware

import (
	"net/http"
	"strings"
)

// CORSConfig holds CORS middleware configuration.
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
}

// DefaultCORSConfig returns a permissive default suitable for development.
// In production, narrow AllowedOrigins to the web frontend domain.
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
		AllowCredentials: false,
	}
}

// CORS returns HTTP middleware that handles Cross-Origin Resource Sharing.
func CORS(cfg CORSConfig) func(http.Handler) http.Handler {
	origins := make(map[string]struct{}, len(cfg.AllowedOrigins))
	for _, o := range cfg.AllowedOrigins {
		origins[o] = struct{}{}
	}
	allowAll := false
	if _, ok := origins["*"]; ok {
		allowAll = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				if allowAll {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else {
					if _, ok := origins[origin]; ok {
						w.Header().Set("Access-Control-Allow-Origin", origin)
						if cfg.AllowCredentials {
							w.Header().Set("Access-Control-Allow-Credentials", "true")
						}
					}
				}
				w.Header().Set("Access-Control-Allow-Methods", joinMethods(cfg.AllowedMethods))
				w.Header().Set("Access-Control-Allow-Headers", joinHeaders(cfg.AllowedHeaders))
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func joinMethods(methods []string) string {
	if len(methods) == 0 {
		return "GET, POST, PUT, DELETE, OPTIONS"
	}
	return strings.Join(methods, ", ")
}

func joinHeaders(headers []string) string {
	if len(headers) == 0 {
		return "Content-Type, Authorization"
	}
	return strings.Join(headers, ", ")
}
