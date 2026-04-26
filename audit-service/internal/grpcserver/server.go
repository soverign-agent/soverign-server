// Package grpcserver provides the gRPC server implementation for audit-service.
package grpcserver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"sovereign-ai-compliance/audit-service/internal/logic"
	"sovereign-ai-compliance/audit-service/internal/types"
	"sovereign-ai-compliance/audit-service/model"
	"sovereign-ai-compliance/audit-service/repo"
	"sovereign-ai-compliance/audit-service/temporal"
	"sovereign-ai-compliance/shared/proto/audit/v1"
	"sovereign-ai-compliance/shared/tenant"
)

// Server implements auditv1.AuditServiceServer.
type Server struct {
	auditv1.UnimplementedAuditServiceServer

	logic    *logic.AuditLogic
	repo     repo.Repository
	temporal *temporal.Client
}

// NewServer creates a new gRPC server for audit-service.
func NewServer(logic *logic.AuditLogic, repo repo.Repository, temporal *temporal.Client) *Server {
	return &Server{
		logic:    logic,
		repo:     repo,
		temporal: temporal,
	}
}

// Register registers the server on the provided gRPC server.
func (s *Server) Register(grpcServer *grpc.Server) {
	auditv1.RegisterAuditServiceServer(grpcServer, s)
}

// withTenant extracts tenant ID from gRPC metadata and injects it into context.
func withTenant(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	vals := md.Get("x-tenant-id")
	if len(vals) > 0 && vals[0] != "" {
		return tenant.WithContext(ctx, vals[0])
	}
	return ctx
}

// ListAudits lists audit jobs with filtering.
func (s *Server) ListAudits(ctx context.Context, req *auditv1.ListAuditsRequest) (*auditv1.ListAuditsResponse, error) {
	ctx = withTenant(ctx)

	// Prefer repeated `statuses`; fall back to legacy single `status` for back-compat.
	statuses := req.GetStatuses()
	if len(statuses) == 0 && req.GetStatus() != "" {
		// api-gateway may collapse multiple ?status= params into a comma-separated string;
		// accept both shapes server-side so the client doesn't have to know which form
		// survives the gateway.
		raw := req.GetStatus()
		for _, s := range splitCommaList(raw) {
			if s != "" {
				statuses = append(statuses, s)
			}
		}
	}

	listReq := &types.ListAuditsRequest{
		RepositoryID: uuidPtr(req.RepositoryId),
		Statuses:     statuses,
		Page:         int(req.Page),
		PageSize:     int(req.PageSize),
	}
	if listReq.Page < 1 {
		listReq.Page = 1
	}
	if listReq.PageSize < 1 {
		listReq.PageSize = 20
	}
	if listReq.PageSize > 100 {
		listReq.PageSize = 100
	}

	items, total, err := s.logic.ListAudits(ctx, listReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list audits: %v", err)
	}

	protoItems := make([]*auditv1.AuditJobSummary, len(items))
	for i, item := range items {
		protoItems[i] = toProtoAuditJobSummary(item)
	}

	pages := (total + listReq.PageSize - 1) / listReq.PageSize
	return &auditv1.ListAuditsResponse{
		Items: protoItems,
		Total: int32(total),
		Page:  int32(listReq.Page),
		Pages: int32(pages),
	}, nil
}

// TriggerAudit triggers a new audit job.
func (s *Server) TriggerAudit(ctx context.Context, req *auditv1.TriggerAuditRequest) (*auditv1.TriggerAuditResponse, error) {
	ctx = withTenant(ctx)

	repoID, err := uuid.Parse(req.RepositoryId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid repository_id: %v", err)
	}
	if req.Name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "name is required")
	}

	auditType := auditTypeFromProto(req.AuditType)
	if auditType != model.AuditTypeFull && auditType != model.AuditTypeIncremental {
		return nil, status.Errorf(codes.InvalidArgument, "audit_type must be 'full' or 'incremental'")
	}

	triggerReq := &types.TriggerAuditRequest{
		RepositoryID: repoID,
		Name:         req.Name,
		AuditType:    auditType,
	}

	audit, err := s.logic.TriggerAudit(ctx, triggerReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "trigger audit: %v", err)
	}

	// Start Temporal workflow in background; do not fail request if workflow start fails.
	// The request ctx will be cancelled once the gRPC response is sent, but the workflow
	// must outlive the request — so we use a fresh background ctx and rebind the tenant
	// onto it. The Temporal ContextPropagator picks the tenant up from this ctx and
	// carries it into every activity invocation, so RLS-protected queries succeed.
	tenantID, _ := tenant.FromContext(ctx)
	go func() {
		bgCtx := context.Background()
		if tenantID != "" {
			bgCtx = tenant.WithContext(bgCtx, tenantID)
		}
		workflowRun, err := s.temporal.StartAuditWorkflow(bgCtx, audit.ID.String(), audit.AuditType)
		if err != nil {
			return
		}
		wfID := workflowRun.GetID()
		_ = s.logic.UpdateWorkflowID(bgCtx, audit.ID, wfID)
	}()

	return &auditv1.TriggerAuditResponse{
		AuditId:    audit.ID.String(),
		Status:     statusToProto(audit.Status),
		WorkflowId: "", // set asynchronously
	}, nil
}

