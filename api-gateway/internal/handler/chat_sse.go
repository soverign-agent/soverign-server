package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"google.golang.org/grpc/metadata"

	"sovereign-ai-compliance/api-gateway/internal/middleware"
	ragv1 "sovereign-ai-compliance/shared/proto/rag/v1"
	"sovereign-ai-compliance/shared/tenant"
)

// handleChatSSE returns an http.HandlerFunc that intercepts POST requests to
// /api/v1/chat/sessions/{session_id}/messages and translates the gRPC server
// streaming Chat RPC into proper SSE format. All other requests are delegated
// to the grpc-gateway handler.
func handleChatSSE(ragClient ragv1.RAGServiceClient, gwHandler http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Only intercept POST requests to /api/v1/chat/sessions/{session_id}/messages
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/chat/sessions/")
		parts := strings.SplitN(path, "/", 2)
		if r.Method != http.MethodPost || len(parts) != 2 || parts[1] != "messages" {
			// Normalize a manually requested trailing-slash collection path so the
			// grpc-gateway collection routes can serve it without another redirect.
			if r.URL.Path == "/api/v1/chat/sessions/" {
				r.URL.Path = "/api/v1/chat/sessions"
			}
			gwHandler.ServeHTTP(w, r)
			return
		}

		sessionID := parts[0]
		if sessionID == "" {
			http.Error(w, "missing session_id", http.StatusBadRequest)
			return
		}

		// Parse request body for message
		var body struct {
			Message string `json:"message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Build gRPC metadata with tenant and user context
		md := metadata.MD{}
		if tenantID, ok := tenant.FromContext(r.Context()); ok && tenantID != "" {
			md.Set("x-tenant-id", tenantID)
		}
		if claims := middleware.ClaimsFromContext(r.Context()); claims != nil {
			if userID, ok := claims["user_id"].(string); ok && userID != "" {
				md.Set("x-user-id", userID)
			}
		}
		ctx := metadata.NewOutgoingContext(r.Context(), md)

		// Call the gRPC streaming Chat method
		stream, err := ragClient.Chat(ctx, &ragv1.ChatRequest{
			SessionId: sessionID,
			Message:   body.Message,
		})
		if err != nil {
			writeSSEError(w, "failed to start chat stream", "")
			return
		}

		// Set SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			writeSSEError(w, "streaming not supported", "")
			return
		}

		// Stream events
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				writeSSEEvent(w, flusher, map[string]any{
					"type":    "error",
					"message": err.Error(),
					"code":    "internal",
				})
				return
			}

			event := chatResponseToEvent(resp)
			writeSSEEvent(w, flusher, event)

			// Check if this is a terminal event
			if resp.GetDone() != nil || resp.GetError() != nil {
				break
			}
		}
	}
}

// chatResponseToEvent converts a ChatResponse protobuf to a frontend-friendly event map.
func chatResponseToEvent(resp *ragv1.ChatResponse) map[string]any {
	switch {
	case resp.GetToken() != nil:
		return map[string]any{
			"type":  "token",
			"delta": resp.GetToken().GetToken(),
		}
	case resp.GetProgress() != nil:
		return map[string]any{
			"type":    "progress",
			"stage":   resp.GetProgress().GetStage(),
			"message": resp.GetProgress().GetDetail(),
		}
	case resp.GetCitations() != nil:
		citations := make([]map[string]any, 0, len(resp.GetCitations().GetCitations()))
		for _, c := range resp.GetCitations().GetCitations() {
			citations = append(citations, map[string]any{
				"documentId":   c.GetDocumentId(),
				"documentName": c.GetDocumentName(),
				"chunkId":      c.GetChunkId(),
				"snippet":      c.GetSnippet(),
				"similarity":   c.GetSimilarity(),
			})
		}
		return map[string]any{
			"type":      "citation",
			"citations": citations,
		}
	case resp.GetDone() != nil:
		return map[string]any{
			"type":      "done",
			"messageId": resp.GetDone().GetAssistantMessageId(),
		}
	case resp.GetError() != nil:
		return map[string]any{
			"type":    "error",
			"message": resp.GetError().GetMessage(),
			"code":    resp.GetError().GetCode(),
		}
	default:
		return map[string]any{
			"type":    "error",
			"message": "unknown event type",
			"code":    "internal",
		}
	}
}

func writeSSEEvent(w http.ResponseWriter, flusher http.Flusher, event map[string]any) {
	data, _ := json.Marshal(event)
	w.Write([]byte("data: "))
	w.Write(data)
	w.Write([]byte("\n\n"))
	flusher.Flush()
}

func writeSSEError(w http.ResponseWriter, message, code string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	data, _ := json.Marshal(map[string]any{
		"type":    "error",
		"message": message,
		"code":    code,
	})
	w.Write([]byte("data: "))
	w.Write(data)
	w.Write([]byte("\n\n"))
}
