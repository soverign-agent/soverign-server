package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChatSessionsExactPathDoesNotRedirectToTrailingSlash(t *testing.T) {
	gwHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	mux := http.NewServeMux()
	mux.Handle("/api/v1/chat/sessions", gwHandler)
	mux.HandleFunc("/api/v1/chat/sessions/", handleChatSSE(nil, gwHandler))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/sessions?page=1&page_size=30", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected exact path handler to serve request without redirect, got status %d", rec.Code)
	}

	if location := rec.Header().Get("Location"); location != "" {
		t.Fatalf("expected no redirect location header, got %q", location)
	}
}

func TestChatSessionsTrailingSlashFallsBackToGatewayWithoutRedirectLoop(t *testing.T) {
	gwHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/chat/sessions" {
			t.Fatalf("expected normalized gateway path, got %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	handler := handleChatSSE(nil, gwHandler)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/sessions/?page=1&page_size=30", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected gateway fallback to handle trailing slash request, got status %d", rec.Code)
	}

	if location := rec.Header().Get("Location"); location != "" {
		t.Fatalf("expected no redirect location header, got %q", location)
	}
}
