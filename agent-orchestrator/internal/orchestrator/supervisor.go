// Package orchestrator implements the supervisor orchestration pattern.
package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	temporalclient "go.temporal.io/sdk/client"
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
// It dispatches incoming AgentRequests on req.TaskType: typed task types
// (compliance_audit, document_generation, knowledge_search) start Temporal
// workflows, while unrecognized task types fall through to a streaming LLM
// completion so the existing A2A protocol keeps working.
type Supervisor struct {
	clients        *client.Clients
	logger         *zap.Logger
	llmConfig      sharedconfig.LLMConfig
	llmClient      llm.Client
	evalPipeline   *eval.Pipeline
	temporalClient temporalclient.Client
}

// NewSupervisor creates a new supervisor wired with downstream gRPC clients,
// LLM client, evaluation pipeline, and a Temporal client used to start the
// orchestrator workflows. temporalClient may be nil in tests that exercise
// only the LLM-completion path.
func NewSupervisor(
	clients *client.Clients,
	logger *zap.Logger,
	llmConfig sharedconfig.LLMConfig,
	llmClient llm.Client,
	evalPipeline *eval.Pipeline,
	temporalClient temporalclient.Client,
) *Supervisor {
	return &Supervisor{
		clients:        clients,
		logger:         logger,
		llmConfig:      llmConfig,
		llmClient:      llmClient,
		evalPipeline:   evalPipeline,
		temporalClient: temporalClient,
	}
}

// payloadEnvelope is the JSON shape we accept on AgentRequest.payload. Callers may
// supply a list of chat messages, a single prompt, or a free-form text field.
// The first non-empty form wins. Typed task fields (ai_system_id, repository_id, …)
// are read on the same envelope when the TaskType selects a workflow path.
type payloadEnvelope struct {
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	Prompt string `json:"prompt"`
	Input  string `json:"input"`
	Text   string `json:"text"`

	// Typed-workflow fields. All optional at the envelope level — required-ness
	// is enforced per workflow inside the dispatch handlers.
	AISystemID   string `json:"ai_system_id"`
	DocType      string `json:"doc_type"`
	Title        string `json:"title"`
	AuditJobID   string `json:"audit_job_id"`
	RepositoryID string `json:"repository_id"`
	Name         string `json:"name"`
	AuditType    string `json:"audit_type"`
	Query        string `json:"query"`
	TopK         int32  `json:"top_k"`
}

// Invoke handles agent invocation. The dispatch decision is on req.TaskType:
//
//   - "document_generation" / "generate_document" → DocumentGenerationWorkflow
//   - "compliance_audit"    / "trigger_audit"     → AuditWorkflow
//   - "knowledge_search"    / "search"            → SearchKnowledgeWorkflow
//   - everything else                              → LLM streaming completion
//
// Workflow dispatches return immediately with the workflow ID in the response
// metadata; SearchKnowledgeWorkflow additionally waits for the search result
// because it is short-lived and the caller usually wants the hits inline.
func (s *Supervisor) Invoke(ctx context.Context, req *agentv1.AgentRequest) (*agentv1.AgentResponse, error) {
	ctx = ensureTenant(ctx, req.GetTenantId())

	switch req.GetTaskType() {
	case "document_generation", "generate_document":
		return s.handleDocumentGenerationTask(ctx, req)
	case "compliance_audit", "trigger_audit":
		return s.handleAuditTask(ctx, req)
	case "knowledge_search", "search":
		return s.handleKnowledgeSearchTask(ctx, req)
	}

	return s.handleLLMCompletion(ctx, req)
}

