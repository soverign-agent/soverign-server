package grpcserver

import (
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	"sovereign-ai-compliance/audit-service/model"
	"sovereign-ai-compliance/shared/proto/audit/v1"
)

// statusFromProto converts proto AuditJobStatus to string.
func statusFromProto(s auditv1.AuditJobStatus) string {
	switch s {
	case auditv1.AuditJobStatus_AUDIT_JOB_STATUS_PENDING:
		return model.AuditJobStatusPending
	case auditv1.AuditJobStatus_AUDIT_JOB_STATUS_RUNNING:
		return model.AuditJobStatusRunning
	case auditv1.AuditJobStatus_AUDIT_JOB_STATUS_PAUSED:
		return model.AuditJobStatusPaused
	case auditv1.AuditJobStatus_AUDIT_JOB_STATUS_COMPLETED:
		return model.AuditJobStatusCompleted
	case auditv1.AuditJobStatus_AUDIT_JOB_STATUS_FAILED:
		return model.AuditJobStatusFailed
	case auditv1.AuditJobStatus_AUDIT_JOB_STATUS_CANCELLED:
		return model.AuditJobStatusCancelled
	default:
		return ""
	}
}

// statusToProto converts string status to proto AuditJobStatus.
func statusToProto(s string) auditv1.AuditJobStatus {
	switch s {
	case model.AuditJobStatusPending:
		return auditv1.AuditJobStatus_AUDIT_JOB_STATUS_PENDING
	case model.AuditJobStatusRunning:
		return auditv1.AuditJobStatus_AUDIT_JOB_STATUS_RUNNING
	case model.AuditJobStatusPaused:
		return auditv1.AuditJobStatus_AUDIT_JOB_STATUS_PAUSED
	case model.AuditJobStatusCompleted:
		return auditv1.AuditJobStatus_AUDIT_JOB_STATUS_COMPLETED
	case model.AuditJobStatusFailed:
		return auditv1.AuditJobStatus_AUDIT_JOB_STATUS_FAILED
	case model.AuditJobStatusCancelled:
		return auditv1.AuditJobStatus_AUDIT_JOB_STATUS_CANCELLED
	default:
		return auditv1.AuditJobStatus_AUDIT_JOB_STATUS_UNSPECIFIED
	}
}

// severityToProto converts string severity to proto RiskSeverity.
func severityToProto(s string) auditv1.RiskSeverity {
	switch s {
	case model.RiskSeverityLow:
		return auditv1.RiskSeverity_RISK_SEVERITY_LOW
	case model.RiskSeverityMedium:
		return auditv1.RiskSeverity_RISK_SEVERITY_MEDIUM
	case model.RiskSeverityHigh:
		return auditv1.RiskSeverity_RISK_SEVERITY_HIGH
	case model.RiskSeverityCritical:
		return auditv1.RiskSeverity_RISK_SEVERITY_CRITICAL
	default:
		return auditv1.RiskSeverity_RISK_SEVERITY_UNSPECIFIED
	}
}

// severityFromProto converts proto RiskSeverity to string.
func severityFromProto(s auditv1.RiskSeverity) string {
	switch s {
	case auditv1.RiskSeverity_RISK_SEVERITY_LOW:
		return model.RiskSeverityLow
	case auditv1.RiskSeverity_RISK_SEVERITY_MEDIUM:
		return model.RiskSeverityMedium
	case auditv1.RiskSeverity_RISK_SEVERITY_HIGH:
		return model.RiskSeverityHigh
	case auditv1.RiskSeverity_RISK_SEVERITY_CRITICAL:
		return model.RiskSeverityCritical
	default:
		return ""
	}
}

// issueTypeToProto converts string issue type to proto IssueType.
func issueTypeToProto(t string) auditv1.IssueType {
	switch t {
	case model.IssueTypeDataPrivacy:
		return auditv1.IssueType_ISSUE_TYPE_DATA_PRIVACY
	case model.IssueTypeTransparency:
		return auditv1.IssueType_ISSUE_TYPE_TRANSPARENCY
	case model.IssueTypeHumanOversight:
		return auditv1.IssueType_ISSUE_TYPE_HUMAN_OVERSIGHT
	case model.IssueTypeAccuracy:
		return auditv1.IssueType_ISSUE_TYPE_ACCURACY
	case model.IssueTypeSecurity:
		return auditv1.IssueType_ISSUE_TYPE_SECURITY
	case model.IssueTypeRecordKeeping:
		return auditv1.IssueType_ISSUE_TYPE_RECORD_KEEPING
	default:
		return auditv1.IssueType_ISSUE_TYPE_UNSPECIFIED
	}
}

// auditTypeToProto converts string audit type to proto AuditType.
func auditTypeToProto(t string) auditv1.AuditType {
	switch t {
	case model.AuditTypeFull:
		return auditv1.AuditType_AUDIT_TYPE_FULL
	case model.AuditTypeIncremental:
		return auditv1.AuditType_AUDIT_TYPE_INCREMENTAL
	default:
		return auditv1.AuditType_AUDIT_TYPE_UNSPECIFIED
	}
}

