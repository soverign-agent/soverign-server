// Package grpcserver provides the gRPC server implementation for doc-service.
package grpcserver

import (
	"context"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"sovereign-ai-compliance/doc-service/internal/logic"
	"sovereign-ai-compliance/doc-service/internal/types"
	"sovereign-ai-compliance/doc-service/model"
	"sovereign-ai-compliance/shared/proto/doc/v1"
	"sovereign-ai-compliance/shared/tenant"
)

// Server implements docv1.DocServiceServer.
type Server struct {
	docv1.UnimplementedDocServiceServer

	documentLogic *logic.DocumentLogic
	versionLogic  *logic.VersionLogic
	exportLogic   *logic.ExportLogic
}

// NewServer creates a new gRPC server for doc-service.
func NewServer(documentLogic *logic.DocumentLogic, versionLogic *logic.VersionLogic, exportLogic *logic.ExportLogic) *Server {
	return &Server{
		documentLogic: documentLogic,
		versionLogic:  versionLogic,
		exportLogic:   exportLogic,
	}
}

// Register registers the server on the provided gRPC server.
func (s *Server) Register(grpcServer *grpc.Server) {
	docv1.RegisterDocServiceServer(grpcServer, s)
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

// ListDocuments lists documents with filtering.
func (s *Server) ListDocuments(ctx context.Context, req *docv1.ListDocumentsRequest) (*docv1.ListDocumentsResponse, error) {
	ctx = withTenant(ctx)

	listReq := types.ListDocumentsRequest{
		Page:     int(req.Page),
		PageSize: int(req.PageSize),
	}
	if req.AiSystemId != "" {
		id, err := uuid.Parse(req.AiSystemId)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid ai_system_id: %v", err)
		}
		listReq.AISystemID = &id
	}
	if req.Status != "" {
		listReq.Status = &req.Status
	}
	if req.DocType != "" {
		listReq.DocType = &req.DocType
	}

	resp, err := s.documentLogic.ListDocuments(ctx, listReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list documents: %v", err)
	}

	items := make([]*docv1.DocumentSummary, len(resp.Items))
	for i, item := range resp.Items {
		items[i] = toProtoDocumentSummary(item)
	}

	return &docv1.ListDocumentsResponse{
		Items: items,
		Total: int32(resp.Total),
		Page:  int32(resp.Page),
		Pages: int32(resp.Pages),
	}, nil
}

// GenerateDocument triggers document generation.
func (s *Server) GenerateDocument(ctx context.Context, req *docv1.GenerateDocumentRequest) (*docv1.GenerateDocumentResponse, error) {
	ctx = withTenant(ctx)

	aiSystemID, err := uuid.Parse(req.AiSystemId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid ai_system_id: %v", err)
	}

	docType := docTypeFromProto(req.DocType)
	if docType == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid or missing doc_type")
	}

	var auditJobID *uuid.UUID
	if req.AuditJobId != "" {
		id, err := uuid.Parse(req.AuditJobId)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid audit_job_id: %v", err)
		}
		auditJobID = &id
	}

	genReq := types.GenerateDocumentRequest{
		AISystemID: aiSystemID,
		DocType:    docType,
		AuditJobID: auditJobID,
		Title:      req.Title,
		Options:    req.Options,
	}

	resp, err := s.documentLogic.GenerateDocument(ctx, genReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generate document: %v", err)
	}

	return &docv1.GenerateDocumentResponse{
		DocumentId: resp.DocumentID.String(),
		Status:     docStatusToProto(resp.Status),
	}, nil
}

// GetDocument gets a single document by ID.
func (s *Server) GetDocument(ctx context.Context, req *docv1.GetDocumentRequest) (*docv1.GetDocumentResponse, error) {
	ctx = withTenant(ctx)

	docID, err := uuid.Parse(req.DocumentId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid document_id: %v", err)
	}

	resp, err := s.documentLogic.GetDocument(ctx, types.GetDocumentRequest{DocumentID: docID})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get document: %v", err)
	}
	if resp == nil {
		return nil, status.Errorf(codes.NotFound, "document not found")
	}

	return &docv1.GetDocumentResponse{
		Document: toProtoGeneratedDocument(&resp.GeneratedDocument),
	}, nil
}

