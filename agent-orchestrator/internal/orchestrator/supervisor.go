// Package orchestrator implements the supervisor orchestration pattern.
package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"sovereign-ai-compliance/agent-orchestrator/internal/client"
	sharedconfig "sovereign-ai-compliance/shared/config"
	"sovereign-ai-compliance/shared/eval"
	"sovereign-ai-compliance/shared/llm"
	agentv1 "sovereign-ai-compliance/shared/proto/agent/v1"
	auditv1 "sovereign-ai-compliance/shared/proto/audit/v1"
	docv1 "sovereign-ai-compliance/shared/proto/doc/v1"
	ragv1 "sovereign-ai-compliance/shared/proto/rag/v1"
	"sovereign-ai-compliance/shared/tenant"
)

// Supervisor coordinates multi-agent workflows for AI compliance auditing.
// It uses typed gRPC stubs to directly invoke doc-service, audit-service, and rag-service
// while preserving the existing A2A (Agent-to-Agent) protocol for generic agent invocation.
type Supervisor struct {
	clients     *client.Clients
	logger      *zap.Logger
	llmConfig   sharedconfig.LLMConfig
	llmClient   llm.Client
	evalPipeline *eval.Pipeline
}

// NewSupervisor creates a new supervisor with typed gRPC clients for downstream services.
func NewSupervisor(clients *client.Clients, logger *zap.Logger, llmConfig sharedconfig.LLMConfig, llmClient llm.Client, evalPipeline *eval.Pipeline) *Supervisor {
	return &Supervisor{
		clients:      clients,
		logger:       logger,
		llmConfig:    llmConfig,
		llmClient:    llmClient,
		evalPipeline: evalPipeline,
	}
}

// payloadEnvelope is the JSON shape we accept on AgentRequest.payload. Callers may
// supply a list of chat messages, a single prompt, or a free-form text field.
// The first non-empty form wins.
type payloadEnvelope struct {
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	Prompt string `json:"prompt"`
	Input  string `json:"input"`
	Text   string `json:"text"`
}

// Invoke handles synchronous A2A agent invocation by performing a streaming LLM
// completion. Tenant ID is propagated from request metadata so the LLM client
// can label inference metrics correctly.
func (s *Supervisor) Invoke(ctx context.Context, req *agentv1.AgentRequest) (*agentv1.AgentResponse, error) {
	if s.llmClient == nil {
		return failedResponse(req, "llm client not configured"), nil
	}
	ctx = ensureTenant(ctx, req.GetTenantId())

	messages, err := messagesFromPayload(req.GetPayload())
	if err != nil {
		return failedResponse(req, fmt.Sprintf("invalid payload: %v", err)), nil
	}
	if len(messages) == 0 {
		return failedResponse(req, "empty payload: provide messages, prompt, input, or text"), nil
	}

	completion, err := s.llmClient.StreamComplete(ctx, llm.CompletionRequest{
		Model:       s.llmConfig.Model,
		Messages:    messages,
		Temperature: s.llmConfig.Temperature,
		MaxTokens:   s.llmConfig.MaxTokens,
	}, nil)
	if err != nil {
		s.logger.Error("llm stream completion failed",
			zap.String("request_id", req.GetRequestId()),
			zap.Error(err),
		)
		return failedResponse(req, err.Error()), nil
	}

	if s.evalPipeline != nil {
		if _, err := s.evalPipeline.Evaluate(ctx, eval.EvalInput{
			TenantID:   req.GetTenantId(),
			Model:      s.llmConfig.Model,
			OutputText: completion.Content,
			RAGContext: nil, // TODO: fetch from rag-service when payload includes context IDs
		}); err != nil {
			s.logger.Warn("eval pipeline failed",
				zap.String("request_id", req.GetRequestId()),
				zap.Error(err),
			)
		}
	}

	return &agentv1.AgentResponse{
		RequestId: req.GetRequestId(),
		AgentId:   req.GetAgentId(),
		Status:    agentv1.AgentResponse_STATUS_COMPLETED,
		Result:    []byte(completion.Content),
		Metadata:  completionMetadata(completion),
	}, nil
}

