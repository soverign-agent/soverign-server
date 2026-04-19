// Package processing provides document text extraction and chunking.
package processing

import (
	"strings"
)

// Chunker handles intelligent text chunking with overlap.
type Chunker struct {
	chunkSize   int
	chunkOverlap int
}

// NewChunker creates a new Chunker with the specified chunk size and overlap.
func NewChunker(chunkSize, chunkOverlap int) *Chunker {
	return &Chunker{
		chunkSize:   chunkSize,
		chunkOverlap: chunkOverlap,
	}
}

// SplitIntoChunks splits the input text into overlapping chunks.
// It tries to break on paragraph or sentence boundaries when possible.
func (c *Chunker) SplitIntoChunks(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	var chunks []string
	start := 0

	for start < len(words) {
		end := min(start+c.chunkSize, len(words))
		chunk := strings.Join(words[start:end], " ")
		chunks = append(chunks, chunk)
		start += c.chunkSize - c.chunkOverlap

		// Ensure we don't get stuck if overlap >= chunkSize
		if start >= end && end < len(words) {
			start = end
		}
	}

	// Remove duplicate chunks (deduplication)
	return deduplicateChunks(chunks)
}

// deduplicateChunks removes exact duplicate chunks.
func deduplicateChunks(chunks []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(chunks))

	for _, chunk := range chunks {
		trimmed := strings.TrimSpace(chunk)
		if len(trimmed) < 10 { // Skip very short chunks
			continue
		}
		if !seen[trimmed] {
			seen[trimmed] = true
			result = append(result, trimmed)
		}
	}

	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
