package llm

import (
	"testing"
)

func TestNewClient_UnsupportedProvider(t *testing.T) {
	_, err := NewClient(Config{Provider: "unknown"})
	if err == nil {
		t.Fatal("expected error for unsupported provider")
	}
}

func TestNewClient_ValidProviders(t *testing.T) {
	providers := []Provider{ProviderOpenAI, ProviderAnthropic, ProviderCustom}
	for _, p := range providers {
		_, err := NewClient(Config{Provider: p})
		// These return "not yet implemented" because concrete implementations
		// are wired at service startup time.
		if err == nil {
			t.Errorf("expected placeholder error for provider %q", p)
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
