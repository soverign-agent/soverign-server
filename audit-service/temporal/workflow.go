package temporal

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/workflow"
	"sovereign-ai-compliance/audit-service/model"
)

// ComplianceAuditWorkflowParams parameters for the compliance audit workflow.
type ComplianceAuditWorkflowParams struct {
	AuditID   string `json:"audit_id"`
	AuditType string `json:"audit_type"`
}

// ApproveAuditSignal signal for approval when paused at approval gate.
type ApproveAuditSignal struct {
	Approved bool `json:"approved"`
}

// GetAuditStatusQuery query to get current audit status.
type GetAuditStatusQuery struct{}

// GetAuditStatusResult result of the status query.
type GetAuditStatusResult struct {
	Status      string `json:"status"`
	Progress    int    `json:"progress"`
	CurrentStep string `json:"current_step"`
}

// ComplianceAuditWorkflow coordinates the end-to-end compliance audit process.
type ComplianceAuditWorkflow struct {
}

// defaultActivityOptions are applied to every activity scheduled by the
// compliance audit workflow. Temporal requires StartToCloseTimeout (or
// ScheduleToCloseTimeout) on every activity invocation; without it the
// workflow task fails with BadScheduleActivityAttributes.
var defaultActivityOptions = workflow.ActivityOptions{
	StartToCloseTimeout:    10 * time.Minute,
	ScheduleToCloseTimeout: 30 * time.Minute,
	HeartbeatTimeout:       1 * time.Minute,
}

