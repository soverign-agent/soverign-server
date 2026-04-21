package exporter

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sovereign-ai-compliance/doc-service/model"
)

func TestPDFExporter_Export(t *testing.T) {
	doc := &model.GeneratedDocument{
		ID:        uuid.New(),
		Title:     "Test Document",
		DocType:   model.DocTypeAnnexIV,
		Version:   1,
		CreatedAt: time.Now(),
		Content: model.DocumentContent{
			Sections: []model.Section{
				{
					ID:      "sec1",
					Title:   "1. Introduction",
					Content: "This is the introduction section.",
					Order:   1,
					SubSections: []model.SubSection{
						{
							ID:      "sub1",
							Title:   "1.1 Background",
							Content: "Background information here.",
							Order:   1,
						},
					},
				},
				{
					ID:      "sec2",
					Title:   "2. Methodology",
					Content: "- Item one\n- Item two\n\n1. First step\n2. Second step",
					Order:   2,
				},
			},
		},
	}

	exporter := NewPDFExporter()
	outputPath := filepath.Join(t.TempDir(), "test.pdf")

	err := exporter.Export(doc, outputPath)
	require.NoError(t, err)

	info, err := os.Stat(outputPath)
	require.NoError(t, err)
	assert.Greater(t, info.Size(), int64(0), "exported PDF should have non-zero size")
}

func TestPDFExporter_Export_EmptyTitle(t *testing.T) {
	doc := &model.GeneratedDocument{
		ID:        uuid.New(),
		DocType:   model.DocTypeAnnexIV,
		Version:   1,
		CreatedAt: time.Now(),
		Content:   model.DocumentContent{},
	}

	exporter := NewPDFExporter()
	outputPath := filepath.Join(t.TempDir(), "empty.pdf")

	err := exporter.Export(doc, outputPath)
	require.NoError(t, err)

	info, err := os.Stat(outputPath)
	require.NoError(t, err)
	assert.Greater(t, info.Size(), int64(0))
}

func TestPDFExporter_Export_NoSections(t *testing.T) {
	doc := &model.GeneratedDocument{
		ID:        uuid.New(),
		Title:     "No Content",
		DocType:   model.DocTypeAnnexIV,
		Version:   1,
		CreatedAt: time.Now(),
		Content: model.DocumentContent{
			Sections: []model.Section{},
		},
	}

	exporter := NewPDFExporter()
	outputPath := filepath.Join(t.TempDir(), "no_sections.pdf")

	err := exporter.Export(doc, outputPath)
	require.NoError(t, err)

	info, err := os.Stat(outputPath)
	require.NoError(t, err)
	assert.Greater(t, info.Size(), int64(0))
}