// UpdateDocument updates document content.
func (s *Server) UpdateDocument(ctx context.Context, req *docv1.UpdateDocumentRequest) (*docv1.UpdateDocumentResponse, error) {
	ctx = withTenant(ctx)

	docID, err := uuid.Parse(req.DocumentId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid document_id: %v", err)
	}

	var updatedBy uuid.UUID
	if req.UpdatedBy != "" {
		updatedBy, err = uuid.Parse(req.UpdatedBy)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid updated_by: %v", err)
		}
	}

	updateReq := types.UpdateDocumentRequest{
		DocumentID: docID,
		Content:    fromProtoDocumentContent(req.Content),
		UpdatedBy:  updatedBy,
	}

	resp, err := s.documentLogic.UpdateDocument(ctx, updateReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "update document: %v", err)
	}

	return &docv1.UpdateDocumentResponse{
		Document: toProtoGeneratedDocument(&resp.GeneratedDocument),
	}, nil
}

// ListVersions lists version history for a document.
func (s *Server) ListVersions(ctx context.Context, req *docv1.ListVersionsRequest) (*docv1.ListVersionsResponse, error) {
	ctx = withTenant(ctx)

	docID, err := uuid.Parse(req.DocumentId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid document_id: %v", err)
	}

	listReq := types.ListVersionsRequest{
		DocumentID: docID,
		Page:       int(req.Page),
		PageSize:   int(req.PageSize),
	}

	resp, err := s.versionLogic.ListVersions(ctx, listReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list versions: %v", err)
	}

	items := make([]*docv1.DocumentVersion, len(resp.Items))
	for i, item := range resp.Items {
		items[i] = toProtoDocumentVersion(item)
	}

	return &docv1.ListVersionsResponse{
		Items: items,
		Total: int32(resp.Total),
		Page:  int32(resp.Page),
		Pages: int32(resp.Pages),
	}, nil
}

// RollbackVersion rolls back a document to a specific version.
func (s *Server) RollbackVersion(ctx context.Context, req *docv1.RollbackVersionRequest) (*docv1.RollbackVersionResponse, error) {
	ctx = withTenant(ctx)

	docID, err := uuid.Parse(req.DocumentId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid document_id: %v", err)
	}

	rollbackReq := types.RollbackVersionRequest{
		DocumentID:    docID,
		VersionNumber: int(req.VersionNumber),
	}

	resp, err := s.versionLogic.RollbackToVersion(ctx, rollbackReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "rollback version: %v", err)
	}

	return &docv1.RollbackVersionResponse{
		Document: toProtoGeneratedDocument(&resp.GeneratedDocument),
	}, nil
}

// CreateExportJob creates an export job.
func (s *Server) CreateExportJob(ctx context.Context, req *docv1.CreateExportJobRequest) (*docv1.CreateExportJobResponse, error) {
	ctx = withTenant(ctx)

	docID, err := uuid.Parse(req.DocumentId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid document_id: %v", err)
	}

	format := exportFormatFromProto(req.Format)
	if format == "" {
		format = model.ExportFormatPDF
	}

	exportReq := types.CreateExportJobRequest{
		DocumentID: docID,
		Format:     format,
	}

	resp, err := s.exportLogic.CreateExportJob(ctx, exportReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create export job: %v", err)
	}

	return &docv1.CreateExportJobResponse{
		JobId:  resp.JobID.String(),
		Status: exportStatusToProto(resp.Status),
	}, nil
}

// GetExportJob gets an export job status.
func (s *Server) GetExportJob(ctx context.Context, req *docv1.GetExportJobRequest) (*docv1.GetExportJobResponse, error) {
	ctx = withTenant(ctx)

	jobID, err := uuid.Parse(req.JobId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid job_id: %v", err)
	}

	resp, err := s.exportLogic.GetExportJob(ctx, types.GetExportJobRequest{JobID: jobID})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get export job: %v", err)
	}

	return &docv1.GetExportJobResponse{
		Job: toProtoExportJob(&resp.ExportJob),
	}, nil
}

// DownloadExport returns the exported file bytes.
func (s *Server) DownloadExport(ctx context.Context, req *docv1.DownloadExportRequest) (*docv1.DownloadExportResponse, error) {
	ctx = withTenant(ctx)

	jobID, err := uuid.Parse(req.JobId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid job_id: %v", err)
	}

	filePath, job, err := s.exportLogic.DownloadExport(ctx, jobID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "download export: %v", err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read export file: %v", err)
	}

	contentType := "application/octet-stream"
	if job.Format == model.ExportFormatPDF {
		contentType = "application/pdf"
	} else if job.Format == model.ExportFormatDOCX {
		contentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	}

	return &docv1.DownloadExportResponse{
		Filename:    filepath.Base(filePath),
		ContentType: contentType,
		Data:        data,
	}, nil
}
