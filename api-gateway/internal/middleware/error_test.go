package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCustomErrorHandler_MapsGRPCCodes(t *testing.T) {
	tests := []struct {
		name     string
		grpcCode codes.Code
		wantHTTP int
		wantMsg  string
	}{
		{"NotFound", codes.NotFound, http.StatusNotFound, "resource missing"},
		{"InvalidArgument", codes.InvalidArgument, http.StatusBadRequest, "bad input"},
		{"PermissionDenied", codes.PermissionDenied, http.StatusForbidden, "no access"},
		{"Unauthenticated", codes.Unauthenticated, http.StatusUnauthorized, "not logged in"},
		{"Internal", codes.Internal, http.StatusInternalServerError, "boom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := status.Error(tt.grpcCode, tt.wantMsg)
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()

			handler := CustomErrorHandler()
			handler(context.Background(), nil, nil, rec, req, err)

			if rec.Code != tt.wantHTTP {
				t.Errorf("expected HTTP %d, got %d", tt.wantHTTP, rec.Code)
			}

			var body map[string]interface{}
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if int(body["code"].(float64)) != tt.wantHTTP {
				t.Errorf("expected code %d, got %v", tt.wantHTTP, body["code"])
			}
			if body["message"] != tt.wantMsg {
				t.Errorf("expected message %q, got %v", tt.wantMsg, body["message"])
			}
		})
	}
}

func TestCustomErrorHandler_UnknownError(t *testing.T) {
	err := errors.New("plain go error")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler := CustomErrorHandler()
	handler(context.Background(), &runtime.ServeMux{}, &runtime.JSONPb{}, rec, req, err)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if int(body["code"].(float64)) != 500 {
		t.Errorf("expected code 500, got %v", body["code"])
	}
}