// handleLLMCompletion performs a streaming LLM completion (the original
// behavior of Invoke). Tenant ID is propagated from request metadata so the
// LLM client can label inference metrics correctly.
func (s *Supervisor) handleLLMCompletion(ctx context.Context, req *agentv1.AgentRequest) (*agentv1.AgentResponse, error) {
	if s.llmClient == nil {
		return failedResponse(req, "llm client not configured"), nil
	}

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

// parseEnvelope decodes the AgentRequest.payload as a payloadEnvelope.
// Non-JSON payload is rejected for typed-task dispatch (unlike the LLM path)
// because typed workflows need named fields.
func parseEnvelope(payload []byte) (payloadEnvelope, error) {
	var env payloadEnvelope
	if len(payload) == 0 {
		return env, fmt.Errorf("payload is required")
	}
	if err := json.Unmarshal(payload, &env); err != nil {
		return env, fmt.Errorf("payload must be JSON: %w", err)
	}
	return env, nil
}

// handleDocumentGenerationTask starts a DocumentGenerationWorkflow and
// returns the workflow execution metadata so the caller can poll later.
func (s *Supervisor) handleDocumentGenerationTask(ctx context.Context, req *agentv1.AgentRequest) (*agentv1.AgentResponse, error) {
	if s.temporalClient == nil {
		return failedResponse(req, "temporal client not configured"), nil
	}
	env, err := parseEnvelope(req.GetPayload())
	if err != nil {
		return failedResponse(req, fmt.Sprintf("invalid payload: %v", err)), nil
	}
	if env.AISystemID == "" {
		return failedResponse(req, "ai_system_id is required for document_generation"), nil
	}
	if env.DocType == "" {
		return failedResponse(req, "doc_type is required for document_generation"), nil
	}

	input := DocumentGenerationWorkflowInput{
		AISystemID: env.AISystemID,
		DocType:    env.DocType,
		Title:      env.Title,
		AuditJobID: env.AuditJobID,
	}
	run, err := s.temporalClient.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:        workflowID("doc-gen", req.GetRequestId()),
		TaskQueue: OrchestratorTaskQueue,
	}, DocumentGenerationWorkflow, input)
	if err != nil {
		s.logger.Error("start document generation workflow",
			zap.String("request_id", req.GetRequestId()), zap.Error(err))
		return failedResponse(req, fmt.Sprintf("start document generation workflow: %v", err)), nil
	}

	return &agentv1.AgentResponse{
		RequestId: req.GetRequestId(),
		AgentId:   req.GetAgentId(),
		Status:    agentv1.AgentResponse_STATUS_IN_PROGRESS,
		Metadata: map[string]string{
			"workflow_id":  run.GetID(),
			"run_id":       run.GetRunID(),
			"task_queue":   OrchestratorTaskQueue,
			"workflow":     "DocumentGenerationWorkflow",
			"ai_system_id": input.AISystemID,
			"doc_type":     input.DocType,
		},
	}, nil
}

// handleAuditTask starts an AuditWorkflow and returns the workflow execution
// metadata so the caller can poll later.
func (s *Supervisor) handleAuditTask(ctx context.Context, req *agentv1.AgentRequest) (*agentv1.AgentResponse, error) {
	if s.temporalClient == nil {
		return failedResponse(req, "temporal client not configured"), nil
	}
	env, err := parseEnvelope(req.GetPayload())
	if err != nil {
		return failedResponse(req, fmt.Sprintf("invalid payload: %v", err)), nil
	}
	if env.RepositoryID == "" {
		return failedResponse(req, "repository_id is required for compliance_audit"), nil
	}

	input := AuditWorkflowInput{
		RepositoryID: env.RepositoryID,
		Name:         env.Name,
		AuditType:    env.AuditType,
	}
	run, err := s.temporalClient.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:        workflowID("audit", req.GetRequestId()),
		TaskQueue: OrchestratorTaskQueue,
	}, AuditWorkflow, input)
	if err != nil {
		s.logger.Error("start audit workflow",
			zap.String("request_id", req.GetRequestId()), zap.Error(err))
		return failedResponse(req, fmt.Sprintf("start audit workflow: %v", err)), nil
	}

	return &agentv1.AgentResponse{
		RequestId: req.GetRequestId(),
		AgentId:   req.GetAgentId(),
		Status:    agentv1.AgentResponse_STATUS_IN_PROGRESS,
		Metadata: map[string]string{
			"workflow_id":   run.GetID(),
			"run_id":        run.GetRunID(),
			"task_queue":    OrchestratorTaskQueue,
			"workflow":      "AuditWorkflow",
			"repository_id": input.RepositoryID,
			"audit_type":    input.AuditType,
		},
	}, nil
}

