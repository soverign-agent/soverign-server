// Package processing provides document text extraction and chunking.
package processing

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"golang.org/x/net/html"
)

// Extractor extracts plain text from various document formats.
type Extractor struct {
	mdParser goldmark.Markdown
}

// NewExtractor creates a new text extractor.
func NewExtractor() *Extractor {
	return &Extractor{
		mdParser: goldmark.New(
			goldmark.WithExtensions(extension.GFM),
		),
	}
}

// Extract extracts text from content based on file type.
func (e *Extractor) Extract(content []byte, fileType string) (string, error) {
	switch strings.ToLower(fileType) {
	case "txt", "text":
		return string(content), nil
	case "md", "markdown":
		return e.extractMarkdown(content)
	case "pdf":
		// PDF extraction requires external library; for now return error
		return "", fmt.Errorf("PDF extraction not implemented yet")
	case "docx", "doc":
		// DOCX extraction requires external library; for now return error
		return "", fmt.Errorf("DOCX extraction not implemented yet")
	case "html", "htm":
		return e.extractHTML(content)
	default:
		return string(content), nil
	}
}

func (e *Extractor) extractMarkdown(content []byte) (string, error) {
	// Parse markdown to HTML then extract text from HTML
	var buf bytes.Buffer
	if err := e.mdParser.Convert(content, &buf); err != nil {
		return "", fmt.Errorf("convert markdown: %w", err)
	}
	return e.extractHTML(buf.Bytes())
}

func (e *Extractor) extractHTML(content []byte) (string, error) {
	doc := html.NewTokenizer(bytes.NewReader(content))
	var text strings.Builder

	inScript := false
	inStyle := false

	for {
		tt := doc.Next()
		switch tt {
		case html.ErrorToken:
			if doc.Err() == io.EOF {
				return text.String(), nil
			}
			return "", doc.Err()
		case html.StartTagToken:
			token := doc.Token()
			switch token.Data {
			case "script":
				inScript = true
			case "style":
				inStyle = true
			}
		case html.EndTagToken:
			token := doc.Token()
			switch token.Data {
			case "script":
				inScript = false
			case "style":
				inStyle = false
			}
		case html.TextToken:
			if !inScript && !inStyle {
				for _, r := range string(doc.Token().Data) {
					if unicode.IsSpace(r) {
						if text.Len() > 0 && !unicode.IsSpace(rune(text.String()[text.Len()-1])) {
							text.WriteByte(' ')
						}
					} else {
						text.WriteRune(r)
					}
				}
			}
		}
	}
}
