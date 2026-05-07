package orchestrator

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"sovereign-ai-compliance/agent-orchestrator/internal/client"
)

// SearchHit is a serialization-friendly variant of ragv1.SearchResult.
// Workflow / activity boundaries serialize values to JSON via the default
// Temporal data converter; using a plain Go struct (instead of the protobuf
// generated type) avoids brittle reflection on unexported proto state.
type SearchHit struct {
	DocumentID   string  `json:"document_id"`
	DocumentName string  `json:"document_name"`
	Text         string  `json:"text"`
	Similarity   float64 `json:"similarity"`
}

// SearchResult is what SearchKnowledgeActivity / SearchKnowledgeWorkflow
// return. It mirrors ragv1.SearchResponse but keeps only the fields the
// orchestrator surfaces back to the caller.
type SearchResult struct {
	Query   string      `json:"query"`
	TopK    int32       `json:"top_k"`
	Results []SearchHit `json:"results"`
}

// TriggerAuditResult is what TriggerAuditActivity returns — the audit ID and
// the workflow ID started inside audit-service so the orchestrator caller can
// query downstream state.
type TriggerAuditResult struct {
	AuditID    string `json:"audit_id"`
	Status     string `json:"status"`
	WorkflowID string `json:"workflow_id"`
}

// AuditState captures the fields AuditWorkflow polls on. It is a flat
// projection of auditv1.AuditJob — enough for the workflow to decide whether
// the audit has reached a terminal state, and enough for callers to render a
// summary.
type AuditState struct {
	AuditID       string `json:"audit_id"`
	WorkflowID    string `json:"workflow_id"`
	Status        string `json:"status"`
	Progress      int32  `json:"progress_percentage"`
	RiskScore     int32  `json:"risk_score"`
	RiskSeverity  string `json:"risk_severity"`
	FindingsCount int32  `json:"findings_count"`
	CurrentStep   string `json:"current_step"`
	ErrorMessage  string `json:"error_message"`
}

// IsTerminal reports whether the audit has reached a status that ends polling.
func (a AuditState) IsTerminal() bool {
	switch a.Status {
	case "completed", "failed", "cancelled":
		return true
	default:
		return false
	}
}

// GenerateDocumentResult is what GenerateDocumentActivity returns — the new
// document ID plus the initial generation status.
type GenerateDocumentResult struct {
	DocumentID string `json:"document_id"`
	Status     string `json:"status"`
}

// DocumentSubSection mirrors docv1.SubSection in a serialization-friendly
// form.
type DocumentSubSection struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Order   int32  `json:"order"`
}

// DocumentSection mirrors docv1.Section in a serialization-friendly form.
type DocumentSection struct {
	ID          string               `json:"id"`
	Title       string               `json:"title"`
	Content     string               `json:"content"`
	Order       int32                `json:"order"`
	SubSections []DocumentSubSection `json:"subsections,omitempty"`
}

// DocumentResult is what GetDocumentActivity returns — a flattened view of
// the generated compliance document.
type DocumentResult struct {
	DocumentID string            `json:"document_id"`
	AISystemID string            `json:"ai_system_id"`
	DocType    string            `json:"doc_type"`
	Title      string            `json:"title"`
	Status     string            `json:"status"`
	Version    int32             `json:"version"`
	Sections   []DocumentSection `json:"sections,omitempty"`
}

// IsGenerating reports whether the document is still being generated; the
// DocumentGenerationWorkflow uses this to decide whether to poll again.
func (d DocumentResult) IsGenerating() bool {
	return d.Status == "generating" || d.Status == "DOCUMENT_STATUS_GENERATING"
}

// Activities holds dependencies needed by Temporal activities. Each method
// is registered with the worker and may be invoked via workflow.ExecuteActivity.
//
// Activities receive their tenant context via the shared tenant propagator
// (registered on the Temporal client). The downstream gRPC client wrappers
// then pull the tenant ID off the Go context and forward it as gRPC metadata,
// so RLS-protected queries on the called services authenticate correctly.
type Activities struct {
	clients *client.Clients
}

// NewActivities constructs an Activities instance bound to the supplied
// downstream gRPC clients.
func NewActivities(clients *client.Clients) *Activities {
	return &Activities{clients: clients}
}

// SearchKnowledgeActivity performs a vector similarity search against the
// rag-service knowledge base.
func (a *Activities) SearchKnowledgeActivity(ctx context.Context, query string, topK int32) (SearchResult, error) {
	if a.clients == nil || a.clients.RAG == nil {
		return SearchResult{}, fmt.Errorf("rag client not configured")
	}
	resp, err := a.clients.RAG.Search(ctx, query, topK)
	if err != nil {
		return SearchResult{}, fmt.Errorf("rag search: %w", err)
	}
	out := SearchResult{
		Query:   resp.GetQuery(),
		TopK:    resp.GetTopK(),
		Results: make([]SearchHit, 0, len(resp.GetResults())),
	}
	for _, r := range resp.GetResults() {
		out.Results = append(out.Results, SearchHit{
			DocumentID:   r.GetDocumentId(),
			DocumentName: r.GetDocumentName(),
			Text:         r.GetText(),
			Similarity:   r.GetSimilarity(),
		})
	}
	return out, nil
}

