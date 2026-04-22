// Package grpcserver provides the gRPC server implementation for rag-service.
package grpcserver

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"sovereign-ai-compliance/rag-service/internal/logic"
	"sovereign-ai-compliance/rag-service/model"
	ragv1 "sovereign-ai-compliance/shared/proto/rag/v1"
)

// toProtoDocument converts model.Document to proto Document.
func toProtoDocument(d model.Document) *ragv1.Document {
	return &ragv1.Document{
		Id:          d.ID.String(),
		TenantId:    d.TenantID.String(),
		Name:        d.Name,
		Description: d.Description,
		FileType:    d.FileType,
		FileSize:    d.FileSize,
		Status:      d.Status,
		CreatedAt:   timestamppb.New(d.CreatedAt),
		UpdatedAt:   timestamppb.New(d.UpdatedAt),
	}
}

// toProtoSearchResult converts logic.SearchResult to proto SearchResult.
func toProtoSearchResult(r logic.SearchResult) *ragv1.SearchResult {
	return &ragv1.SearchResult{
		DocumentId:   r.DocumentID,
		DocumentName: r.DocumentName,
		Text:         r.Text,
		Similarity:   r.Similarity,
	}
}

// timeToProto converts *time.Time to proto timestamp.
func timeToProto(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}
