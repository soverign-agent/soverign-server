// Package generator provides document generation capabilities for doc-service.
package generator

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"go.uber.org/zap"
	"sovereign-ai-compliance/doc-service/model"
	"sovereign-ai-compliance/shared/llm"
)

// DocumentGenerator orchestrates LLM-based document generation.
type DocumentGenerator struct {
	llmClient llm.Client
	logger    *zap.Logger
}

// NewDocumentGenerator creates a new DocumentGenerator.
func NewDocumentGenerator(client llm.Client, logger *zap.Logger) *DocumentGenerator {
	return &DocumentGenerator{
		llmClient: client,
		logger:    logger,
	}
}

// GenerateOptions controls document generation behavior.
type GenerateOptions struct {
	RegenerateSections []string // Section IDs to regenerate; empty means all
	PreserveSections   []string // Section IDs to preserve from existing content
}

// Generate creates or regenerates document content using the LLM.
// It iterates over the template sections and calls the LLM for each.
func (g *DocumentGenerator) Generate(
	ctx context.Context,
	doc *model.GeneratedDocument,
	data *AssembledData,
	opts GenerateOptions,
) (*model.DocumentContent, error) {
	template, ok := GetTemplateByID(doc.DocType)
	if !ok {
		return nil, fmt.Errorf("unknown document template: %s", doc.DocType)
	}

	// Determine which sections to regenerate vs preserve
	preserveSet := make(map[string]bool)
	for _, id := range opts.PreserveSections {
		preserveSet[id] = true
	}
	regenerateSet := make(map[string]bool)
	for _, id := range opts.RegenerateSections {
		regenerateSet[id] = true
	}

	// If no specific regeneration requested, regenerate all
	regenerateAll := len(opts.RegenerateSections) == 0

	content := &model.DocumentContent{
		Sections: make([]model.Section, 0, len(template.Sections)),
		Metadata: map[string]string{
			"template_id":    template.ID,
			"template_name":  template.Name,
			"system_name":    data.SystemName,
			"system_version": data.SystemVersion,
			"risk_level":     data.RiskLevel,
			"risk_score":     fmt.Sprintf("%.0f", data.RiskScore*100),
		},
	}

	// Try to preserve existing sections that aren't being regenerated
	existingSections := make(map[string]model.Section)
	if doc.Content.Sections != nil {
		for _, sec := range doc.Content.Sections {
			existingSections[sec.ID] = sec
		}
	}

	var mu sync.Mutex
	var genErr error

	for i, tmplSec := range template.Sections {
		shouldRegenerate := regenerateAll || regenerateSet[tmplSec.ID]
		shouldPreserve := preserveSet[tmplSec.ID] && !shouldRegenerate

		if !shouldRegenerate && shouldPreserve {
			if existing, ok := existingSections[tmplSec.ID]; ok {
				content.Sections = append(content.Sections, existing)
				continue
			}
		}

		if !shouldRegenerate && !shouldPreserve && !regenerateAll {
			// Skip sections not in regenerate list when partial regeneration is specified
			if existing, ok := existingSections[tmplSec.ID]; ok {
				content.Sections = append(content.Sections, existing)
				continue
			}
		}

		// Generate section content via LLM
		section, err := g.generateSection(ctx, tmplSec, data, i+1)
		if err != nil {
			mu.Lock()
			genErr = fmt.Errorf("generate section %s: %w", tmplSec.ID, err)
			mu.Unlock()
			g.logger.Error("section generation failed",
				zap.String("section_id", tmplSec.ID),
				zap.Error(err),
			)
			// Continue with other sections; we'll return the first error but have partial content
			continue
		}

		content.Sections = append(content.Sections, section)
	}

	if genErr != nil {
		return content, genErr
	}

	return content, nil
}

// generateSection generates content for a single section using the LLM.
func (g *DocumentGenerator) generateSection(
	ctx context.Context,
	tmpl SectionTemplate,
	data *AssembledData,
	sectionIndex int,
) (model.Section, error) {
	g.logger.Info("generating section",
		zap.String("section_id", tmpl.ID),
		zap.String("title", tmpl.Title),
	)

	if g.llmClient == nil {
		return model.Section{}, fmt.Errorf("llm client not initialized")
	}

	prompt := BuildSectionPrompt(tmpl, data, sectionIndex)

	req := llm.CompletionRequest{
		Model:       "", // Use default from client config
		Messages:    []llm.Message{{Role: "user", Content: prompt}},
		Temperature: 0.3, // Lower temperature for consistent regulatory documentation
		MaxTokens:   4000,
	}

	resp, err := g.llmClient.Complete(ctx, req)
	if err != nil {
		return model.Section{}, fmt.Errorf("llm completion: %w", err)
	}

	section := model.Section{
		ID:          tmpl.ID,
		Title:       tmpl.Title,
		Content:     strings.TrimSpace(resp.Content),
		Order:       tmpl.Order,
		SubSections: make([]model.SubSection, 0, len(tmpl.SubSections)),
	}

	// Generate subsections if present
	for j, subTmpl := range tmpl.SubSections {
		subPrompt := BuildSubSectionPrompt(tmpl, subTmpl, data, sectionIndex, j+1)
		subReq := llm.CompletionRequest{
			Model:       "",
			Messages:    []llm.Message{{Role: "user", Content: subPrompt}},
			Temperature: 0.3,
			MaxTokens:   2000,
		}

		subResp, err := g.llmClient.Complete(ctx, subReq)
		if err != nil {
			g.logger.Warn("subsection generation failed",
				zap.String("section_id", tmpl.ID),
				zap.String("subsection_id", subTmpl.ID),
				zap.Error(err),
			)
			// Continue without this subsection rather than failing the whole section
			continue
		}

		section.SubSections = append(section.SubSections, model.SubSection{
			ID:      subTmpl.ID,
			Title:   subTmpl.Title,
			Content: strings.TrimSpace(subResp.Content),
			Order:   subTmpl.Order,
		})
	}

	return section, nil
}

// ValidateContent performs basic validation on generated document content.
func (g *DocumentGenerator) ValidateContent(content *model.DocumentContent, docType string) error {
	if content == nil {
		return fmt.Errorf("content is nil")
	}

	template, ok := GetTemplateByID(docType)
	if !ok {
		return fmt.Errorf("unknown document template: %s", docType)
	}

	if len(content.Sections) == 0 {
		return fmt.Errorf("no sections generated")
	}

	// Check that all required sections are present
	sectionMap := make(map[string]bool)
	for _, sec := range content.Sections {
		sectionMap[sec.ID] = true
		if sec.Content == "" {
			g.logger.Warn("section has empty content", zap.String("section_id", sec.ID))
		}
	}

	for _, tmplSec := range template.Sections {
		if !sectionMap[tmplSec.ID] {
			return fmt.Errorf("missing required section: %s", tmplSec.ID)
		}
	}

	return nil
}
