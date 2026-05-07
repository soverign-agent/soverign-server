package orchestrator

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// OrchestratorTaskQueue is the Temporal task queue all orchestrator workflows
// and activities run on. The agent-orchestrator main starts a worker bound to
// this queue, and the supervisor uses the same constant when dispatching
// workflow executions so producer and consumer can never drift.
const OrchestratorTaskQueue = "orchestrator-task-queue"

// pollInterval is how long workflows wait between polls of a downstream
// long-running job (audit, document generation). Five seconds matches the
// audit-service status streaming cadence.
const pollInterval = 5 * time.Second

// pollTimeout caps the total time a workflow will keep polling before giving
// up. Compliance audits and full Annex IV generations can take several
// minutes, so 30 minutes leaves comfortable headroom.
const pollTimeout = 30 * time.Minute

// defaultActivityOptions are applied to every activity scheduled by an
// orchestrator workflow. Temporal requires StartToCloseTimeout (or
// ScheduleToCloseTimeout) on every invocation; without it the workflow task
// fails with BadScheduleActivityAttributes.
var defaultActivityOptions = workflow.ActivityOptions{
	StartToCloseTimeout: 2 * time.Minute,
}

// activities is a zero-value activity reference used for type-safe
// workflow.ExecuteActivity calls. The Temporal SDK registers the struct type
// (via worker.RegisterActivity), and the actual instance with real clients is
// created in main.go. Keeping a package-level variable avoids repeating
// struct-literal boilerplate inside every workflow function.
var activities = &Activities{}

// DocumentGenerationWorkflowInput is the input to DocumentGenerationWorkflow.
// Fields use json tags so Temporal's default data converter encodes them
// stably; AuditJobID is optional and links the generation to a specific
// audit job when called from inside an audit pipeline.
type DocumentGenerationWorkflowInput struct {
	AISystemID string `json:"ai_system_id"`
	DocType    string `json:"doc_type"`
	Title      string `json:"title"`
	AuditJobID string `json:"audit_job_id,omitempty"`
}

// DocumentGenerationWorkflowOutput is what the workflow returns once doc-service
// reports the document is no longer in the GENERATING state.
type DocumentGenerationWorkflowOutput struct {
	Document      DocumentResult `json:"document"`
	SearchResults SearchResult   `json:"search_results"`
}

// AuditWorkflowInput is the input to AuditWorkflow.
type AuditWorkflowInput struct {
	RepositoryID string `json:"repository_id"`
	Name         string `json:"name"`
	AuditType    string `json:"audit_type"`
}

// AuditWorkflowOutput is what the workflow returns once audit-service reports
// the audit has reached a terminal state.
type AuditWorkflowOutput struct {
	Trigger TriggerAuditResult `json:"trigger"`
	State   AuditState         `json:"state"`
}

// SearchKnowledgeWorkflowInput is the input to SearchKnowledgeWorkflow.
type SearchKnowledgeWorkflowInput struct {
	Query string `json:"query"`
	TopK  int32  `json:"top_k"`
}

// DocumentGenerationWorkflow orchestrates Annex IV / risk report generation:
// it first pulls compliance context out of the knowledge base, then asks
// doc-service to generate the document, then polls until generation finishes.
// Returning the final DocumentResult lets the caller display the produced
// document directly without a follow-up GetDocument call.
func DocumentGenerationWorkflow(ctx workflow.Context, input DocumentGenerationWorkflowInput) (DocumentGenerationWorkflowOutput, error) {
	logger := workflow.GetLogger(ctx)
	ctx = workflow.WithActivityOptions(ctx, defaultActivityOptions)

	if input.AISystemID == "" {
		return DocumentGenerationWorkflowOutput{}, temporal.NewNonRetryableApplicationError(
			"ai_system_id is required", "InvalidArgument", nil,
		)
	}
	if input.DocType == "" {
		return DocumentGenerationWorkflowOutput{}, temporal.NewNonRetryableApplicationError(
			"doc_type is required", "InvalidArgument", nil,
		)
	}

	// Step 1: pull compliance context from the knowledge base. Failures here
	// are non-fatal — generation can still proceed without RAG context — so we
	// log and continue with an empty SearchResult on error.
	var search SearchResult
	searchErr := workflow.ExecuteActivity(ctx,
		activities.SearchKnowledgeActivity,
		"AI system compliance documentation", int32(5),
	).Get(ctx, &search)
	if searchErr != nil {
		logger.Warn("search knowledge failed; continuing without RAG context", "error", searchErr)
	}

	// Step 2: ask doc-service to start generating the document.
	var generated GenerateDocumentResult
	if err := workflow.ExecuteActivity(ctx,
		activities.GenerateDocumentActivity,
		input.AISystemID, input.DocType, input.Title,
	).Get(ctx, &generated); err != nil {
		return DocumentGenerationWorkflowOutput{}, fmt.Errorf("generate document: %w", err)
	}

	// Step 3: poll until generation finishes. Bound the total wait by
	// pollTimeout so a stuck doc-service can't hold the workflow forever.
	deadline := workflow.Now(ctx).Add(pollTimeout)
	var doc DocumentResult
	for {
		if err := workflow.ExecuteActivity(ctx,
			activities.GetDocumentActivity,
			generated.DocumentID,
		).Get(ctx, &doc); err != nil {
			return DocumentGenerationWorkflowOutput{}, fmt.Errorf("get document: %w", err)
		}
		if !doc.IsGenerating() {
			break
		}
		if !workflow.Now(ctx).Before(deadline) {
			return DocumentGenerationWorkflowOutput{}, temporal.NewNonRetryableApplicationError(
				fmt.Sprintf("document generation timed out after %s", pollTimeout),
				"DeadlineExceeded", nil,
			)
		}
		if err := workflow.Sleep(ctx, pollInterval); err != nil {
			return DocumentGenerationWorkflowOutput{}, err
		}
	}

	return DocumentGenerationWorkflowOutput{Document: doc, SearchResults: search}, nil
}

