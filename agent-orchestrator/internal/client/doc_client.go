// Package client provides typed gRPC clients for downstream services used by agent-orchestrator.
package client

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	docv1 "sovereign-ai-compliance/shared/proto/doc/v1"
)

// DocClient wraps the doc-service gRPC client with typed helpers.
type DocClient struct {
	client docv1.DocServiceClient
}

// NewDocClient creates a new DocClient.
func NewDocClient(conn *grpc.ClientConn) *DocClient {
	return &DocClient{client: docv1.NewDocServiceClient(conn)}
}

// GenerateDocument triggers async document generation for an AI system.
func (c *DocClient) GenerateDocument(ctx context.Context, aiSystemID uuid.UUID, docType, title string) (*docv1.GenerateDocumentResponse, error) {
	ctx = withTenantMetadata(ctx)
	resp, err := c.client.GenerateDocument(ctx, &docv1.GenerateDocumentRequest{
		AiSystemId: aiSystemID.String(),
		DocType:    mapDocType(docType),
		Title:      title,
	})
	if err != nil {
		return nil, fmt.Errorf("generate document: %w", err)
	}
	return resp, nil
}

func mapDocType(t string) docv1.DocType {
	switch t {
	case "annex_iv":
		return docv1.DocType_DOC_TYPE_ANNEX_IV
	case "risk_report":
		return docv1.DocType_DOC_TYPE_RISK_REPORT
	case "compliance":
		return docv1.DocType_DOC_TYPE_COMPLIANCE
	default:
		return docv1.DocType_DOC_TYPE_UNSPECIFIED
	}
}

// GetDocument retrieves a generated document by ID.
func (c *DocClient) GetDocument(ctx context.Context, docID uuid.UUID) (*docv1.GetDocumentResponse, error) {
	ctx = withTenantMetadata(ctx)
	resp, err := c.client.GetDocument(ctx, &docv1.GetDocumentRequest{
		DocumentId: docID.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("get document: %w", err)
	}
	return resp, nil
}

// ListDocuments lists documents with optional filtering.
func (c *DocClient) ListDocuments(ctx context.Context, aiSystemID *uuid.UUID, docType, status string, page, pageSize int32) (*docv1.ListDocumentsResponse, error) {
	ctx = withTenantMetadata(ctx)
	req := &docv1.ListDocumentsRequest{
		Page:     page,
		PageSize: pageSize,
	}
	if aiSystemID != nil {
		req.AiSystemId = aiSystemID.String()
	}
	if docType != "" {
		req.DocType = docType
	}
	if status != "" {
		req.Status = status
	}
	resp, err := c.client.ListDocuments(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	return resp, nil
}
