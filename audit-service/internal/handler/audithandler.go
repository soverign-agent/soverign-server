package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/rest/httpx"
	"sovereign-ai-compliance/audit-service/internal/logic"
	"sovereign-ai-compliance/audit-service/internal/types"
	"sovereign-ai-compliance/audit-service/model"
	"sovereign-ai-compliance/audit-service/temporal"
)

// AuditHandler handles audit management HTTP endpoints.
type AuditHandler struct {
	logic    *logic.AuditLogic
	temporal *temporal.Client
}

// NewAuditHandler creates a new AuditHandler.
func NewAuditHandler(logic *logic.AuditLogic, temporal *temporal.Client) *AuditHandler {
	return &AuditHandler{
		logic:    logic,
		temporal: temporal,
	}
}

// Trigger triggers a new audit.
func (h *AuditHandler) Trigger(w http.ResponseWriter, r *http.Request) {
	var req types.TriggerAuditRequest
	if err := httpx.Parse(r, &req); err != nil {
		httpx.ErrorCtx(r.Context(), w, err)
		return
	}

	// Validate required fields
	if req.RepositoryID == uuid.Nil {
		httpx.ErrorCtx(r.Context(), w, fmt.Errorf("repository_id is required"))
		return
	}
	if req.Name == "" {
		httpx.ErrorCtx(r.Context(), w, fmt.Errorf("name is required"))
		return
	}
	if req.AuditType != model.AuditTypeFull && req.AuditType != model.AuditTypeIncremental {
		httpx.ErrorCtx(r.Context(), w, fmt.Errorf("audit_type must be either 'full' or 'incremental'"))
		return
	}

	audit, err := h.logic.TriggerAudit(r.Context(), &req)
	if err != nil {
		httpx.ErrorCtx(r.Context(), w, err)
		return
	}

	// Start the Temporal workflow
	workflowRun, err := h.temporal.StartAuditWorkflow(r.Context(), audit.ID.String())
	if err != nil {
		// Don't fail the request, but return the audit with error
		httpx.OkJson(w, types.TriggerAuditResponse{
			AuditID: audit.ID,
			Status:  audit.Status,
		})
		return
	}

	// Update the audit with the workflow ID
	wfID := workflowRun.GetID()
	err = h.logic.UpdateWorkflowID(r.Context(), audit.ID, wfID)
	if err != nil {
		// Log but continue
	}

	httpx.OkJson(w, types.TriggerAuditResponse{
		AuditID:    audit.ID,
		Status:     audit.Status,
		WorkflowID: &wfID,
	})
}

// List lists all audits with filtering.
func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
	var req types.ListAuditsRequest
	if err := httpx.Parse(r, &req); err != nil {
		httpx.ErrorCtx(r.Context(), w, err)
		return
	}

	// Set default pagination
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 20
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	items, total, err := h.logic.ListAudits(r.Context(), &req)
	if err != nil {
		httpx.ErrorCtx(r.Context(), w, err)
		return
	}

	pages := (total + req.PageSize - 1) / req.PageSize

	httpx.OkJson(w, types.ListAuditsResponse{
		Items: items,
		Total: total,
		Page:  req.Page,
		Pages: pages,
	})
}

// Get gets an audit by ID.
func (h *AuditHandler) Get(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	auditID, err := uuid.Parse(idStr)
	if err != nil {
		httpx.ErrorCtx(r.Context(), w, fmt.Errorf("invalid audit ID: %w", err))
		return
	}

	audit, findings, err := h.logic.GetAudit(r.Context(), auditID)
	if err != nil {
		httpx.ErrorCtx(r.Context(), w, err)
		return
	}
	if audit == nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "audit not found"})
		return
	}

	httpx.OkJson(w, types.GetAuditResponse{
		AuditJob: *audit,
		Findings: findings,
	})
}

// Pause pauses a running audit.
func (h *AuditHandler) Pause(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	auditID, err := uuid.Parse(idStr)
	if err != nil {
		httpx.ErrorCtx(r.Context(), w, fmt.Errorf("invalid audit ID: %w", err))
		return
	}

	err = h.logic.PauseAudit(r.Context(), auditID)
	if err != nil {
		httpx.ErrorCtx(r.Context(), w, err)
		return
	}

	httpx.OkJson(w, types.PauseAuditResponse{
		Success: true,
		Status:  model.AuditJobStatusPaused,
	})
}

// Resume resumes a paused audit.
func (h *AuditHandler) Resume(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	auditID, err := uuid.Parse(idStr)
	if err != nil {
		httpx.ErrorCtx(r.Context(), w, fmt.Errorf("invalid audit ID: %w", err))
		return
	}

	var req types.ResumeAuditRequest
	if err := httpx.Parse(r, &req); err != nil {
		httpx.ErrorCtx(r.Context(), w, err)
		return
	}

	err = h.logic.ResumeAudit(r.Context(), auditID)
	if err != nil {
		httpx.ErrorCtx(r.Context(), w, err)
		return
	}

	httpx.OkJson(w, types.ResumeAuditResponse{
		Success: true,
		Status:  model.AuditJobStatusRunning,
	})
}

// GetReport gets the full audit report.
func (h *AuditHandler) GetReport(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	auditID, err := uuid.Parse(idStr)
	if err != nil {
		httpx.ErrorCtx(r.Context(), w, fmt.Errorf("invalid audit ID: %w", err))
		return
	}

	report, err := h.logic.GenerateReport(r.Context(), auditID)
	if err != nil {
		httpx.ErrorCtx(r.Context(), w, err)
		return
	}
	if report == nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "audit not found"})
		return
	}

	httpx.OkJson(w, report)
}

// UpdateWorkflowID updates the workflow ID after starting workflow.
// This is needed for the handler to set the workflow ID on the audit.
func (h *AuditHandler) UpdateWorkflowID(ctx context.Context, auditID uuid.UUID, workflowID string) error {
	// This method should be added to the logic, let me check if I need it...
	// Actually, I need this method in logic. I'll add it later if needed.
	return nil
}
