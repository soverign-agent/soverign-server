// Package handler registers API gateway routes.
package handler

import (
	"net/http"

	"sovereign-ai-compliance/api-gateway/internal/gateway"
	"sovereign-ai-compliance/api-gateway/internal/svc"
)

// NewRootHandler builds the root HTTP handler that combines grpc-gateway
// with reverse-proxy fallback for services without grpc-gateway support.
func NewRootHandler(serverCtx *svc.ServiceContext, grpcGateway *gateway.Mux) http.Handler {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// grpc-gateway catch-all for services with REST-to-gRPC proto annotations
	gwHandler := grpcGateway.Handler()
	mux.Handle("/api/v1/", gwHandler)

	return mux
}
