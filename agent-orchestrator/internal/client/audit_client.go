package client

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	auditv1 "sovereign-ai-compliance/shared/proto/audit/v1"
)

// AuditClient wraps the audit-service gRPC client with typed helpers.
type AuditClient struct {
	client auditv1.AuditServiceClient
}

// NewAuditClient creates a new AuditClient.
func NewAuditClient(conn *grpc.ClientConn) *AuditClient {
	return &AuditClient{client: auditv1.NewAuditServiceClient(conn)}
}

// TriggerAudit starts a new compliance audit for a repository.
func (c *AuditClient) TriggerAudit(ctx context.Context, repositoryID uuid.UUID, name, auditType string) (*auditv1.TriggerAuditResponse, error) {
	ctx = withTenantMetadata(ctx)
	resp, err := c.client.TriggerAudit(ctx, &auditv1.TriggerAuditRequest{
		RepositoryId: repositoryID.String(),
		Name:         name,
		AuditType:    mapAuditType(auditType),
	})
	if err != nil {
		return nil, fmt.Errorf("trigger audit: %w", err)
	}
	return resp, nil
}

// GetAudit retrieves an audit job with findings.
func (c *AuditClient) GetAudit(ctx context.Context, auditID uuid.UUID) (*auditv1.GetAuditResponse, error) {
	ctx = withTenantMetadata(ctx)
	resp, err := c.client.GetAudit(ctx, &auditv1.GetAuditRequest{
		AuditId: auditID.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("get audit: %w", err)
	}
	return resp, nil
}

// ListAudits lists audit jobs with optional filtering.
func (c *AuditClient) ListAudits(ctx context.Context, status string, page, pageSize int32) (*auditv1.ListAuditsResponse, error) {
	ctx = withTenantMetadata(ctx)
	req := &auditv1.ListAuditsRequest{
		Page:     page,
		PageSize: pageSize,
	}
	if status != "" {
		req.Status = status
	}
	resp, err := c.client.ListAudits(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("list audits: %w", err)
	}
	return resp, nil
}

// GetAuditReport generates a full audit report.
func (c *AuditClient) GetAuditReport(ctx context.Context, auditID uuid.UUID) (*auditv1.GetAuditReportResponse, error) {
	ctx = withTenantMetadata(ctx)
	resp, err := c.client.GetAuditReport(ctx, &auditv1.GetAuditReportRequest{
		AuditId: auditID.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("get audit report: %w", err)
	}
	return resp, nil
}

func mapAuditType(t string) auditv1.AuditType {
	switch t {
	case "full":
		return auditv1.AuditType_AUDIT_TYPE_FULL
	case "incremental":
		return auditv1.AuditType_AUDIT_TYPE_INCREMENTAL
	default:
		return auditv1.AuditType_AUDIT_TYPE_UNSPECIFIED
	}
}