// auditTypeFromProto converts proto AuditType to string.
func auditTypeFromProto(t auditv1.AuditType) string {
	switch t {
	case auditv1.AuditType_AUDIT_TYPE_FULL:
		return model.AuditTypeFull
	case auditv1.AuditType_AUDIT_TYPE_INCREMENTAL:
		return model.AuditTypeIncremental
	default:
		return ""
	}
}

// toProtoAuditJob converts model.AuditJob to proto.
func toProtoAuditJob(a *model.AuditJob) *auditv1.AuditJob {
	if a == nil {
		return nil
	}
	return &auditv1.AuditJob{
		Id:                 a.ID.String(),
		TenantId:           a.TenantID.String(),
		RepositoryId:       a.RepositoryID.String(),
		Name:               a.Name,
		AuditType:          auditTypeToProto(a.AuditType),
		Status:             statusToProto(a.Status),
		RiskScore:          int32(a.RiskScore),
		RiskSeverity:       severityToProto(a.RiskSeverity),
		ProgressPercentage: int32(a.ProgressPercentage),
		FindingsCount:      int32(a.FindingsCount),
		CriticalFindings:   int32(a.CriticalFindings),
		HighFindings:       int32(a.HighFindings),
		MediumFindings:     int32(a.MediumFindings),
		LowFindings:        int32(a.LowFindings),
		WorkflowId:         pointerString(a.WorkflowID),
		StartedAt:          timeToProto(a.StartedAt),
		CompletedAt:        timeToProto(a.CompletedAt),
		CreatedAt:          timestamppb.New(a.CreatedAt),
		UpdatedAt:          timestamppb.New(a.UpdatedAt),
	}
}

// toProtoAuditJobSummary converts model.AuditJobSummary to proto.
func toProtoAuditJobSummary(a model.AuditJobSummary) *auditv1.AuditJobSummary {
	return &auditv1.AuditJobSummary{
		Id:            a.ID.String(),
		Name:          a.Name,
		AuditType:     auditTypeToProto(a.AuditType),
		Status:        statusToProto(a.Status),
		RiskScore:     int32(a.RiskScore),
		RiskSeverity:  severityToProto(a.RiskSeverity),
		FindingsCount: int32(a.FindingsCount),
		CreatedAt:     timestamppb.New(a.CreatedAt),
	}
}

// toProtoFinding converts model.Finding to proto.
func toProtoFinding(f model.Finding) *auditv1.Finding {
	var lineNumber int32
	if f.LineNumber != nil {
		lineNumber = int32(*f.LineNumber)
	}
	return &auditv1.Finding{
		Id:          f.ID.String(),
		TenantId:    f.TenantID.String(),
		AuditJobId:  f.AuditJobID.String(),
		FilePath:    f.FilePath,
		LineNumber:  lineNumber,
		IssueType:   issueTypeToProto(f.IssueType),
		Severity:    severityToProto(f.Severity),
		Title:       f.Title,
		Description: f.Description,
		Remediation: pointerString(f.Remediation),
		CreatedAt:   timestamppb.New(f.CreatedAt),
	}
}

// toProtoAuditReportSummary converts model report summary fields to proto.
func toProtoAuditReportSummary(summary *auditv1.AuditReportSummary) *auditv1.AuditReportSummary {
	return summary
}

// timeToProto converts *time.Time to *timestamppb.Timestamp.
func timeToProto(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

// pointerString converts *string to string (empty if nil).
func pointerString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// uuidPtr parses a string into *uuid.UUID, returning nil on empty or invalid.
func uuidPtr(s string) *uuid.UUID {
	if s == "" {
		return nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil
	}
	return &id
}

// toProtoFindingGroup converts a slice of model.Findings to proto FindingGroup.
func toProtoFindingGroup(findings []model.Finding) *auditv1.FindingGroup {
	items := make([]*auditv1.Finding, len(findings))
	for i, f := range findings {
		items[i] = toProtoFinding(f)
	}
	return &auditv1.FindingGroup{Findings: items}
}

// approvalStatusToProto converts string approval status to proto ApprovalStatus.
func approvalStatusToProto(s string) auditv1.ApprovalStatus {
	switch s {
	case model.ApprovalStatusPending:
		return auditv1.ApprovalStatus_APPROVAL_STATUS_PENDING
	case model.ApprovalStatusApproved:
		return auditv1.ApprovalStatus_APPROVAL_STATUS_APPROVED
	case model.ApprovalStatusRejected:
		return auditv1.ApprovalStatus_APPROVAL_STATUS_REJECTED
	default:
		return auditv1.ApprovalStatus_APPROVAL_STATUS_UNSPECIFIED
	}
}

// toProtoApprovalRequest converts model.ApprovalRequest to proto.
func toProtoApprovalRequest(a *model.ApprovalRequest) *auditv1.ApprovalRequest {
	if a == nil {
		return nil
	}
	return &auditv1.ApprovalRequest{
		Id:          a.ID.String(),
		TenantId:    a.TenantID.String(),
		AuditJobId:  a.AuditJobID.String(),
		RequestedBy: a.RequestedBy,
		AssignedTo:  a.AssignedTo,
		Title:       a.Title,
		ContextJson: a.ContextJSON,
		Status:      approvalStatusToProto(a.Status),
		CreatedAt:   timestamppb.New(a.CreatedAt),
		DecidedAt:   timeToProto(a.DecidedAt),
		DecidedBy:   pointerString(a.DecidedBy),
	}
}
