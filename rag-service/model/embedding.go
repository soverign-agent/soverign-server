// Package model defines the data models for rag-service.
package model

import (
	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"
)

// Embedding represents a text chunk with its embedding vector.
type Embedding struct {
	ID         uuid.UUID          `json:"id"`
	TenantID   uuid.UUID          `json:"tenant_id"`
	DocumentID uuid.UUID          `json:"document_id"`
	ChunkIndex int                `json:"chunk_index"`
	Text       string             `json:"text"`
	Embedding  pgvector.Vector    `json:"-"` // Vector stored in database, not serialized to JSON
	Checksum   [32]byte           `json:"-"` // SHA256 checksum for deduplication
}