// handleKnowledgeSearchTask starts a SearchKnowledgeWorkflow and waits for
// the result inline — search is short-lived and callers typically want the
// hits in the response, not a workflow ID to poll.
func (s *Supervisor) handleKnowledgeSearchTask(ctx context.Context, req *agentv1.AgentRequest) (*agentv1.AgentResponse, error) {
	if s.temporalClient == nil {
		return failedResponse(req, "temporal client not configured"), nil
	}
	env, err := parseEnvelope(req.GetPayload())
	if err != nil {
		return failedResponse(req, fmt.Sprintf("invalid payload: %v", err)), nil
	}
	if env.Query == "" {
		return failedResponse(req, "query is required for knowledge_search"), nil
	}

	input := SearchKnowledgeWorkflowInput{Query: env.Query, TopK: env.TopK}
	run, err := s.temporalClient.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:        workflowID("search", req.GetRequestId()),
		TaskQueue: OrchestratorTaskQueue,
	}, SearchKnowledgeWorkflow, input)
	if err != nil {
		s.logger.Error("start search workflow",
			zap.String("request_id", req.GetRequestId()), zap.Error(err))
		return failedResponse(req, fmt.Sprintf("start search workflow: %v", err)), nil
	}

	var result SearchResult
	if err := run.Get(ctx, &result); err != nil {
		s.logger.Error("search workflow failed",
			zap.String("request_id", req.GetRequestId()),
			zap.String("workflow_id", run.GetID()), zap.Error(err))
		return failedResponse(req, fmt.Sprintf("search workflow failed: %v", err)), nil
	}

	body, err := json.Marshal(result)
	if err != nil {
		return failedResponse(req, fmt.Sprintf("encode search result: %v", err)), nil
	}
	return &agentv1.AgentResponse{
		RequestId: req.GetRequestId(),
		AgentId:   req.GetAgentId(),
		Status:    agentv1.AgentResponse_STATUS_COMPLETED,
		Result:    body,
		Metadata: map[string]string{
			"workflow_id": run.GetID(),
			"run_id":      run.GetRunID(),
			"task_queue":  OrchestratorTaskQueue,
			"workflow":    "SearchKnowledgeWorkflow",
		},
	}, nil
}

// workflowID derives a stable workflow ID for an AgentRequest. We prefer the
// caller-supplied request_id when present (gives reuse-by-ID idempotency),
// and fall back to a generated UUID otherwise. The prefix lets operators
// distinguish doc-gen / audit / search runs in the Temporal UI at a glance.
func workflowID(prefix, requestID string) string {
	if requestID != "" {
		return prefix + "-" + requestID
	}
	return prefix + "-" + uuid.NewString()
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
		Skills:      []string{"orchestration", "workflow", "compliance", "audit", "document_generation", "knowledge_search"},
		Version:     "1.0.0",
		Available:   true,
	}, nil
}

// TriggerDocumentGeneration starts a DocumentGenerationWorkflow and returns the
// underlying GenerateDocumentResponse-shaped payload alongside the workflow ID
// so callers can correlate later GetDocument calls. The RPC stays synchronous
// (it waits on .Get) because existing callers expect a populated response, but
// the workflow itself is durable so process restarts do not lose work.
func (s *Supervisor) TriggerDocumentGeneration(ctx context.Context, aiSystemID string, docType, title string) (*docv1.GenerateDocumentResponse, error) {
	if s.temporalClient == nil {
		return nil, fmt.Errorf("temporal client not configured")
	}
	if _, err := uuid.Parse(aiSystemID); err != nil {
		return nil, fmt.Errorf("invalid ai system id: %w", err)
	}
	ctx = ensureTenant(ctx, "")

	run, err := s.temporalClient.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:        workflowID("doc-gen", ""),
		TaskQueue: OrchestratorTaskQueue,
	}, DocumentGenerationWorkflow, DocumentGenerationWorkflowInput{
		AISystemID: aiSystemID,
		DocType:    docType,
		Title:      title,
	})
	if err != nil {
		return nil, fmt.Errorf("start document generation workflow: %w", err)
	}

	var output DocumentGenerationWorkflowOutput
	if err := run.Get(ctx, &output); err != nil {
		return nil, fmt.Errorf("document generation workflow: %w", err)
	}
	return &docv1.GenerateDocumentResponse{
		DocumentId: output.Document.DocumentID,
		Status:     mapDocumentStatusFromString(output.Document.Status),
	}, nil
}