// GetAudit gets a single audit by ID with findings.
func (s *Server) GetAudit(ctx context.Context, req *auditv1.GetAuditRequest) (*auditv1.GetAuditResponse, error) {
	ctx = withTenant(ctx)

	auditID, err := uuid.Parse(req.AuditId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid audit_id: %v", err)
	}

	audit, findings, err := s.logic.GetAudit(ctx, auditID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get audit: %v", err)
	}
	if audit == nil {
		return nil, status.Errorf(codes.NotFound, "audit not found")
	}

	protoFindings := make([]*auditv1.Finding, len(findings))
	for i, f := range findings {
		protoFindings[i] = toProtoFinding(f)
	}

	return &auditv1.GetAuditResponse{
		Audit:    toProtoAuditJob(audit),
		Findings: protoFindings,
	}, nil
}

// PauseAudit pauses a running audit.
func (s *Server) PauseAudit(ctx context.Context, req *auditv1.PauseAuditRequest) (*auditv1.PauseAuditResponse, error) {
	ctx = withTenant(ctx)

	auditID, err := uuid.Parse(req.AuditId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid audit_id: %v", err)
	}

	if err := s.logic.PauseAudit(ctx, auditID); err != nil {
		return nil, status.Errorf(codes.Internal, "pause audit: %v", err)
	}

	return &auditv1.PauseAuditResponse{
		Success: true,
		Status:  auditv1.AuditJobStatus_AUDIT_JOB_STATUS_PAUSED,
	}, nil
}

// ResumeAudit resumes a paused audit.
func (s *Server) ResumeAudit(ctx context.Context, req *auditv1.ResumeAuditRequest) (*auditv1.ResumeAuditResponse, error) {
	ctx = withTenant(ctx)

	auditID, err := uuid.Parse(req.AuditId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid audit_id: %v", err)
	}

	if err := s.logic.ResumeAudit(ctx, auditID); err != nil {
		return nil, status.Errorf(codes.Internal, "resume audit: %v", err)
	}

	return &auditv1.ResumeAuditResponse{
		Success: true,
		Status:  auditv1.AuditJobStatus_AUDIT_JOB_STATUS_RUNNING,
	}, nil
}

// GetAuditReport returns the full audit report.
func (s *Server) GetAuditReport(ctx context.Context, req *auditv1.GetAuditReportRequest) (*auditv1.GetAuditReportResponse, error) {
	ctx = withTenant(ctx)

	auditID, err := uuid.Parse(req.AuditId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid audit_id: %v", err)
	}

	report, err := s.logic.GenerateReport(ctx, auditID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generate report: %v", err)
	}
	if report == nil {
		return nil, status.Errorf(codes.NotFound, "audit not found")
	}

	protoFindings := make([]*auditv1.Finding, len(report.Findings))
	for i, f := range report.Findings {
		protoFindings[i] = toProtoFinding(f)
	}

	bySeverity := make(map[string]*auditv1.FindingGroup)
	for k, v := range report.BySeverity {
		bySeverity[k] = toProtoFindingGroup(v)
	}

	byIssueType := make(map[string]*auditv1.FindingGroup)
	for k, v := range report.ByIssueType {
		byIssueType[k] = toProtoFindingGroup(v)
	}

	return &auditv1.GetAuditReportResponse{
		Audit:       toProtoAuditJob(&report.Audit),
		Findings:    protoFindings,
		BySeverity:  bySeverity,
		ByIssueType: byIssueType,
		Summary: &auditv1.AuditReportSummary{
			TotalFindings: int32(report.Summary.TotalFindings),
			Critical:      int32(report.Summary.Critical),
			High:          int32(report.Summary.High),
			Medium:        int32(report.Summary.Medium),
			Low:           int32(report.Summary.Low),
			RiskScore:     int32(report.Summary.RiskScore),
			RiskSeverity:  severityToProto(report.Summary.RiskSeverity),
			Compliant:     report.Summary.Compliant,
		},
	}, nil
}

