// Package exporter provides document export capabilities for PDF and DOCX formats.
package exporter

import (
	"fmt"
	"strings"

	"github.com/go-pdf/fpdf"
	"sovereign-ai-compliance/doc-service/model"
)

// PDFExporter generates PDF documents from generated content.
type PDFExporter struct{}

// NewPDFExporter creates a new PDFExporter.
func NewPDFExporter() *PDFExporter {
	return &PDFExporter{}
}

// Export converts a GeneratedDocument to PDF and writes it to outputPath.
func (e *PDFExporter) Export(doc *model.GeneratedDocument, outputPath string) error {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 20)

	// Title page
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 24)
	pdf.Ln(60)

	// Calculate title width for centering
	title := doc.Title
	if title == "" {
		title = "Compliance Documentation"
	}
	pdf.CellFormat(0, 12, title, "", 1, "C", false, 0, "")

	pdf.SetFont("Arial", "", 12)
	pdf.Ln(10)
	pdf.CellFormat(0, 8, fmt.Sprintf("Document Type: %s", doc.DocType), "", 1, "C", false, 0, "")
	pdf.CellFormat(0, 8, fmt.Sprintf("Version: %d", doc.Version), "", 1, "C", false, 0, "")
	pdf.CellFormat(0, 8, fmt.Sprintf("Generated: %s", doc.CreatedAt.Format("2006-01-02")), "", 1, "C", false, 0, "")

	// Content pages
	for _, section := range doc.Content.Sections {
		pdf.AddPage()

		// Section heading
		pdf.SetFont("Arial", "B", 16)
		pdf.SetTextColor(0, 51, 102)
		pdf.CellFormat(0, 10, section.Title, "", 1, "L", false, 0, "")
		pdf.Ln(4)

		// Reset color
		pdf.SetTextColor(0, 0, 0)

		// Section content
		if section.Content != "" {
			e.renderMarkdown(pdf, section.Content)
		}

		// Subsections
		for _, sub := range section.SubSections {
			pdf.SetFont("Arial", "B", 13)
			pdf.SetTextColor(0, 51, 102)
			pdf.Ln(6)
			pdf.CellFormat(0, 8, sub.Title, "", 1, "L", false, 0, "")
			pdf.SetTextColor(0, 0, 0)

			if sub.Content != "" {
				e.renderMarkdown(pdf, sub.Content)
			}
		}
	}

	return pdf.OutputFileAndClose(outputPath)
}

// renderMarkdown renders markdown-like text as plain text with basic formatting.
func (e *PDFExporter) renderMarkdown(pdf *fpdf.Fpdf, text string) {
	pdf.SetFont("Arial", "", 11)

	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			pdf.Ln(4)
			continue
		}

		// Handle headers
		if strings.HasPrefix(line, "# ") {
			pdf.SetFont("Arial", "B", 14)
			pdf.CellFormat(0, 8, strings.TrimPrefix(line, "# "), "", 1, "L", false, 0, "")
			pdf.SetFont("Arial", "", 11)
			continue
		}
		if strings.HasPrefix(line, "## ") {
			pdf.SetFont("Arial", "B", 12)
			pdf.CellFormat(0, 7, strings.TrimPrefix(line, "## "), "", 1, "L", false, 0, "")
			pdf.SetFont("Arial", "", 11)
			continue
		}
		if strings.HasPrefix(line, "### ") {
			pdf.SetFont("Arial", "BI", 11)
			pdf.CellFormat(0, 6, strings.TrimPrefix(line, "### "), "", 1, "L", false, 0, "")
			pdf.SetFont("Arial", "", 11)
			continue
		}

		// Handle bullet points
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			content := strings.TrimPrefix(strings.TrimPrefix(line, "- "), "* ")
			pdf.CellFormat(6, 6, "\u2022", "", 0, "L", false, 0, "")
			pdf.MultiCell(0, 6, content, "", "L", false)
			continue
		}

		// Handle numbered lists
		if len(line) > 2 && line[0] >= '0' && line[0] <= '9' && strings.HasPrefix(line[1:], ". ") {
			pdf.CellFormat(0, 6, line, "", 1, "L", false, 0, "")
			continue
		}

		// Handle bold text markers
		line = strings.ReplaceAll(line, "**", "")
		line = strings.ReplaceAll(line, "__", "")

		// Regular paragraph
		pdf.MultiCell(0, 6, line, "", "J", false)
	}
}
