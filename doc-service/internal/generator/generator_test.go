package generator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"sovereign-ai-compliance/doc-service/model"
	"sovereign-ai-compliance/shared/llm"
)

// mockLLMClient is a test double for llm.Client.
type mockLLMClient struct {
	completeFunc func(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error)
}

func (m *mockLLMClient) Complete(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
	if m.completeFunc != nil {
		return m.completeFunc(ctx, req)
	}
	return llm.CompletionResponse{Content: "mocked content"}, nil
}

func (m *mockLLMClient) Embed(ctx context.Context, req llm.EmbeddingRequest) (llm.EmbeddingResponse, error) {
	return llm.EmbeddingResponse{}, nil
}

func (m *mockLLMClient) Health(ctx context.Context) error {
	return nil
}

func (m *mockLLMClient) StreamComplete(ctx context.Context, req llm.CompletionRequest, onDelta func(token string)) (llm.StreamCompletionResponse, error) {
	resp, err := m.Complete(ctx, req)
	if err != nil {
		return llm.StreamCompletionResponse{}, err
	}
	return llm.StreamCompletionResponse{Content: resp.Content}, nil
}

func TestDocumentGenerator_ValidateContent(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	gen := NewDocumentGenerator(&mockLLMClient{}, logger)

	template := AnnexIVTemplate()
	validContent := &model.DocumentContent{
		Sections: make([]model.Section, 0, len(template.Sections)),
	}
	for _, sec := range template.Sections {
		validContent.Sections = append(validContent.Sections, model.Section{
			ID:      sec.ID,
			Title:   sec.Title,
			Content: "some content",
		})
	}

	t.Run("valid content", func(t *testing.T) {
		err := gen.ValidateContent(validContent, "annex_iv")
		require.NoError(t, err)
	})

	t.Run("nil content", func(t *testing.T) {
		err := gen.ValidateContent(nil, "annex_iv")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "content is nil")
	})

	t.Run("empty sections", func(t *testing.T) {
		err := gen.ValidateContent(&model.DocumentContent{}, "annex_iv")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no sections generated")
	})

	t.Run("missing required section", func(t *testing.T) {
		partial := &model.DocumentContent{
			Sections: []model.Section{{ID: "general_description", Title: "General", Content: "text"}},
		}
		err := gen.ValidateContent(partial, "annex_iv")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing required section")
	})

	t.Run("unknown template", func(t *testing.T) {
		err := gen.ValidateContent(validContent, "unknown_type")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown document template")
	})
}

func TestDocumentGenerator_Generate_NilClient(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	gen := NewDocumentGenerator(nil, logger)

	doc := &model.GeneratedDocument{
		DocType: "annex_iv",
		Content: model.DocumentContent{},
	}
	data := &AssembledData{}

	_, err := gen.Generate(context.Background(), doc, data, GenerateOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "llm client not initialized")
}

func TestDocumentGenerator_Generate_WithMockClient(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mock := &mockLLMClient{
		completeFunc: func(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
			return llm.CompletionResponse{Content: "Generated section content"}, nil
		},
	}
	gen := NewDocumentGenerator(mock, logger)

	doc := &model.GeneratedDocument{
		DocType: "annex_iv",
		Content: model.DocumentContent{},
	}
	data := &AssembledData{
		SystemName:    "Test System",
		SystemVersion: "1.0",
		RiskLevel:     "high",
		RiskScore:     0.75,
	}

	content, err := gen.Generate(context.Background(), doc, data, GenerateOptions{})
	require.NoError(t, err)
	require.NotNil(t, content)

	// Should generate all template sections
	template := AnnexIVTemplate()
	assert.Len(t, content.Sections, len(template.Sections))

	// Verify metadata is set
	assert.Equal(t, "annex_iv", content.Metadata["template_id"])
	assert.Equal(t, "Test System", content.Metadata["system_name"])
}

func TestDocumentGenerator_Generate_PartialRegeneration(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mock := &mockLLMClient{
		completeFunc: func(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
			return llm.CompletionResponse{Content: "Regenerated content"}, nil
		},
	}
	gen := NewDocumentGenerator(mock, logger)

	existingSection := model.Section{
		ID:      "general_description",
		Title:   "1. General Description",
		Content: "Existing content",
	}
	doc := &model.GeneratedDocument{
		DocType: "annex_iv",
		Content: model.DocumentContent{Sections: []model.Section{existingSection}},
	}
	data := &AssembledData{SystemName: "Test"}

	// Regenerate only developer_info, preserve general_description
	opts := GenerateOptions{
		RegenerateSections: []string{"developer_info"},
		PreserveSections:   []string{"general_description"},
	}

	content, err := gen.Generate(context.Background(), doc, data, opts)
	require.NoError(t, err)

	// Verify preserved section retains old content
	var preservedSection *model.Section
	for i := range content.Sections {
		if content.Sections[i].ID == "general_description" {
			preservedSection = &content.Sections[i]
			break
		}
	}
	require.NotNil(t, preservedSection)
	assert.Equal(t, "Existing content", preservedSection.Content)
}

func TestDocumentGenerator_Generate_UnknownTemplate(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	gen := NewDocumentGenerator(&mockLLMClient{}, logger)

	doc := &model.GeneratedDocument{DocType: "unknown"}
	_, err := gen.Generate(context.Background(), doc, &AssembledData{}, GenerateOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown document template")
}