func messagesFromPayload(payload []byte) ([]llm.Message, error) {
	if len(payload) == 0 {
		return nil, nil
	}
	var env payloadEnvelope
	if err := json.Unmarshal(payload, &env); err != nil {
		// Treat non-JSON payload as a raw user prompt.
		return []llm.Message{{Role: "user", Content: string(payload)}}, nil
	}
	if len(env.Messages) > 0 {
		out := make([]llm.Message, 0, len(env.Messages))
		for _, m := range env.Messages {
			role := m.Role
			if role == "" {
				role = "user"
			}
			out = append(out, llm.Message{Role: role, Content: m.Content})
		}
		return out, nil
	}
	for _, candidate := range []string{env.Prompt, env.Input, env.Text} {
		if candidate != "" {
			return []llm.Message{{Role: "user", Content: candidate}}, nil
		}
	}
	return nil, nil
}

func completionMetadata(c llm.StreamCompletionResponse) map[string]string {
	return map[string]string{
		"finish_reason":     c.FinishReason,
		"prompt_tokens":     fmt.Sprintf("%d", c.Usage.PromptTokens),
		"completion_tokens": fmt.Sprintf("%d", c.Usage.CompletionTokens),
		"total_tokens":      fmt.Sprintf("%d", c.Usage.TotalTokens),
		"ttft_ms":           fmt.Sprintf("%d", c.Timings.TTFT.Milliseconds()),
		"tpot_ms":           fmt.Sprintf("%d", c.Timings.TPOT.Milliseconds()),
		"total_duration_ms": fmt.Sprintf("%d", c.Timings.TotalDuration.Milliseconds()),
	}
}

func failedResponse(req *agentv1.AgentRequest, msg string) *agentv1.AgentResponse {
	return &agentv1.AgentResponse{
		RequestId:    req.GetRequestId(),
		AgentId:      req.GetAgentId(),
		Status:       agentv1.AgentResponse_STATUS_FAILED,
		ErrorMessage: msg,
	}
}

// ensureTenant prefers tenant ID already in context (set by the gRPC interceptor
// from x-tenant-id metadata). It falls back to AgentRequest.tenant_id so direct
// callers without metadata still produce labelled metrics.
func ensureTenant(ctx context.Context, requestTenantID string) context.Context {
	if _, ok := tenant.FromContext(ctx); ok {
		return ctx
	}
	if requestTenantID != "" {
		return tenant.WithContext(ctx, requestTenantID)
	}
	return ctx
}

// GetAgentCard returns the capability card for this supervisor agent.
func (s *Supervisor) GetAgentCard(ctx context.Context, agentID string) (*agentv1.AgentCard, error) {
	return &agentv1.AgentCard{
		AgentId:     agentID,
		Name:        "Supervisor Orchestrator",
		Description: "Coordinates multi-agent workflows for AI compliance auditing and documentation",
		Endpoint:    "agent-orchestrator:9088",
		Skills:      []string{"orchestration", "workflow", "compliance", "audit", "document_generation"},
		Version:     "1.0.0",
		Available:   true,
	}, nil
}

// TriggerDocumentGeneration triggers document generation via the doc-service wrapper.
func (s *Supervisor) TriggerDocumentGeneration(ctx context.Context, aiSystemID string, docType, title string) (*docv1.GenerateDocumentResponse, error) {
	id, err := uuid.Parse(aiSystemID)
	if err != nil {
		return nil, fmt.Errorf("invalid ai system id: %w", err)
	}
	return s.clients.Doc.GenerateDocument(ctx, id, docType, title)
}

// TriggerAudit triggers an audit via the audit-service wrapper.
func (s *Supervisor) TriggerAudit(ctx context.Context, repositoryID string, name, auditType string) (*auditv1.TriggerAuditResponse, error) {
	id, err := uuid.Parse(repositoryID)
	if err != nil {
		return nil, fmt.Errorf("invalid repository id: %w", err)
	}
	return s.clients.Audit.TriggerAudit(ctx, id, name, auditType)
}

// SearchKnowledge searches the knowledge base via the rag-service wrapper.
func (s *Supervisor) SearchKnowledge(ctx context.Context, query string, topK int32) (*ragv1.SearchResponse, error) {
	return s.clients.RAG.Search(ctx, query, topK)
}