// AuditWorkflow orchestrates a compliance audit: it triggers the audit on
// audit-service (which itself runs a Temporal workflow), then polls until
// audit-service reports a terminal state, and returns the final audit summary.
func AuditWorkflow(ctx workflow.Context, input AuditWorkflowInput) (AuditWorkflowOutput, error) {
	ctx = workflow.WithActivityOptions(ctx, defaultActivityOptions)

	if input.RepositoryID == "" {
		return AuditWorkflowOutput{}, temporal.NewNonRetryableApplicationError(
			"repository_id is required", "InvalidArgument", nil,
		)
	}

	// Step 1: kick off the downstream audit.
	var trigger TriggerAuditResult
	if err := workflow.ExecuteActivity(ctx,
		activities.TriggerAuditActivity,
		input.RepositoryID, input.Name, input.AuditType,
	).Get(ctx, &trigger); err != nil {
		return AuditWorkflowOutput{}, fmt.Errorf("trigger audit: %w", err)
	}

	// Step 2: poll the audit job until it terminates. We bound the wait by
	// pollTimeout for the same reasons as DocumentGenerationWorkflow.
	deadline := workflow.Now(ctx).Add(pollTimeout)
	var state AuditState
	for {
		if err := workflow.ExecuteActivity(ctx,
			activities.GetAuditActivity,
			trigger.AuditID,
		).Get(ctx, &state); err != nil {
			return AuditWorkflowOutput{}, fmt.Errorf("get audit: %w", err)
		}
		if state.IsTerminal() {
			break
		}
		if !workflow.Now(ctx).Before(deadline) {
			return AuditWorkflowOutput{}, temporal.NewNonRetryableApplicationError(
				fmt.Sprintf("audit polling timed out after %s", pollTimeout),
				"DeadlineExceeded", nil,
			)
		}
		if err := workflow.Sleep(ctx, pollInterval); err != nil {
			return AuditWorkflowOutput{}, err
		}
	}

	return AuditWorkflowOutput{Trigger: trigger, State: state}, nil
}

// SearchKnowledgeWorkflow runs a single SearchKnowledgeActivity. It exists as
// a workflow (rather than just an activity exposed directly) so callers get a
// queryable workflow execution they can correlate with audits and document
// generations in the Temporal UI.
func SearchKnowledgeWorkflow(ctx workflow.Context, input SearchKnowledgeWorkflowInput) (SearchResult, error) {
	ctx = workflow.WithActivityOptions(ctx, defaultActivityOptions)

	if input.Query == "" {
		return SearchResult{}, temporal.NewNonRetryableApplicationError(
			"query is required", "InvalidArgument", nil,
		)
	}
	topK := input.TopK
	if topK <= 0 {
		topK = 5
	}

	var result SearchResult
	if err := workflow.ExecuteActivity(ctx,
		activities.SearchKnowledgeActivity,
		input.Query, topK,
	).Get(ctx, &result); err != nil {
		return SearchResult{}, fmt.Errorf("search knowledge: %w", err)
	}
	return result, nil
}
