// Package orchestrator implements the agentic RAG pipeline for compliance chat.
package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"sovereign-ai-compliance/shared/llm"
)

var guardrailTracer = otel.Tracer("rag-service/orchestrator/guardrail")

// complianceKeywords is a broad, heuristic keyword list used as a fast pre-filter
// before invoking the LLM-based guardrail. It reduces unnecessary LLM calls for
// obviously off-topic queries.
var complianceKeywords = []string{
	"compliance", "ai act", "eu ai act", "regulation", "risk", "annex iv",
	"documentation", "audit", "conformity", "assessment", "high-risk",
	"transparency", "governance", "ethical", "algorithm", "model",
	"data protection", "gdpr", "policy", "standard", "certification",
	"liability", "accountability", "human oversight", "accuracy",
	"robustness", "safety", "security", "bias", "fairness",
	"explainability", "traceability", "logging", "monitoring",
	"incident", "breach", "violation", "penalty", "fine",
	"provider", "deployer", "importer", "distributor", "operator",
	"notified body", "conformity assessment", "technical documentation",
	"quality management", "post-market", "vigilance", "systemic risk",
	"general purpose", "foundation model", "generative ai", "chatbot",
	"deepfake", "biometric", "emotion recognition", "social scoring",
	"manipulation", "subliminal", "exploitation", "vulnerability",
	"children", "disability", "discrimination", "profiling",
	"real-time", "remote", "law enforcement", "migration",
	"asylum", "border control", "justice", "democratic process",
	"election", "recommender system", "content moderation",
	"synthetic data", "training data", "validation data", "test data",
	"data governance", "data quality", "data preparation",
	"model validation", "model verification", "risk management",
	"mitigation measure", "control measure", "safeguard",
}

// GuardrailConfig controls the domain guardrail behaviour.
type GuardrailConfig struct {
	Enabled        bool
	UseLLMCheck    bool
	BlockThreshold float64 // 0.0-1.0; scores below this are blocked
}

// DefaultGuardrailConfig returns sensible defaults.
func DefaultGuardrailConfig() GuardrailConfig {
	return GuardrailConfig{
		Enabled:        true,
		UseLLMCheck:    true,
		BlockThreshold: 0.3,
	}
}

// GuardrailNode rejects questions that are outside the AI Act compliance domain.
func GuardrailNode(cfg GuardrailConfig, client llm.Client) NodeFunc {
	return func(ctx context.Context, state *PipelineState) error {
		ctx, span := guardrailTracer.Start(ctx, "GuardrailNode")
		defer span.End()

		span.SetAttributes(
			attribute.Bool("enabled", cfg.Enabled),
			attribute.Bool("use_llm_check", cfg.UseLLMCheck),
			attribute.Float64("block_threshold", cfg.BlockThreshold),
			attribute.String("query", truncateAttr(state.Input.Query, 200)),
		)

		if !cfg.Enabled {
			span.SetAttributes(attribute.String("result", "skipped_disabled"))
			return nil
		}

		state.AddProgress(StageGuardrail, "Checking question domain...", nil, 0)

		query := strings.ToLower(state.Input.Query)

		// Fast-path: heuristic keyword check
		heuristicScore := heuristicComplianceScore(query)
		if heuristicScore >= 0.7 {
			span.SetAttributes(attribute.String("result", "passed_heuristic"))
			return nil
		}

		if !cfg.UseLLMCheck {
			if heuristicScore < cfg.BlockThreshold {
				state.Analysis.IsOffTopic = true
				state.Analysis.OffTopicReason = "This question appears to be outside the scope of AI Act compliance. Please ask a question related to EU AI Act compliance, risk assessment, or technical documentation."
				span.SetAttributes(attribute.String("result", "blocked_heuristic"))
				return ErrOffTopicQuestion
			}
			span.SetAttributes(attribute.String("result", "passed_heuristic_low_confidence"))
			return nil
		}

		// LLM-based classification for ambiguous queries
		score, reason, err := llmGuardrailCheck(ctx, client, state.Input.Query)
		if err != nil {
			// Fail open on LLM error: log but don't block
			return nil
		}

		if score < cfg.BlockThreshold {
			state.Analysis.IsOffTopic = true
			state.Analysis.OffTopicReason = reason
			return ErrOffTopicQuestion
		}

		return nil
	}
}

// heuristicComplianceScore returns a rough 0.0-1.0 score based on keyword overlap.
func heuristicComplianceScore(query string) float64 {
	query = strings.ToLower(query)
	matches := 0
	for _, kw := range complianceKeywords {
		if strings.Contains(query, kw) {
			matches++
		}
	}
	// Score saturates quickly; 3+ keyword hits = high confidence
	if matches >= 3 {
		return 1.0
	}
	if matches == 2 {
		return 0.6
	}
	if matches == 1 {
		return 0.3
	}
	return 0.0
}

// guardrailResponse is the expected JSON shape from the LLM guardrail prompt.
type guardrailResponse struct {
	IsCompliant bool    `json:"is_compliant"`
	Score       float64 `json:"score"`
	Reason      string  `json:"reason"`
}

func llmGuardrailCheck(ctx context.Context, client llm.Client, query string) (float64, string, error) {
	prompt := fmt.Sprintf(`You are a domain guardrail for an EU AI Act compliance platform.
Your job is to decide whether the following user question is related to AI Act compliance, risk assessment, technical documentation, audit, or governance.

User question: "%s"

Respond with a JSON object exactly in this format (no markdown, no extra text):
{"is_compliant": true|false, "score": 0.0-1.0, "reason": "short explanation"}

Rules:
- score >= 0.5: the question is on-topic (compliance-related)
- score < 0.5: the question is off-topic
- Be lenient with follow-up questions that refer to previous compliance context.`, query)

	resp, err := client.Complete(ctx, llm.CompletionRequest{
		Messages: []llm.Message{
			{Role: "system", Content: "You are a helpful classifier that only outputs valid JSON."},
			{Role: "user", Content: prompt},
		},
		Temperature: 0.0,
		MaxTokens:   256,
	})
	if err != nil {
		return 0, "", err
	}

	var result guardrailResponse
	clean := strings.TrimSpace(resp.Content)
	// Strip markdown code fences if present
	clean = strings.TrimPrefix(clean, "```json")
	clean = strings.TrimPrefix(clean, "```")
	clean = strings.TrimSuffix(clean, "```")
	clean = strings.TrimSpace(clean)

	if err := json.Unmarshal([]byte(clean), &result); err != nil {
		return 0, "", fmt.Errorf("unmarshal guardrail response: %w", err)
	}

	if result.IsCompliant {
		return result.Score, "", nil
	}
	return result.Score, result.Reason, nil
}