// GetAuditStatus streams progress events for an audit job.
func (s *Server) GetAuditStatus(req *auditv1.GetAuditStatusRequest, stream auditv1.AuditService_GetAuditStatusServer) error {
	ctx := withTenant(stream.Context())

	auditID, err := uuid.Parse(req.AuditId)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid audit_id: %v", err)
	}

	audit, err := s.repo.GetAuditByID(ctx, auditID)
	if err != nil {
		return status.Errorf(codes.Internal, "get audit: %v", err)
	}
	if audit == nil {
		return status.Errorf(codes.NotFound, "audit not found")
	}

	// If already terminal, send one event and close.
	if audit.Status == model.AuditJobStatusCompleted || audit.Status == model.AuditJobStatusFailed || audit.Status == model.AuditJobStatusCancelled {
		_ = stream.Send(&auditv1.GetAuditStatusResponse{
			Percentage:   100,
			State:        audit.Status,
			CurrentStep:  audit.CurrentStep,
			Message:      fmt.Sprintf("Audit %s", audit.Status),
			ErrorMessage: audit.ErrorMessage,
			Timestamp:    time.Now().Unix(),
		})
		return nil
	}

	// Stream progress by polling the repository every second.
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	lastProgress := audit.ProgressPercentage
	lastState := audit.Status
	lastStep := audit.CurrentStep

	// Always send an initial event so the client gets headers and the current
	// state immediately, even if the audit hasn't moved yet (e.g. stuck in
	// pending). Without this, the response writer buffers indefinitely until
	// the first state change, which the client perceives as a hang.
	if err := stream.Send(&auditv1.GetAuditStatusResponse{
		Percentage:   int32(audit.ProgressPercentage),
		State:        audit.Status,
		CurrentStep:  audit.CurrentStep,
		Message:      fmt.Sprintf("Audit is %s", audit.Status),
		ErrorMessage: audit.ErrorMessage,
		Timestamp:    time.Now().Unix(),
	}); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			current, err := s.repo.GetAuditByID(ctx, auditID)
			if err != nil {
				return status.Errorf(codes.Internal, "poll audit: %v", err)
			}
			if current == nil {
				return status.Errorf(codes.NotFound, "audit not found")
			}

			if current.ProgressPercentage != lastProgress || current.Status != lastState || current.CurrentStep != lastStep {
				if err := stream.Send(&auditv1.GetAuditStatusResponse{
					Percentage:   int32(current.ProgressPercentage),
					State:        current.Status,
					CurrentStep:  current.CurrentStep,
					Message:      fmt.Sprintf("Audit is %s", current.Status),
					ErrorMessage: current.ErrorMessage,
					Timestamp:    time.Now().Unix(),
				}); err != nil {
					return err
				}
				lastProgress = current.ProgressPercentage
				lastState = current.Status
				lastStep = current.CurrentStep
			}

			if current.Status == model.AuditJobStatusCompleted || current.Status == model.AuditJobStatusFailed || current.Status == model.AuditJobStatusCancelled {
				return nil
			}
		}
	}
}

// ListPendingApprovals lists approval requests with optional status filter.
func (s *Server) ListPendingApprovals(ctx context.Context, req *auditv1.ListPendingApprovalsRequest) (*auditv1.ListPendingApprovalsResponse, error) {
	ctx = withTenant(ctx)

	requests, err := s.logic.ListApprovalRequests(ctx, req.Status)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list approvals: %v", err)
	}

	protoItems := make([]*auditv1.ApprovalRequest, len(requests))
	for i, r := range requests {
		protoItems[i] = toProtoApprovalRequest(&r)
	}

	return &auditv1.ListPendingApprovalsResponse{
		Items: protoItems,
		Total: int32(len(requests)),
	}, nil
}

// DecideApproval submits an approval decision and resumes the audit if approved.
func (s *Server) DecideApproval(ctx context.Context, req *auditv1.DecideApprovalRequest) (*auditv1.DecideApprovalResponse, error) {
	ctx = withTenant(ctx)

	approvalID, err := uuid.Parse(req.ApprovalId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid approval_id: %v", err)
	}

	if req.Decision != "approved" && req.Decision != "rejected" {
		return nil, status.Errorf(codes.InvalidArgument, "decision must be 'approved' or 'rejected'")
	}

	// Extract decided_by from JWT claims via context metadata if available
	decidedBy := ""
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("x-user-id"); len(vals) > 0 {
			decidedBy = vals[0]
		}
	}

	// Fetch the approval request first so we know which audit job / workflow to signal.
	approvalReq, err := s.logic.GetApprovalRequest(ctx, approvalID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get approval request: %v", err)
	}
	if approvalReq == nil {
		return nil, status.Errorf(codes.NotFound, "approval request not found")
	}

	if err := s.logic.DecideApproval(ctx, approvalID, req.Decision, decidedBy); err != nil {
		return nil, status.Errorf(codes.Internal, "decide approval: %v", err)
	}

	// Signal the Temporal workflow so it can resume from the approval gate.
	// The workflow ID was set to the audit job ID when the workflow started.
	workflowID := approvalReq.AuditJobID.String()
	signal := temporal.ApproveAuditSignal{Approved: req.Decision == "approved"}
	if err := s.temporal.Client().SignalWorkflow(ctx, workflowID, "", "ApproveAuditSignal", signal); err != nil {
		// Log but don't fail the request — the DB state is already updated.
		// The workflow may still be polled via query or retried later.
		return nil, status.Errorf(codes.Internal, "signal workflow: %v", err)
	}

	statusProto := auditv1.ApprovalStatus_APPROVAL_STATUS_APPROVED
	if req.Decision == "rejected" {
		statusProto = auditv1.ApprovalStatus_APPROVAL_STATUS_REJECTED
	}

	return &auditv1.DecideApprovalResponse{
		Success: true,
		Status:  statusProto,
	}, nil
}

// splitCommaList splits a comma-separated list, trimming whitespace and dropping empties.
// Used to accept api-gateway-collapsed multi-status filters from the legacy `status` query
// param when the client sent `?status=pending,running`.
func splitCommaList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