// TriggerAuditActivity starts a compliance audit workflow inside audit-service
// and returns the new audit's ID and workflow ID. It does NOT wait for
// completion — that is the responsibility of GetAuditActivity.
func (a *Activities) TriggerAuditActivity(ctx context.Context, repositoryID, name, auditType string) (TriggerAuditResult, error) {
	if a.clients == nil || a.clients.Audit == nil {
		return TriggerAuditResult{}, fmt.Errorf("audit client not configured")
	}
	id, err := uuid.Parse(repositoryID)
	if err != nil {
		return TriggerAuditResult{}, fmt.Errorf("invalid repository id: %w", err)
	}
	resp, err := a.clients.Audit.TriggerAudit(ctx, id, name, auditType)
	if err != nil {
		return TriggerAuditResult{}, fmt.Errorf("trigger audit: %w", err)
	}
	return TriggerAuditResult{
		AuditID:    resp.GetAuditId(),
		Status:     resp.GetStatus().String(),
		WorkflowID: resp.GetWorkflowId(),
	}, nil
}

// GetAuditActivity fetches the current state of an audit job. AuditWorkflow
// calls this on a polling loop until AuditState.IsTerminal() reports done.
func (a *Activities) GetAuditActivity(ctx context.Context, auditID string) (AuditState, error) {
	if a.clients == nil || a.clients.Audit == nil {
		return AuditState{}, fmt.Errorf("audit client not configured")
	}
	id, err := uuid.Parse(auditID)
	if err != nil {
		return AuditState{}, fmt.Errorf("invalid audit id: %w", err)
	}
	resp, err := a.clients.Audit.GetAudit(ctx, id)
	if err != nil {
		return AuditState{}, fmt.Errorf("get audit: %w", err)
	}
	job := resp.GetAudit()
	if job == nil {
		return AuditState{}, fmt.Errorf("audit %s not found", auditID)
	}
	return AuditState{
		AuditID:       job.GetId(),
		WorkflowID:    job.GetWorkflowId(),
		Status:        job.GetStatus(),
		Progress:      job.GetProgressPercentage(),
		RiskScore:     job.GetRiskScore(),
		RiskSeverity:  job.GetRiskSeverity().String(),
		FindingsCount: job.GetFindingsCount(),
		CurrentStep:   job.GetCurrentStep(),
		ErrorMessage:  job.GetErrorMessage(),
	}, nil
}

// GenerateDocumentActivity asks doc-service to start generating a compliance
// document for an AI system. Generation is async — the workflow polls via
// GetDocumentActivity afterwards.
func (a *Activities) GenerateDocumentActivity(ctx context.Context, aiSystemID, docType, title string) (GenerateDocumentResult, error) {
	if a.clients == nil || a.clients.Doc == nil {
		return GenerateDocumentResult{}, fmt.Errorf("doc client not configured")
	}
	id, err := uuid.Parse(aiSystemID)
	if err != nil {
		return GenerateDocumentResult{}, fmt.Errorf("invalid ai system id: %w", err)
	}
	resp, err := a.clients.Doc.GenerateDocument(ctx, id, docType, title)
	if err != nil {
		return GenerateDocumentResult{}, fmt.Errorf("generate document: %w", err)
	}
	return GenerateDocumentResult{
		DocumentID: resp.GetDocumentId(),
		Status:     resp.GetStatus().String(),
	}, nil
}

// GetDocumentActivity fetches a document by ID. DocumentGenerationWorkflow
// uses this in its polling loop and to return the final document.
func (a *Activities) GetDocumentActivity(ctx context.Context, docID string) (DocumentResult, error) {
	if a.clients == nil || a.clients.Doc == nil {
		return DocumentResult{}, fmt.Errorf("doc client not configured")
	}
	id, err := uuid.Parse(docID)
	if err != nil {
		return DocumentResult{}, fmt.Errorf("invalid document id: %w", err)
	}
	resp, err := a.clients.Doc.GetDocument(ctx, id)
	if err != nil {
		return DocumentResult{}, fmt.Errorf("get document: %w", err)
	}
	doc := resp.GetDocument()
	if doc == nil {
		return DocumentResult{}, fmt.Errorf("document %s not found", docID)
	}
	out := DocumentResult{
		DocumentID: doc.GetId(),
		AISystemID: doc.GetAiSystemId(),
		DocType:    doc.GetDocType().String(),
		Title:      doc.GetTitle(),
		Status:     doc.GetStatus().String(),
		Version:    doc.GetVersion(),
	}
	if content := doc.GetContent(); content != nil {
		for _, s := range content.GetSections() {
			section := DocumentSection{
				ID:      s.GetId(),
				Title:   s.GetTitle(),
				Content: s.GetContent(),
				Order:   s.GetOrder(),
			}
			for _, sub := range s.GetSubsections() {
				section.SubSections = append(section.SubSections, DocumentSubSection{
					ID:      sub.GetId(),
					Title:   sub.GetTitle(),
					Content: sub.GetContent(),
					Order:   sub.GetOrder(),
				})
			}
			out.Sections = append(out.Sections, section)
		}
	}
	return out, nil
}
