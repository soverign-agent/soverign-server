// Package client provides gRPC clients for downstream services used by audit-service.
package client

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"sovereign-ai-compliance/audit-service/model"
	"sovereign-ai-compliance/audit-service/repo"
	notificationv1 "sovereign-ai-compliance/shared/proto/notification/v1"
)

// NotificationClient wraps the notification-service gRPC client to send audit alerts.
type NotificationClient struct {
	client notificationv1.NotificationServiceClient
	repo   repo.Repository
}

// NewNotificationClient creates a new NotificationClient.
func NewNotificationClient(conn *grpc.ClientConn, repo repo.Repository) *NotificationClient {
	return &NotificationClient{
		client: notificationv1.NewNotificationServiceClient(conn),
		repo:   repo,
	}
}

// DialNotification creates a gRPC connection to the notification-service.
func DialNotification(addr string, insecureConn bool, tlsCertFile string) (*grpc.ClientConn, error) {
	var opts []grpc.DialOption
	if tlsCertFile != "" {
		creds, err := credentials.NewClientTLSFromFile(tlsCertFile, "")
		if err != nil {
			return nil, fmt.Errorf("load TLS credentials: %w", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else if insecureConn {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		return nil, fmt.Errorf("TLS config required; set tls_cert_file or insecure=true for local dev")
	}
	return grpc.NewClient(addr, opts...)
}

// SendApprovalRequiredNotification sends a notification when an audit reaches the approval gate.
func (n *NotificationClient) SendApprovalRequiredNotification(ctx context.Context, auditID uuid.UUID) error {
	audit, err := n.repo.GetAuditByID(ctx, auditID)
	if err != nil {
		return fmt.Errorf("get audit: %w", err)
	}
	if audit == nil {
		return fmt.Errorf("audit not found: %s", auditID)
	}

	ctx = withTenantMetadata(ctx, audit.TenantID.String())

	priority := mapSeverityToPriority(audit.RiskSeverity)
	title := fmt.Sprintf("Audit approval required: %s", audit.Name)
	body := fmt.Sprintf("Risk score: %d (%s) — Click to review and approve.", audit.RiskScore, audit.RiskSeverity)

	// Fallback: use tenant ID as user ID since AuditJob lacks CreatedBy
	userID := audit.TenantID.String()

	_, err = n.client.SendNotification(ctx, &notificationv1.SendNotificationRequest{
		UserId:    userID,
		Title:     title,
		Body:      body,
		Channel:   notificationv1.NotificationChannel_NOTIFICATION_CHANNEL_IN_APP,
		Priority:  priority,
		ActionUrl: "/approvals",
		Metadata: map[string]string{
			"audit_id":   auditID.String(),
			"audit_name": audit.Name,
			"risk_score": fmt.Sprintf("%d", audit.RiskScore),
			"risk_level": audit.RiskSeverity,
			"event_type": "approval_required",
		},
	})
	if err != nil {
		return fmt.Errorf("send approval notification: %w", err)
	}
	return nil
}

// SendAuditCompletedNotification sends a notification when an audit job completes.
func (n *NotificationClient) SendAuditCompletedNotification(ctx context.Context, auditID uuid.UUID) error {
	audit, err := n.repo.GetAuditByID(ctx, auditID)
	if err != nil {
		return fmt.Errorf("get audit: %w", err)
	}
	if audit == nil {
		return fmt.Errorf("audit not found: %s", auditID)
	}

	ctx = withTenantMetadata(ctx, audit.TenantID.String())

	priority := mapSeverityToPriority(audit.RiskSeverity)
	title := fmt.Sprintf("Audit '%s' completed", audit.Name)
	body := fmt.Sprintf("Risk score: %d (%s). Findings: %d total (%d critical, %d high, %d medium, %d low).",
		audit.RiskScore, audit.RiskSeverity, audit.FindingsCount,
		audit.CriticalFindings, audit.HighFindings, audit.MediumFindings, audit.LowFindings,
	)

	// Fallback: use tenant ID as user ID since AuditJob lacks CreatedBy
	userID := audit.TenantID.String()

	_, err = n.client.SendNotification(ctx, &notificationv1.SendNotificationRequest{
		UserId:    userID,
		Title:     title,
		Body:      body,
		Channel:   notificationv1.NotificationChannel_NOTIFICATION_CHANNEL_IN_APP,
		Priority:  priority,
		ActionUrl: fmt.Sprintf("/audits/%s", auditID.String()),
		Metadata: map[string]string{
			"audit_id":   auditID.String(),
			"audit_name": audit.Name,
			"risk_score": fmt.Sprintf("%d", audit.RiskScore),
			"risk_level": audit.RiskSeverity,
			"event_type": "audit_completed",
		},
	})
	if err != nil {
		return fmt.Errorf("send notification: %w", err)
	}
	return nil
}

func withTenantMetadata(ctx context.Context, tenantID string) context.Context {
	if tenantID == "" {
		return ctx
	}
	md := metadata.Pairs("x-tenant-id", tenantID)
	return metadata.NewOutgoingContext(ctx, md)
}

func mapSeverityToPriority(severity string) notificationv1.NotificationPriority {
	switch severity {
	case model.RiskSeverityCritical:
		return notificationv1.NotificationPriority_NOTIFICATION_PRIORITY_URGENT
	case model.RiskSeverityHigh:
		return notificationv1.NotificationPriority_NOTIFICATION_PRIORITY_HIGH
	case model.RiskSeverityMedium:
		return notificationv1.NotificationPriority_NOTIFICATION_PRIORITY_NORMAL
	default:
		return notificationv1.NotificationPriority_NOTIFICATION_PRIORITY_LOW
	}
}
