// Package exporter provides document export capabilities for PDF and DOCX formats.
package exporter

import (
	"fmt"
	"os"
	"strings"

	"github.com/fumiama/go-docx"
	"sovereign-ai-compliance/doc-service/model"
)

// DOCXExporter generates DOCX documents from generated content.
type DOCXExporter struct{}

// NewDOCXExporter creates a new DOCXExporter.
func NewDOCXExporter() *DOCXExporter {
	return &DOCXExporter{}
}

// Export converts a GeneratedDocument to DOCX and writes it to outputPath.
func (e *DOCXExporter) Export(doc *model.GeneratedDocument, outputPath string) error {
	w := docx.New().WithDefaultTheme().WithA4Page()

	// Title
	title := doc.Title
	if title == "" {
		title = "Compliance Documentation"
	}
	p := w.AddParagraph().Justification("center")
	p.AddText(title).Size("48").Bold()

	p = w.AddParagraph().Justification("center")
	p.AddText(fmt.Sprintf("Document Type: %s", doc.DocType)).Size("22")

	p = w.AddParagraph().Justification("center")
	p.AddText(fmt.Sprintf("Version: %d", doc.Version)).Size("22")

	p = w.AddParagraph().Justification("center")
	p.AddText(fmt.Sprintf("Generated: %s", doc.CreatedAt.Format("2006-01-02"))).Size("22")

	// Page break after title
	w.AddParagraph().AddPageBreaks()

	// Content sections
	for _, section := range doc.Content.Sections {
		// Section heading
		p = w.AddParagraph()
		p.AddText(section.Title).Size("32").Bold().Color("003366")

		// Section content
		if section.Content != "" {
			e.renderMarkdown(w, section.Content)
		}

		// Subsections
		for _, sub := range section.SubSections {
			p = w.AddParagraph()
			p.AddText(sub.Title).Size("28").Bold().Color("003366")

			if sub.Content != "" {
				e.renderMarkdown(w, sub.Content)
			}
		}
	}

	// Write to file
	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create docx file: %w", err)
	}
	defer f.Close()

	_, err = w.WriteTo(f)
	if err != nil {
		return fmt.Errorf("serialize docx: %w", err)
	}

	return nil
}

// renderMarkdown renders markdown-like text into docx paragraphs.
func (e *DOCXExporter) renderMarkdown(w *docx.Docx, text string) {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Handle headers
		if strings.HasPrefix(line, "# ") {
			p := w.AddParagraph()
			p.AddText(strings.TrimPrefix(line, "# ")).Size("28").Bold()
			continue
		}
		if strings.HasPrefix(line, "## ") {
			p := w.AddParagraph()
			p.AddText(strings.TrimPrefix(line, "## ")).Size("26").Bold()
			continue
		}
		if strings.HasPrefix(line, "### ") {
			p := w.AddParagraph()
			p.AddText(strings.TrimPrefix(line, "### ")).Size("24").Bold().Italic()
			continue
		}

		// Handle bullet points
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			content := strings.TrimPrefix(strings.TrimPrefix(line, "- "), "* ")
			p := w.AddParagraph()
			p.AddText("\u2022 " + content).Size("20")
			continue
		}

		// Remove bold markers for plain text
		line = strings.ReplaceAll(line, "**", "")
		line = strings.ReplaceAll(line, "__", "")

		// Regular paragraph
		p := w.AddParagraph()
		p.AddText(line).Size("20")
	}
}
