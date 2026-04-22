package client

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	ragv1 "sovereign-ai-compliance/shared/proto/rag/v1"
)

// RAGClient wraps the rag-service gRPC client with typed helpers.
type RAGClient struct {
	client ragv1.RAGServiceClient
}

// NewRAGClient creates a new RAGClient.
func NewRAGClient(conn *grpc.ClientConn) *RAGClient {
	return &RAGClient{client: ragv1.NewRAGServiceClient(conn)}
}

// Search performs a vector search over the knowledge base.
func (c *RAGClient) Search(ctx context.Context, query string, topK int32) (*ragv1.SearchResponse, error) {
	ctx = withTenantMetadata(ctx)
	resp, err := c.client.Search(ctx, &ragv1.SearchRequest{
		Query: query,
		TopK:  topK,
	})
	if err != nil {
		return nil, fmt.Errorf("rag search: %w", err)
	}
	return resp, nil
}

// GetStats retrieves RAG service statistics.
func (c *RAGClient) GetStats(ctx context.Context) (*ragv1.GetStatsResponse, error) {
	ctx = withTenantMetadata(ctx)
	resp, err := c.client.GetStats(ctx, &ragv1.GetStatsRequest{})
	if err != nil {
		return nil, fmt.Errorf("rag get stats: %w", err)
	}
	return resp, nil
}

// UploadDocument uploads a document for processing and embedding.
func (c *RAGClient) UploadDocument(ctx context.Context, name string, content []byte, docType string) (*ragv1.UploadDocumentResponse, error) {
	ctx = withTenantMetadata(ctx)
	resp, err := c.client.UploadDocument(ctx, &ragv1.UploadDocumentRequest{
		Name:        name,
		FileContent: content,
		FileType:    docType,
	})
	if err != nil {
		return nil, fmt.Errorf("upload document: %w", err)
	}
	return resp, nil
}
