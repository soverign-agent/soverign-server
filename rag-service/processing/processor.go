// Package processing provides document text extraction and chunking.
package processing

import (
	"context"
	"crypto/sha256"
	"fmt"

	"sovereign-ai-compliance/rag-service/model"
	"sovereign-ai-compliance/rag-service/repo"

	"github.com/google/uuid"
)

// Processor combines extraction and chunking into a pipeline.
type Processor struct {
	extractor *Extractor
	chunker   *Chunker
}

// NewProcessor creates a new combined document processor.
func NewProcessor(extractor *Extractor, chunker *Chunker) *Processor {
	return &Processor{
		extractor: extractor,
		chunker:   chunker,
	}
}

// ProcessedChunk represents a processed text chunk.
type ProcessedChunk struct {
	Index    int
	Text     string
	Checksum [32]byte
}

// Process processes a document: extract text and chunk it.
func (p *Processor) Process(content []byte, fileType string) ([]ProcessedChunk, error) {
	text, err := p.extractor.Extract(content, fileType)
	if err != nil {
		return nil, fmt.Errorf("extract text: %w", err)
	}

	chunks := p.chunker.SplitIntoChunks(text)
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no text chunks after processing")
	}

	result := make([]ProcessedChunk, 0, len(chunks))
	for i, chunk := range chunks {
		checksum := sha256.Sum256([]byte(chunk))
		result = append(result, ProcessedChunk{
			Index:    i,
			Text:     chunk,
			Checksum: checksum,
		})
	}

	return result, nil
}

// CreateEmbeddings creates embedding records from processed chunks.
func (p *Processor) CreateEmbeddings(
	ctx context.Context,
	tenantID uuid.UUID,
	documentID uuid.UUID,
	chunks []ProcessedChunk,
	repo *repo.SQLRepository,
) ([]model.Embedding, error) {
	var embeddings []model.Embedding

	for _, chunk := range chunks {
		// Check for duplicate before inserting
		exists, err := repo.ExistsChecksum(ctx, chunk.Checksum)
		if err != nil {
			return nil, fmt.Errorf("check duplicate: %w", err)
		}
		if exists {
			continue // Skip duplicate chunk
		}

		emb := model.Embedding{
			ID:         uuid.New(),
			TenantID:   tenantID,
			DocumentID: documentID,
			ChunkIndex: chunk.Index,
			Text:       chunk.Text,
			Checksum:   chunk.Checksum,
		}
		embeddings = append(embeddings, emb)
	}

	return embeddings, nil
}
