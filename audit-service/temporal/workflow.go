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

	// Workflow steps with progress tracking.
	// The order here drives both the progress percentage and the `current_step`
	// field streamed to the client; steps[i] is the step that runs at iteration i.
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

	// runStep executes one workflow activity and writes step-level progress to the
	// DB on success. On failure it marks the audit failed (with the step name and
	// error message) and returns the original error so the workflow halts.
	runStep := func(stepIdx int, activity any, args ...any) error {
		stepName := steps[stepIdx]
		future := workflow.ExecuteActivity(ctx, activity, args...)
		if err := future.Get(ctx, nil); err != nil {
			logger.Error("Audit step failed", "step", stepName, "error", err)
			progressOnFail := (stepIdx * 100) / len(steps)
			_ = w.markFailed(ctx, params.AuditID, stepName, progressOnFail, err.Error())
			return err
		}
		// Step succeeded — write progress and the step name that just finished.
		percentage := ((stepIdx + 1) * 100) / len(steps)
		_ = w.updateStep(ctx, params.AuditID, model.AuditJobStatusRunning, stepName, percentage)
		return nil
	}

	currentStep := 0

	// Step: initialize — flips status to running and stamps started_at.
	if err := runStep(currentStep, a.InitializeAudit, params.AuditID); err != nil {
		return err
	}
	currentStep++

	// Step: load_previous_findings (incremental only).
	if isIncremental {
		if err := runStep(currentStep, a.LoadPreviousFindings, params.AuditID); err != nil {
			return err
		}
		currentStep++
	}

	// Step: fetch_repository.
	if err := runStep(currentStep, a.FetchRepository, params.AuditID); err != nil {
		return err
	}
	currentStep++

	// Step: run_static_analysis.
	if err := runStep(currentStep, a.RunStaticAnalysis, params.AuditID); err != nil {
		return err
	}
	currentStep++

	// Step: generate_findings.
	if err := runStep(currentStep, a.GenerateFindings, params.AuditID); err != nil {
		return err
	}
	currentStep++

	// Step: calculate_risk_score.
	if err := runStep(currentStep, a.CalculateRiskScore, params.AuditID); err != nil {
		return err
	}
	currentStep++

	// Step: check_approval_gate — also handles human-in-the-loop pause/resume.
	approvalStepIdx := currentStep
	approvalStepName := steps[approvalStepIdx]
	approvalProgress := ((approvalStepIdx + 1) * 100) / len(steps)

	var requiresApproval bool
	approvalFuture := workflow.ExecuteActivity(ctx, a.CheckApprovalGate, params.AuditID)
	if err := approvalFuture.Get(ctx, &requiresApproval); err != nil {
		logger.Error("Audit step failed", "step", approvalStepName, "error", err)
		_ = w.markFailed(ctx, params.AuditID, approvalStepName, (approvalStepIdx*100)/len(steps), err.Error())
		return err
	}

	if requiresApproval {
		// Create approval request so the frontend /approvals page can surface it.
		if err := workflow.ExecuteActivity(ctx, a.CreateApprovalRequest, params.AuditID).Get(ctx, nil); err != nil {
			logger.Error("Failed to create approval request", "error", err)
			// Non-fatal: continue to pause even if DB write fails.
		}

		// Pause for approval at the approval gate step.
		if err := w.updateStep(ctx, params.AuditID, model.AuditJobStatusPaused, approvalStepName, approvalProgress); err != nil {
			return err
		}

		// Wait for approval signal.
		signalChan := workflow.GetSignalChannel(ctx, "ApproveAuditSignal")
		var signal ApproveAuditSignal
		signalChan.Receive(ctx, &signal)

		if !signal.Approved {
			// Audit rejected — mark as cancelled.
			if cancelErr := workflow.ExecuteActivity(ctx, a.CancelAudit, params.AuditID).Get(ctx, nil); cancelErr != nil {
				logger.Error("Failed to cancel audit", "error", cancelErr)
			}
			return fmt.Errorf("audit rejected by approval")
		}

		// Resume after approval — back to running on the same step.
		if err := w.updateStep(ctx, params.AuditID, model.AuditJobStatusRunning, approvalStepName, approvalProgress); err != nil {
			return err
		}
	} else {
		// Approval not required — mark this step done and move on.
		_ = w.updateStep(ctx, params.AuditID, model.AuditJobStatusRunning, approvalStepName, approvalProgress)
	}
	currentStep++

	// Step: generate_report.
	if err := runStep(currentStep, a.GenerateReport, params.AuditID); err != nil {
		return err
	}
	currentStep++

	// Step: notify_completion. We don't fail the workflow if the notification
	// activity errors — but we still want the step name and 100% reflected, so
	// drive the step write directly instead of via runStep.
	notifyStepName := steps[currentStep]
	if err := workflow.ExecuteActivity(ctx, a.NotifyCompletion, params.AuditID).Get(ctx, nil); err != nil {
		logger.Warn("Failed to send completion notification", "error", err)
	}
	_ = w.updateStep(ctx, params.AuditID, model.AuditJobStatusRunning, notifyStepName, 100)

	// Mark as completed — flips status to completed and stamps completed_at.
	if err := workflow.ExecuteActivity(ctx, a.CompleteAudit, params.AuditID).Get(ctx, nil); err != nil {
		logger.Error("Failed to complete audit", "error", err)
		_ = w.markFailed(ctx, params.AuditID, notifyStepName, 100, err.Error())
		return err
	}

	logger.Info("Audit completed successfully", "audit_id", params.AuditID, "audit_type", params.AuditType)
	return nil
}

// updateStep writes status, current step name, and progress percentage in one call.
// Used on every successful workflow step transition.
func (w *ComplianceAuditWorkflow) updateStep(ctx workflow.Context, auditID, status, step string, progress int) error {
	activityCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	})
	return workflow.ExecuteActivity(activityCtx, a.UpdateAuditStep, auditID, status, step, progress).Get(ctx, nil)
}

// markFailed records the failure step + message so the streaming endpoint can
// surface a real error to the client. Best-effort: callers ignore the return.
func (w *ComplianceAuditWorkflow) markFailed(ctx workflow.Context, auditID, step string, progress int, errMsg string) error {
	activityCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	})
	return workflow.ExecuteActivity(activityCtx, a.MarkAuditFailed, auditID, step, progress, errMsg).Get(ctx, nil)
}

// a is the activity reference - kept for code completion
var a = &Activities{}
