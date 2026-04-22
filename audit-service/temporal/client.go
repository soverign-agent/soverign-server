// Package temporal provides Temporal workflow and activity implementations for audit execution.
package temporal

import (
	"context"
	"fmt"

	"sovereign-ai-compliance/audit-service/internal/logic"
	"sovereign-ai-compliance/audit-service/repo"
	"sovereign-ai-compliance/audit-service/scoring"
	sharedconfig "sovereign-ai-compliance/shared/config"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

// Client wraps a Temporal client and worker for audit workflows.
type Client struct {
	temporalClient client.Client
	worker         worker.Worker
}

// NewClient creates a new Temporal client and worker.
func NewClient(cfg sharedconfig.TemporalConfig, repo repo.Repository, logic *logic.AuditLogic, calculator *scoring.Calculator, notificationClient NotificationServiceClient) (*Client, error) {
	// Create Temporal client
	tc, err := client.NewClient(client.Options{
		HostPort:  cfg.HostPort,
		Namespace: cfg.Namespace,
	})
	if err != nil {
		return nil, fmt.Errorf("create temporal client: %w", err)
	}

	// Create worker
	w := worker.New(tc, "audit-task-queue", worker.Options{})

	// Register workflow and activities
	w.RegisterWorkflow(&ComplianceAuditWorkflow{})

	activities := NewActivities(repo, logic, calculator, nil, notificationClient)
	w.RegisterActivity(activities)

	c := &Client{
		temporalClient: tc,
		worker:         w,
	}

	return c, nil
}

// Start starts the worker.
func (c *Client) Start() error {
	err := c.worker.Start()
	if err != nil {
		return fmt.Errorf("start temporal worker: %w", err)
	}
	return nil
}

// Stop stops the client and worker.
func (c *Client) Stop() {
	c.worker.Stop()
	c.temporalClient.Close()
}

// Client returns the underlying Temporal client.
func (c *Client) Client() client.Client {
	return c.temporalClient
}

// StartAuditWorkflow starts a new compliance audit workflow.
func (c *Client) StartAuditWorkflow(ctx context.Context, auditID string) (client.WorkflowRun, error) {
	options := client.StartWorkflowOptions{
		ID:        auditID,
		TaskQueue: "audit-task-queue",
	}

	workflowParams := ComplianceAuditWorkflowParams{
		AuditID: auditID,
	}

	return c.temporalClient.ExecuteWorkflow(ctx, options, new(ComplianceAuditWorkflow).Execute, workflowParams)
}
