package tests

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"sovereign-ai-compliance/agent-orchestrator/internal/orchestrator"
)

// TestDocumentGenerationWorkflow_HappyPath verifies the happy-path flow:
// search → generate → poll once → complete with the final document.
func TestDocumentGenerationWorkflow_HappyPath(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	acts := &orchestrator.Activities{}
	env.RegisterActivity(acts)

	searchResult := orchestrator.SearchResult{
		Query:   "AI system compliance documentation",
		TopK:    5,
		Results: []orchestrator.SearchHit{{DocumentID: "doc-1", Similarity: 0.9}},
	}
	generateResult := orchestrator.GenerateDocumentResult{
		DocumentID: "generated-doc-1",
		Status:     "DOCUMENT_STATUS_GENERATING",
	}
	generatingDoc := orchestrator.DocumentResult{
		DocumentID: "generated-doc-1",
		Status:     "DOCUMENT_STATUS_GENERATING",
		Sections:   []orchestrator.DocumentSection{},
	}
	finalDoc := orchestrator.DocumentResult{
		DocumentID: "generated-doc-1",
		Status:     "DOCUMENT_STATUS_APPROVED",
		Title:      "Annex IV Report",
		Sections: []orchestrator.DocumentSection{
			{ID: "sec-1", Title: "Purpose", Content: "To comply.", Order: 1},
		},
	}

	env.OnActivity(
		acts.SearchKnowledgeActivity,
		mock.Anything, "AI system compliance documentation", int32(5),
	).Return(searchResult, nil)
	env.OnActivity(
		acts.GenerateDocumentActivity,
		mock.Anything, "sys-1", "annex_iv", "My Title",
	).Return(generateResult, nil)
	// First GetDocument call still GENERATING → workflow sleeps and polls again.
	env.OnActivity(
		acts.GetDocumentActivity,
		mock.Anything, "generated-doc-1",
	).Return(generatingDoc, nil).Once()
	// Second GetDocument call reports done.
	env.OnActivity(
		acts.GetDocumentActivity,
		mock.Anything, "generated-doc-1",
	).Return(finalDoc, nil).Once()

	env.ExecuteWorkflow(orchestrator.DocumentGenerationWorkflow, orchestrator.DocumentGenerationWorkflowInput{
		AISystemID: "sys-1",
		DocType:    "annex_iv",
		Title:      "My Title",
	})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result orchestrator.DocumentGenerationWorkflowOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, "generated-doc-1", result.Document.DocumentID)
	assert.Equal(t, "DOCUMENT_STATUS_APPROVED", result.Document.Status)
	assert.Equal(t, "Annex IV Report", result.Document.Title)
	assert.Len(t, result.Document.Sections, 1)
	assert.Equal(t, "doc-1", result.SearchResults.Results[0].DocumentID)
}

// TestDocumentGenerationWorkflow_SearchOptional verifies that a failure from
// SearchKnowledgeActivity is non-fatal and the workflow proceeds to generation.
func TestDocumentGenerationWorkflow_SearchOptional(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	acts := &orchestrator.Activities{}
	env.RegisterActivity(acts)

	generateResult := orchestrator.GenerateDocumentResult{DocumentID: "doc-2", Status: "generating"}
	finalDoc := orchestrator.DocumentResult{DocumentID: "doc-2", Status: "published", Title: "R"}

	env.OnActivity(
		acts.SearchKnowledgeActivity,
		mock.Anything, "AI system compliance documentation", int32(5),
	).Return(orchestrator.SearchResult{}, errors.New("rag down"))
	env.OnActivity(
		acts.GenerateDocumentActivity,
		mock.Anything, "sys-2", "risk_report", "",
	).Return(generateResult, nil)
	env.OnActivity(
		acts.GetDocumentActivity,
		mock.Anything, "doc-2",
	).Return(finalDoc, nil).Once()

	env.ExecuteWorkflow(orchestrator.DocumentGenerationWorkflow, orchestrator.DocumentGenerationWorkflowInput{
		AISystemID: "sys-2",
		DocType:    "risk_report",
		Title:      "",
	})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var result orchestrator.DocumentGenerationWorkflowOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, "doc-2", result.Document.DocumentID)
	assert.Empty(t, result.SearchResults.Results)
}

// TestDocumentGenerationWorkflow_InvalidInput verifies that the workflow
// fails fast with a non-retryable error when required fields are missing.
func TestDocumentGenerationWorkflow_InvalidInput(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	env.ExecuteWorkflow(orchestrator.DocumentGenerationWorkflow, orchestrator.DocumentGenerationWorkflowInput{
		AISystemID: "",
		DocType:    "annex_iv",
	})

	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())
	assert.Contains(t, env.GetWorkflowError().Error(), "ai_system_id is required")
}