// Execute runs the compliance audit workflow.
func (w *ComplianceAuditWorkflow) Execute(ctx workflow.Context, params ComplianceAuditWorkflowParams) error {
	logger := workflow.GetLogger(ctx)

	// Apply default activity options to the entire workflow context so every
	// downstream ExecuteActivity call inherits valid timeouts.
	ctx = workflow.WithActivityOptions(ctx, defaultActivityOptions)

	isIncremental := params.AuditType == model.AuditTypeIncremental

	// Workflow steps with progress tracking
	var steps []string
	if isIncremental {
		steps = []string{
			"initialize",
			"load_previous_findings",
			"fetch_repository",
			"run_static_analysis",
			"generate_findings",
			"calculate_risk_score",
			"check_approval_gate",
			"generate_report",
			"notify_completion",
		}
	} else {
		steps = []string{
			"initialize",
			"fetch_repository",
			"run_static_analysis",
			"generate_findings",
			"calculate_risk_score",
			"check_approval_gate",
			"generate_report",
			"notify_completion",
		}
	}

	currentStep := 0
	progress := 0

	// Update audit status to running
	err := workflow.ExecuteActivity(ctx, a.InitializeAudit, params.AuditID).Get(ctx, nil)
	if err != nil {
		logger.Error("Failed to initialize audit", "error", err)
		return err
	}
	currentStep++
	progress = (currentStep * 100) / len(steps)

	// For incremental audits, carry forward findings from the previous completed audit
	if isIncremental {
		err = workflow.ExecuteActivity(ctx, a.LoadPreviousFindings, params.AuditID).Get(ctx, nil)
		if err != nil {
			logger.Error("Failed to load previous findings", "error", err)
			_ = w.updateStatus(ctx, params.AuditID, model.AuditJobStatusFailed, progress, steps[currentStep-1])
			return err
		}
		currentStep++
		progress = (currentStep * 100) / len(steps)
	}

	// Fetch repository from repo-service
	err = workflow.ExecuteActivity(ctx, a.FetchRepository, params.AuditID).Get(ctx, nil)
	if err != nil {
		logger.Error("Failed to fetch repository", "error", err)
		_ = w.updateStatus(ctx, params.AuditID, model.AuditJobStatusFailed, progress, steps[currentStep-1])
		return err
	}
	currentStep++
	progress = (currentStep * 100) / len(steps)

	// Run static analysis on repository
	err = workflow.ExecuteActivity(ctx, a.RunStaticAnalysis, params.AuditID).Get(ctx, nil)
	if err != nil {
		logger.Error("Failed to run static analysis", "error", err)
		_ = w.updateStatus(ctx, params.AuditID, model.AuditJobStatusFailed, progress, steps[currentStep-1])
		return err
	}
	currentStep++
	progress = (currentStep * 100) / len(steps)

	// Generate findings from analysis results
	err = workflow.ExecuteActivity(ctx, a.GenerateFindings, params.AuditID).Get(ctx, nil)
	if err != nil {
		logger.Error("Failed to generate findings", "error", err)
		_ = w.updateStatus(ctx, params.AuditID, model.AuditJobStatusFailed, progress, steps[currentStep-1])
		return err
	}
	currentStep++
	progress = (currentStep * 100) / len(steps)

	// Calculate risk score
	err = workflow.ExecuteActivity(ctx, a.CalculateRiskScore, params.AuditID).Get(ctx, nil)
	if err != nil {
		logger.Error("Failed to calculate risk score", "error", err)
		_ = w.updateStatus(ctx, params.AuditID, model.AuditJobStatusFailed, progress, steps[currentStep-1])
		return err
	}
	currentStep++
	progress = (currentStep * 100) / len(steps)

	// Check if approval is required
	var requiresApproval bool
	err = workflow.ExecuteActivity(ctx, a.CheckApprovalGate, params.AuditID).Get(ctx, &requiresApproval)
	if err != nil {
		logger.Error("Failed to check approval gate", "error", err)
		_ = w.updateStatus(ctx, params.AuditID, model.AuditJobStatusFailed, progress, steps[currentStep-1])
		return err
	}

	if requiresApproval {
		// Pause for approval
		err = w.updateStatus(ctx, params.AuditID, model.AuditJobStatusPaused, progress, steps[currentStep])
		if err != nil {
			return err
		}

		// Wait for approval signal
		signalChan := workflow.GetSignalChannel(ctx, "ApproveAuditSignal")
		var signal ApproveAuditSignal
		signalChan.Receive(ctx, &signal)

		if !signal.Approved {
			// Audit rejected - mark as cancelled
			err = workflow.ExecuteActivity(ctx, a.CancelAudit, params.AuditID).Get(ctx, nil)
			if err != nil {
				logger.Error("Failed to cancel audit", "error", err)
			}
			return fmt.Errorf("audit rejected by approval")
		}

		// Resume after approval
		err = w.updateStatus(ctx, params.AuditID, model.AuditJobStatusRunning, progress, steps[currentStep])
		if err != nil {
			return err
		}
	}
	currentStep++
	progress = (currentStep * 100) / len(steps)

	// Generate final report
	err = workflow.ExecuteActivity(ctx, a.GenerateReport, params.AuditID).Get(ctx, nil)
	if err != nil {
		logger.Error("Failed to generate report", "error", err)
		_ = w.updateStatus(ctx, params.AuditID, model.AuditJobStatusFailed, progress, steps[currentStep-1])
		return err
	}
	currentStep++
	progress = (currentStep * 100) / len(steps)

	// Send completion notification
	err = workflow.ExecuteActivity(ctx, a.NotifyCompletion, params.AuditID).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to send completion notification", "error", err)
		// Don't fail the workflow for notification failure
	}
	currentStep++
	progress = 100

	// Mark as completed
	err = workflow.ExecuteActivity(ctx, a.CompleteAudit, params.AuditID).Get(ctx, nil)
	if err != nil {
		logger.Error("Failed to complete audit", "error", err)
		return err
	}

	logger.Info("Audit completed successfully", "audit_id", params.AuditID, "audit_type", params.AuditType)
	return nil
}

func (w *ComplianceAuditWorkflow) updateStatus(ctx workflow.Context, auditID string, status string, progress int, step string) error {
	activityCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	})
	return workflow.ExecuteActivity(activityCtx, a.UpdateAuditStatus, auditID, status, progress, step).Get(ctx, nil)
}

// a is the activity reference - kept for code completion
var a = &Activities{}
