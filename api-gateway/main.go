// Package main is the API gateway entry point.
package main

import (
	"flag"
	"fmt"
	"net/http"
	"time"

	"sovereign-ai-compliance/api-gateway/internal/config"
	"sovereign-ai-compliance/api-gateway/internal/gateway"
	"sovereign-ai-compliance/api-gateway/internal/handler"
	"sovereign-ai-compliance/api-gateway/internal/middleware"
	"sovereign-ai-compliance/api-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
)

var configFile = flag.String("f", "etc/config.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	// Build service context (reverse proxies, circuit breakers)
	ctx := svc.NewServiceContext(c)

	// Set up grpc-gateway for REST-to-gRPC translation
	grpcGateway, err := gateway.New(c)
	if err != nil {
		panic(fmt.Errorf("failed to create grpc-gateway: %w", err))
	}
	defer grpcGateway.Close()

	// Build the root HTTP handler: mux with grpc-gateway + fallback routes
	rootHandler := handler.NewRootHandler(ctx, grpcGateway)

	// Apply middleware chain: CORS -> JWT auth -> tenant extraction -> rate limit
	// Order matters: Chain wraps right-to-left, requests flow left-to-right.
	// Tenant must run before RateLimit so per-tenant limits use the real tenant.
	chain := middleware.Chain(
		middleware.CORS(middleware.DefaultCORSConfig()),
		middleware.JWTAuth(c.Auth.PublicKeyPath, c.PublicEndpoints),
		middleware.Tenant,
		middleware.RateLimit(middleware.DefaultRateLimiterConfig()),
	)
	finalHandler := chain(rootHandler)

	addr := fmt.Sprintf("%s:%d", c.Host, c.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      finalHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	fmt.Printf("Starting API gateway at %s (grpc-gateway + REST fallback)...\n", addr)
	if err := server.ListenAndServe(); err != nil {
		panic(fmt.Errorf("server failed: %w", err))
	}
}
