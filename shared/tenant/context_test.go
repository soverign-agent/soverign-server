package tenant

import (
	"context"
	"testing"
)

func TestWithContextAndFromContext(t *testing.T) {
	ctx := context.Background()

	// No tenant in empty context
	_, ok := FromContext(ctx)
	if ok {
		t.Error("expected no tenant in empty context")
	}

	// Attach tenant and retrieve
	ctx = WithContext(ctx, "tenant-123")
	tenantID, ok := FromContext(ctx)
	if !ok {
		t.Fatal("expected tenant after WithContext")
	}
	if tenantID != "tenant-123" {
		t.Errorf("expected tenant-123, got %s", tenantID)
	}
}

func TestMustFromContext(t *testing.T) {
	ctx := WithContext(context.Background(), "tenant-456")
	if got := MustFromContext(ctx); got != "tenant-456" {
		t.Errorf("expected tenant-456, got %s", got)
	}
}

func TestMustFromContext_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for missing tenant context")
		}
	}()
	MustFromContext(context.Background())
}
