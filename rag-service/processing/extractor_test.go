package processing

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"github.com/go-pdf/fpdf"
)

func TestExtractor_ExtractPDF(t *testing.T) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("pdftotext not installed")
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "", 14)
	pdf.Cell(40, 10, "Resume Content: Chen Zilong")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		t.Fatalf("generate pdf: %v", err)
	}

	extractor := NewExtractor()
	text, err := extractor.Extract(buf.Bytes(), "pdf")
	if err != nil {
		t.Fatalf("extract pdf: %v", err)
	}
	if !strings.Contains(text, "Resume Content: Chen Zilong") {
		t.Fatalf("expected extracted text to contain PDF content, got %q", text)
	}
}
