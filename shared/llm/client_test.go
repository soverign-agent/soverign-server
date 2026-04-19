package llm

import (
	"testing"

	"go.uber.org/zap"
)

func TestNewClient_UnsupportedProvider(t *testing.T) {
	logger := zap.NewNop()
	_, err := NewClient(Config{Provider: "unknown"}, logger)
	if err == nil {
		t.Fatal("expected error for unsupported provider")
	}
}

func TestNewClient_OpenAI(t *testing.T) {
	logger := zap.NewNop()
	client, err := NewClient(Config{
		Provider: ProviderOpenAI,
		APIKey:   "test-key",
	}, logger)
	// OpenAI is now implemented
	if err != nil {
		t.Errorf("unexpected error for OpenAI provider: %v", err)
	}
	if client == nil {
		t.Error("expected non-nil client for OpenAI provider")
	}
}

func TestNewClient_NotImplementedProviders(t *testing.T) {
	logger := zap.NewNop()
	providers := []Provider{ProviderAnthropic, ProviderCustom}
	for _, p := range providers {
		_, err := NewClient(Config{Provider: p}, logger)
		// These still return "not yet implemented"
		if err == nil {
			t.Errorf("expected not implemented error for provider %q", p)
		}
	}
}

func TestMessage_Struct(t *testing.T) {
	m := Message{Role: "user", Content: "hello"}
	if m.Role != "user" || m.Content != "hello" {
		t.Error("Message struct fields mismatch")
	}
}

func TestUsage_Total(t *testing.T) {
	u := Usage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30}
	if u.TotalTokens != 30 {
		t.Errorf("expected total tokens 30, got %d", u.TotalTokens)
	}
}
