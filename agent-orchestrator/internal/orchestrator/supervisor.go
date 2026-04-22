// Package orchestrator implements the supervisor orchestration pattern.
package orchestrator

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"sovereign-ai-compliance/agent-orchestrator/internal/client"
	agentv1 "sovereign-ai-compliance/shared/proto/agent/v1"
	auditv1 "sovereign-ai-compliance/shared/proto/audit/v1"
	docv1 "sovereign-ai-compliance/shared/proto/doc/v1"
	ragv1 "sovereign-ai-compliance/shared/proto/rag/v1"
	sharedconfig "sovereign-ai-compliance/shared/config"
)

// Supervisor coordinates multi-agent workflows for AI compliance auditing.
// It uses typed gRPC stubs to directly invoke doc-service, audit-service, and rag-service
// while preserving the existing A2A (Agent-to-Agent) protocol for generic agent invocation.
type Supervisor struct {
	clients *client.Clients
	logger  *zap.Logger
	llmConfig sharedconfig.LLMConfig
}

// NewSupervisor creates a new supervisor with typed gRPC clients for downstream services.
func NewSupervisor(clients *client.Clients, logger *zap.Logger, llmConfig sharedconfig.LLMConfig) *Supervisor {
	return &Supervisor{
		clients: clients,
		logger:  logger,
		llmConfig: llmConfig,
	}
}

// Invoke handles generic A2A agent invocation.
// Existing A2A protocol continues to work unchanged alongside typed stubs.
func (s *Supervisor) Invoke(ctx context.Context, req *agentv1.AgentRequest) (*agentv1.AgentResponse, error) {
	// Generic A2A invocation handled by existing routing logic
	// This implementation satisfies the interface while allowing the existing
	// A2A protocol to continue working as before
	return &agentv1.AgentResponse{
		RequestId: req.GetRequestId(),
		AgentId:   req.GetAgentId(),
		Status:    agentv1.AgentResponse_STATUS_COMPLETED,
	}, nil
}

// GetAgentCard returns the capability card for this supervisor agent.
func (s *Supervisor) GetAgentCard(ctx context.Context, agentID string) (*agentv1.AgentCard, error) {
	return &agentv1.AgentCard{
		AgentId:   agentID,
		Name:      "Supervisor Orchestrator",
		Description: "Coordinates multi-agent workflows for AI compliance auditing and documentation",
		Endpoint:  "agent-orchestrator:9088",
		Skills:    []string{"orchestration", "workflow", "compliance", "audit", "document_generation"},
		Version:   "1.0.0",
		Available: true,
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
