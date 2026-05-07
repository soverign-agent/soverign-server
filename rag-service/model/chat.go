// Package model defines the data models for rag-service.
package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Chat message role enum (mirrors the CHECK constraint in chat_messages).
const (
	ChatRoleUser      = "user"
	ChatRoleAssistant = "assistant"
	ChatRoleSystem    = "system"
)

// MaxChatTurns is the per-session conversation length cap. A "turn" is a
// user+assistant pair, so 20 turns = 40 messages.
const MaxChatTurns = 20

// ChatSession represents a single chat conversation owned by (TenantID, UserID).
type ChatSession struct {
	ID           uuid.UUID `json:"id"`
	TenantID     uuid.UUID `json:"tenant_id"`
	UserID       uuid.UUID `json:"user_id"`
	Title        string    `json:"title"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	MessageCount int       `json:"message_count"`
}

// ChatCitation is the persisted shape of a single citation attached to an
// assistant message. JSON-serialised into the chat_messages.citations column.
type ChatCitation struct {
	DocumentID   string  `json:"document_id"`
	DocumentName string  `json:"document_name"`
	ChunkID      string  `json:"chunk_id,omitempty"`
	Snippet      string  `json:"snippet"`
	Similarity   float64 `json:"similarity"`
}

// ChatMessage represents a single turn within a chat session.
type ChatMessage struct {
	ID        uuid.UUID      `json:"id"`
	SessionID uuid.UUID      `json:"session_id"`
	TenantID  uuid.UUID      `json:"tenant_id"`
	Role      string         `json:"role"`
	Content   string         `json:"content"`
	Citations []ChatCitation `json:"citations"`
	CreatedAt time.Time      `json:"created_at"`
}

// MarshalCitations returns the citations JSON-encoded for storage. An empty
// list is rendered as `[]` rather than `null` so the column always contains
// valid JSON.
func MarshalCitations(citations []ChatCitation) ([]byte, error) {
	if citations == nil {
		citations = []ChatCitation{}
	}
	return json.Marshal(citations)
}

// UnmarshalCitations decodes the citations JSON column. nil/empty bytes
// produce an empty slice rather than an error.
func UnmarshalCitations(raw []byte) ([]ChatCitation, error) {
	if len(raw) == 0 {
		return []ChatCitation{}, nil
	}
	var out []ChatCitation
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []ChatCitation{}
	}
	return out, nil
}
