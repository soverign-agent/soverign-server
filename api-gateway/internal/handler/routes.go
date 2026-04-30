// Package handler registers API gateway routes.
package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"sovereign-ai-compliance/api-gateway/internal/gateway"
	"sovereign-ai-compliance/api-gateway/internal/svc"

	repov1 "sovereign-ai-compliance/shared/proto/repo/v1"
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

	// Webhook handler: bypass grpc-gateway JSON marshaler to preserve raw body
	if repoClient := grpcGateway.RepoClient(); repoClient != nil {
		mux.HandleFunc("/api/v1/repository-webhooks/", handleWebhook(repoClient))
	}

	// grpc-gateway catch-all for services with REST-to-gRPC proto annotations
	gwHandler := grpcGateway.Handler()
	mux.Handle("/api/v1/", gwHandler)

	return mux
}

func handleWebhook(repoClient repov1.RepoServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Extract repository_id from path: /api/v1/repository-webhooks/{repository_id}
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/repository-webhooks/")
		repoID := strings.SplitN(path, "/", 2)[0]
		if repoID == "" {
			http.Error(w, "missing repository_id", http.StatusBadRequest)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Build gRPC metadata with signature headers
		md := metadata.MD{}
		if v := r.Header.Get("x-hub-signature-256"); v != "" {
			md.Set("x-hub-signature-256", v)
		}
		if v := r.Header.Get("x-gitlab-token"); v != "" {
			md.Set("x-gitlab-token", v)
		}
		ctx := metadata.NewOutgoingContext(r.Context(), md)

		resp, err := repoClient.ProcessWebhook(ctx, &repov1.ProcessWebhookRequest{
			RepositoryId: repoID,
			Payload:      body,
		})
		if err != nil {
			st, _ := status.FromError(err)
			switch st.Code() {
			case codes.InvalidArgument:
				http.Error(w, st.Message(), http.StatusBadRequest)
			case codes.NotFound:
				http.Error(w, st.Message(), http.StatusNotFound)
			case codes.Unauthenticated:
				http.Error(w, st.Message(), http.StatusUnauthorized)
			default:
				http.Error(w, st.Message(), http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"success": resp.Success,
			"message": resp.Message,
		})
	}
}