// TestAuditWorkflow_HappyPath verifies trigger → poll → terminal state.
func TestAuditWorkflow_HappyPath(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	acts := &orchestrator.Activities{}
	env.RegisterActivity(acts)

	triggerResult := orchestrator.TriggerAuditResult{
		AuditID:    "audit-99",
		Status:     "AUDIT_JOB_STATUS_PENDING",
		WorkflowID: "wf-99",
	}
	runningState := orchestrator.AuditState{
		AuditID:  "audit-99",
		Status:   "running",
		Progress: 50,
	}
	finalState := orchestrator.AuditState{
		AuditID:       "audit-99",
		Status:        "completed",
		Progress:      100,
		RiskScore:     88,
		RiskSeverity:  "RISK_SEVERITY_HIGH",
		FindingsCount: 12,
		CurrentStep:   "notify_completion",
	}

	env.OnActivity(
		acts.TriggerAuditActivity,
		mock.Anything, "repo-7", "My Audit", "full",
	).Return(triggerResult, nil)
	env.OnActivity(
		acts.GetAuditActivity,
		mock.Anything, "audit-99",
	).Return(runningState, nil).Once()
	env.OnActivity(
		acts.GetAuditActivity,
		mock.Anything, "audit-99",
	).Return(finalState, nil).Once()

	env.ExecuteWorkflow(orchestrator.AuditWorkflow, orchestrator.AuditWorkflowInput{
		RepositoryID: "repo-7",
		Name:         "My Audit",
		AuditType:    "full",
	})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result orchestrator.AuditWorkflowOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, "audit-99", result.Trigger.AuditID)
	assert.Equal(t, "completed", result.State.Status)
	assert.Equal(t, int32(100), result.State.Progress)
	assert.True(t, result.State.IsTerminal())
}

// TestAuditWorkflow_InvalidInput verifies fast-fail on missing repository_id.
func TestAuditWorkflow_InvalidInput(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	env.ExecuteWorkflow(orchestrator.AuditWorkflow, orchestrator.AuditWorkflowInput{})

	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())
	assert.Contains(t, env.GetWorkflowError().Error(), "repository_id is required")
}

// TestSearchKnowledgeWorkflow_HappyPath verifies the single-activity workflow.
func TestSearchKnowledgeWorkflow_HappyPath(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	acts := &orchestrator.Activities{}
	env.RegisterActivity(acts)

	searchResult := orchestrator.SearchResult{
		Query: "EU AI Act transparency",
		TopK:  10,
		Results: []orchestrator.SearchHit{
			{DocumentID: "hit-1", Similarity: 0.91},
		},
	}

	env.OnActivity(
		acts.SearchKnowledgeActivity,
		mock.Anything, "EU AI Act transparency", int32(10),
	).Return(searchResult, nil)

	env.ExecuteWorkflow(orchestrator.SearchKnowledgeWorkflow, orchestrator.SearchKnowledgeWorkflowInput{
		Query: "EU AI Act transparency",
		TopK:  10,
	})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result orchestrator.SearchResult
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, "EU AI Act transparency", result.Query)
	assert.Equal(t, int32(10), result.TopK)
	assert.Len(t, result.Results, 1)
}

// TestSearchKnowledgeWorkflow_DefaultTopK verifies that zero/negative TopK
// is clamped to the default of 5 before the activity is invoked.
func TestSearchKnowledgeWorkflow_DefaultTopK(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	acts := &orchestrator.Activities{}
	env.RegisterActivity(acts)

	env.OnActivity(
		acts.SearchKnowledgeActivity,
		mock.Anything, "hello", int32(5),
	).Return(orchestrator.SearchResult{Query: "hello", TopK: 5}, nil)

	env.ExecuteWorkflow(orchestrator.SearchKnowledgeWorkflow, orchestrator.SearchKnowledgeWorkflowInput{
		Query: "hello",
		TopK:  0,
	})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
}

// TestSearchKnowledgeWorkflow_InvalidInput verifies fast-fail on empty query.
func TestSearchKnowledgeWorkflow_InvalidInput(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	env.ExecuteWorkflow(orchestrator.SearchKnowledgeWorkflow, orchestrator.SearchKnowledgeWorkflowInput{})

	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())
	assert.Contains(t, env.GetWorkflowError().Error(), "query is required")
}

// TestDocumentGenerationWorkflow_PollTimeout verifies that the workflow
// returns a non-retryable error when the document never leaves the GENERATING
// state before the polling deadline.
func TestDocumentGenerationWorkflow_PollTimeout(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	acts := &orchestrator.Activities{}
	env.RegisterActivity(acts)

	generateResult := orchestrator.GenerateDocumentResult{DocumentID: "doc-3", Status: "generating"}
	generatingDoc := orchestrator.DocumentResult{DocumentID: "doc-3", Status: "DOCUMENT_STATUS_GENERATING"}

	env.OnActivity(
		acts.SearchKnowledgeActivity,
		mock.Anything, "AI system compliance documentation", int32(5),
	).Return(orchestrator.SearchResult{}, nil)
	env.OnActivity(
		acts.GenerateDocumentActivity,
		mock.Anything, "sys-3", "annex_iv", "T",
	).Return(generateResult, nil)
	// Return GENERATING for a large number of iterations so the poll loop
	// keeps spinning until the 30-minute deadline is reached. In the test
	// environment workflow.Sleep advances the simulated clock, so after
	// enough 5-second sleeps the deadline check triggers.
	env.OnActivity(
		acts.GetDocumentActivity,
		mock.Anything, "doc-3",
	).Return(generatingDoc, nil).Maybe()

	env.SetWorkflowRunTimeout(35 * time.Minute)

	env.ExecuteWorkflow(orchestrator.DocumentGenerationWorkflow, orchestrator.DocumentGenerationWorkflowInput{
		AISystemID: "sys-3",
		DocType:    "annex_iv",
		Title:      "T",
	})

	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())
	assert.Contains(t, env.GetWorkflowError().Error(), "timed out")
}