// TriggerAudit starts an AuditWorkflow that itself calls audit-service. It
// returns the TriggerAuditResponse fields so the caller sees the audit ID and
// the audit-service workflow ID without needing to wait for the audit to
// finish — the orchestrator workflow continues polling in the background.
func (s *Supervisor) TriggerAudit(ctx context.Context, repositoryID string, name, auditType string) (*auditv1.TriggerAuditResponse, error) {
	if s.temporalClient == nil {
		return nil, fmt.Errorf("temporal client not configured")
	}
	if _, err := uuid.Parse(repositoryID); err != nil {
		return nil, fmt.Errorf("invalid repository id: %w", err)
	}
	ctx = ensureTenant(ctx, "")

	// Run the trigger activity directly so we can return as soon as audit-service
	// has accepted the request — same UX as the previous direct gRPC call. The
	// caller still gets a workflow execution they can correlate, via the audit
	// job's own workflow_id field.
	if s.clients == nil || s.clients.Audit == nil {
		return nil, fmt.Errorf("audit client not configured")
	}
	id, err := uuid.Parse(repositoryID)
	if err != nil {
		return nil, fmt.Errorf("invalid repository id: %w", err)
	}
	resp, err := s.clients.Audit.TriggerAudit(ctx, id, name, auditType)
	if err != nil {
		return nil, fmt.Errorf("trigger audit: %w", err)
	}

	// Fire-and-forget: kick off the orchestrator-side AuditWorkflow so the
	// audit progress is observable from the orchestrator's own Temporal
	// namespace. Failure to enqueue is non-fatal — the audit itself is
	// already running on audit-service.
	if _, wfErr := s.temporalClient.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:        workflowID("audit", resp.GetAuditId()),
		TaskQueue: OrchestratorTaskQueue,
	}, AuditWorkflow, AuditWorkflowInput{
		RepositoryID: repositoryID,
		Name:         name,
		AuditType:    auditType,
	}); wfErr != nil {
		s.logger.Warn("failed to start orchestrator audit workflow; audit-service still running",
			zap.String("audit_id", resp.GetAuditId()), zap.Error(wfErr))
	}

	return resp, nil
}

// SearchKnowledge starts a SearchKnowledgeWorkflow and returns the search
// hits inline. Search is short-lived so we wait on .Get and surface the
// results directly to the caller — same shape as the previous direct gRPC call.
func (s *Supervisor) SearchKnowledge(ctx context.Context, query string, topK int32) (*ragv1.SearchResponse, error) {
	if s.temporalClient == nil {
		return nil, fmt.Errorf("temporal client not configured")
	}
	ctx = ensureTenant(ctx, "")

	run, err := s.temporalClient.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:        workflowID("search", ""),
		TaskQueue: OrchestratorTaskQueue,
	}, SearchKnowledgeWorkflow, SearchKnowledgeWorkflowInput{
		Query: query,
		TopK:  topK,
	})
	if err != nil {
		return nil, fmt.Errorf("start search workflow: %w", err)
	}

	var result SearchResult
	if err := run.Get(ctx, &result); err != nil {
		return nil, fmt.Errorf("search workflow: %w", err)
	}

	out := &ragv1.SearchResponse{
		Query:   result.Query,
		TopK:    result.TopK,
		Results: make([]*ragv1.SearchResult, 0, len(result.Results)),
	}
	for _, h := range result.Results {
		out.Results = append(out.Results, &ragv1.SearchResult{
			DocumentId:   h.DocumentID,
			DocumentName: h.DocumentName,
			Text:         h.Text,
			Similarity:   h.Similarity,
		})
	}
	return out, nil
}

// mapDocumentStatusFromString turns the proto enum string ("DOCUMENT_STATUS_GENERATING")
// or our lowercase form ("generating") back into the docv1.DocumentStatus enum
// for callers that still expect a typed proto value.
func mapDocumentStatusFromString(s string) docv1.DocumentStatus {
	switch s {
	case docv1.DocumentStatus_DOCUMENT_STATUS_GENERATING.String(), "generating":
		return docv1.DocumentStatus_DOCUMENT_STATUS_GENERATING
	case docv1.DocumentStatus_DOCUMENT_STATUS_EDITING.String(), "editing":
		return docv1.DocumentStatus_DOCUMENT_STATUS_EDITING
	case docv1.DocumentStatus_DOCUMENT_STATUS_APPROVED.String(), "approved":
		return docv1.DocumentStatus_DOCUMENT_STATUS_APPROVED
	case docv1.DocumentStatus_DOCUMENT_STATUS_PUBLISHED.String(), "published":
		return docv1.DocumentStatus_DOCUMENT_STATUS_PUBLISHED
	case docv1.DocumentStatus_DOCUMENT_STATUS_FAILED.String(), "failed":
		return docv1.DocumentStatus_DOCUMENT_STATUS_FAILED
	default:
		return docv1.DocumentStatus_DOCUMENT_STATUS_UNSPECIFIED
	}
}
