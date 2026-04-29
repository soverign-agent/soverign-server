// Package grpcserver provides the gRPC server implementation for rag-service.
package grpcserver

import (
	"context"
	"mime/multipart"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"sovereign-ai-compliance/rag-service/internal/logic"
	ragv1 "sovereign-ai-compliance/shared/proto/rag/v1"
	"sovereign-ai-compliance/shared/tenant"
)

// Server implements ragv1.RAGServiceServer.
type Server struct {
	ragv1.UnimplementedRAGServiceServer

	documentsLogic *logic.DocumentsLogic
	searchLogic    *logic.SearchLogic
	statsLogic     *logic.StatsLogic
}

// NewServer creates a new gRPC server for rag-service.
func NewServer(documentsLogic *logic.DocumentsLogic, searchLogic *logic.SearchLogic, statsLogic *logic.StatsLogic) *Server {
	return &Server{
		documentsLogic: documentsLogic,
		searchLogic:    searchLogic,
		statsLogic:     statsLogic,
	}
}

// Register registers the server on the provided gRPC server.
func (s *Server) Register(grpcServer *grpc.Server) {
	ragv1.RegisterRAGServiceServer(grpcServer, s)
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

// UploadDocument uploads a new document with file content.
func (s *Server) UploadDocument(ctx context.Context, req *ragv1.UploadDocumentRequest) (*ragv1.UploadDocumentResponse, error) {
	ctx = withTenant(ctx)

	if req.Name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "name is required")
	}

	uploadReq := logic.UploadDocumentRequest{
		Name:        req.Name,
		Description: req.Description,
		File: &multipart.FileHeader{
			Filename: req.Name,
			Size:     int64(len(req.FileContent)),
		},
	}
	if req.AiSystemId != "" {
		aiSystemID, err := uuid.Parse(req.AiSystemId)
		if err == nil {
			uploadReq.AISystemID = &aiSystemID
		}
	}

	resp, err := s.documentsLogic.UploadDocument(ctx, uploadReq, req.FileContent)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "upload document: %v", err)
	}

	return &ragv1.UploadDocumentResponse{
		DocumentId: resp.DocumentID.String(),
		Status:     resp.Status,
	}, nil
}

// ListDocuments lists documents with pagination.
func (s *Server) ListDocuments(ctx context.Context, req *ragv1.ListDocumentsRequest) (*ragv1.ListDocumentsResponse, error) {
	ctx = withTenant(ctx)

	listReq := logic.ListDocumentsRequest{
		Page:     int(req.Page),
		PageSize: int(req.PageSize),
	}
	if req.AiSystemId != "" {
		aiSystemID, err := uuid.Parse(req.AiSystemId)
		if err == nil {
			listReq.AISystemID = &aiSystemID
		}
	}

	resp, err := s.documentsLogic.ListDocuments(ctx, listReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list documents: %v", err)
	}

	docs := make([]*ragv1.Document, len(resp.Documents))
	for i, d := range resp.Documents {
		docs[i] = toProtoDocument(d)
	}

	return &ragv1.ListDocumentsResponse{
		Documents: docs,
		Total:     int32(resp.Total),
		Page:      int32(resp.Page),
		PageSize:  int32(resp.PageSize),
	}, nil
}

// DeleteDocument deletes a document by ID.
func (s *Server) DeleteDocument(ctx context.Context, req *ragv1.DeleteDocumentRequest) (*ragv1.DeleteDocumentResponse, error) {
	ctx = withTenant(ctx)

	docID, err := uuid.Parse(req.DocumentId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid document_id: %v", err)
	}

	deleteReq := logic.DeleteDocumentRequest{
		DocumentID: docID,
	}

	if err := s.documentsLogic.DeleteDocument(ctx, deleteReq); err != nil {
		return nil, status.Errorf(codes.Internal, "delete document: %v", err)
	}

	return &ragv1.DeleteDocumentResponse{
		Success: true,
	}, nil
}

// ReprocessDocument reprocesses an existing document.
func (s *Server) ReprocessDocument(ctx context.Context, req *ragv1.ReprocessDocumentRequest) (*ragv1.ReprocessDocumentResponse, error) {
	ctx = withTenant(ctx)

	docID, err := uuid.Parse(req.DocumentId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid document_id: %v", err)
	}

	reprocessReq := logic.ReprocessDocumentRequest{
		DocumentID:  docID,
		FileContent: req.FileContent,
	}

	if err := s.documentsLogic.ReprocessDocument(ctx, reprocessReq); err != nil {
		return nil, status.Errorf(codes.Internal, "reprocess document: %v", err)
	}

	return &ragv1.ReprocessDocumentResponse{
		Success: true,
	}, nil
}

// Search performs similarity search for the query.
func (s *Server) Search(ctx context.Context, req *ragv1.SearchRequest) (*ragv1.SearchResponse, error) {
	ctx = withTenant(ctx)

	searchReq := logic.SearchRequest{
		Query: req.Query,
		TopK:  int(req.TopK),
	}

	resp, err := s.searchLogic.Search(ctx, searchReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "search: %v", err)
	}

	results := make([]*ragv1.SearchResult, len(resp.Results))
	for i, r := range resp.Results {
		results[i] = toProtoSearchResult(r)
	}

	return &ragv1.SearchResponse{
		Query:   resp.Query,
		TopK:    int32(resp.TopK),
		Results: results,
	}, nil
}

// GetStats returns RAG statistics for the current tenant.
func (s *Server) GetStats(ctx context.Context, req *ragv1.GetStatsRequest) (*ragv1.GetStatsResponse, error) {
	ctx = withTenant(ctx)

	resp, err := s.statsLogic.GetStats(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get stats: %v", err)
	}

	return &ragv1.GetStatsResponse{
		DocumentCount:       int32(resp.DocumentCount),
		CompletedDocuments:  int32(resp.CompletedDocuments),
		PendingDocuments:    int32(resp.PendingDocuments),
		EmbeddingCount:      int32(resp.EmbeddingCount),
		TotalFileSizeBytes:  resp.TotalFileSizeBytes,
	}, nil
}
