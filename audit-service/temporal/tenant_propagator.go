package temporal

import (
	"context"
	"fmt"

	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/workflow"

	"sovereign-ai-compliance/shared/tenant"
)

// tenantHeaderKey is the key used to carry the tenant ID across Temporal
// workflow / activity boundaries via headers. Headers are persisted in the
// workflow history, so this value survives worker restarts and replay.
const tenantHeaderKey = "x-tenant-id"

// tenantWorkflowKey is the workflow.Context key used to stash the tenant ID
// after Extract reads it from the header. It is intentionally unexported and
// scoped to this package — workflow code should never read tenant directly;
// activities receive it on their *Go* context via the propagator.
type tenantWorkflowKey struct{}

// tenantPropagator carries the tenant ID across the Go-context →
// workflow.Context → activity Go-context boundary. Without it, Temporal
// activities run with a bare context and every RLS-protected query fails
// with "tenant context required".
type tenantPropagator struct{}

// NewTenantPropagator returns a Temporal ContextPropagator that round-trips
// the shared/tenant context value through workflow headers.
func NewTenantPropagator() workflow.ContextPropagator {
	return &tenantPropagator{}
}

// Inject is called when starting a workflow or scheduling an activity from a
// regular Go context (e.g. the gRPC handler that calls StartAuditWorkflow).
// It reads the tenant ID from the Go context and writes it to the workflow
// header so the workflow / activity can read it back on the other side.
func (t *tenantPropagator) Inject(ctx context.Context, writer workflow.HeaderWriter) error {
	tenantID, ok := tenant.FromContext(ctx)
	if !ok || tenantID == "" {
		return nil
	}
	payload, err := encodeTenantPayload(tenantID)
	if err != nil {
		return err
	}
	writer.Set(tenantHeaderKey, payload)
	return nil
}

// InjectFromWorkflow is called when scheduling an activity from inside a
// workflow. It reads the tenant ID from the workflow.Context (where
// ExtractToWorkflow stashed it on workflow start) and writes it back into the
// activity's header so Extract can resurrect it as a Go context value.
func (t *tenantPropagator) InjectFromWorkflow(ctx workflow.Context, writer workflow.HeaderWriter) error {
	val := ctx.Value(tenantWorkflowKey{})
	if val == nil {
		return nil
	}
	tenantID, ok := val.(string)
	if !ok || tenantID == "" {
		return nil
	}
	payload, err := encodeTenantPayload(tenantID)
	if err != nil {
		return err
	}
	writer.Set(tenantHeaderKey, payload)
	return nil
}

// Extract is called when an activity worker receives a task. It pulls the
// tenant ID out of the header and attaches it to the activity's Go context
// via the shared/tenant package — so any repo / RLS code downstream sees the
// tenant exactly as it would on a regular gRPC request path.
func (t *tenantPropagator) Extract(ctx context.Context, reader workflow.HeaderReader) (context.Context, error) {
	payload, ok := reader.Get(tenantHeaderKey)
	if !ok {
		return ctx, nil
	}
	tenantID, err := decodeTenantPayload(payload)
	if err != nil {
		return ctx, err
	}
	if tenantID == "" {
		return ctx, nil
	}
	return tenant.WithContext(ctx, tenantID), nil
}

// ExtractToWorkflow is called when a workflow task is dispatched (including on
// every replay). It stashes the tenant ID on the workflow.Context under a
// private key so InjectFromWorkflow can find it when scheduling activities.
func (t *tenantPropagator) ExtractToWorkflow(ctx workflow.Context, reader workflow.HeaderReader) (workflow.Context, error) {
	payload, ok := reader.Get(tenantHeaderKey)
	if !ok {
		return ctx, nil
	}
	tenantID, err := decodeTenantPayload(payload)
	if err != nil {
		return ctx, err
	}
	if tenantID == "" {
		return ctx, nil
	}
	return workflow.WithValue(ctx, tenantWorkflowKey{}, tenantID), nil
}

func encodeTenantPayload(tenantID string) (*commonpb.Payload, error) {
	payload, err := converter.GetDefaultDataConverter().ToPayload(tenantID)
	if err != nil {
		return nil, fmt.Errorf("encode tenant id: %w", err)
	}
	return payload, nil
}

func decodeTenantPayload(payload *commonpb.Payload) (string, error) {
	var tenantID string
	if err := converter.GetDefaultDataConverter().FromPayload(payload, &tenantID); err != nil {
		return "", fmt.Errorf("decode tenant id: %w", err)
	}
	return tenantID, nil
}
