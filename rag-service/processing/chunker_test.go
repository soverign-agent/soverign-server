package processing

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChunker_SplitIntoChunks_SingleChunk(t *testing.T) {
	chunker := NewChunker(100, 20)

	text := `This is a long piece of text that should be split into multiple chunks.
	The chunker should handle overlapping chunks to preserve context between chunks.
	Each chunk should respect the maximum size limit and overlap with the previous.`

	chunks := chunker.SplitIntoChunks(text)
	assert.Equal(t, 1, len(chunks)) // Fits in one chunk
}

func TestChunker_SplitIntoChunks_MultipleChunks(t *testing.T) {
	// Make chunk size smaller to force multiple chunks
	chunker := NewChunker(10, 5)

	text := `This is a long piece of text that should be split into multiple chunks.
	The chunker should handle overlapping chunks to preserve context between chunks.
	Each chunk should respect the maximum size limit and overlap with the previous.`

	chunks := chunker.SplitIntoChunks(text)
	assert.Greater(t, len(chunks), 1)
}

func TestChunker_EmptyText(t *testing.T) {
	chunker := NewChunker(100, 20)
	chunks := chunker.SplitIntoChunks("")
	assert.Empty(t, chunks)
}
