// Package grpcserver provides the gRPC server implementation for rag-service.
package grpcserver

import (
	"context"
	"errors"
	"mime/multipart"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"sovereign-ai-compliance/rag-service/internal/logic"
	"sovereign-ai-compliance/rag-service/internal/orchestrator"
	"sovereign-ai-compliance/rag-service/model"
	ragv1 "sovereign-ai-compliance/shared/proto/rag/v1"
	"sovereign-ai-compliance/shared/tenant"
)

var tracer = otel.Tracer("rag-service/grpcserver")

// Server implements ragv1.RAGServiceServer.
type Server struct {
	ragv1.UnimplementedRAGServiceServer

	documentsLogic *logic.DocumentsLogic
	searchLogic    *logic.SearchLogic
	statsLogic     *logic.StatsLogic
	chatLogic      *logic.ChatLogic
}

// NewServer creates a new gRPC server for rag-service. chatLogic may be nil;
// chat RPCs will return Unimplemented in that case so the rest of the service
// remains operational.
func NewServer(
	documentsLogic *logic.DocumentsLogic,
	searchLogic *logic.SearchLogic,
	statsLogic *logic.StatsLogic,
	chatLogic *logic.ChatLogic,
) *Server {
	return &Server{
		documentsLogic: documentsLogic,
		searchLogic:    searchLogic,
		statsLogic:     statsLogic,
		chatLogic:      chatLogic,
	}
}

// Register registers the server on the provided gRPC server.
func (s *Server) Register(grpcServer *grpc.Server) {
	ragv1.RegisterRAGServiceServer(grpcServer, s)
}

// withTenant extracts tenant ID and user ID from gRPC metadata and injects
// both into context. Tenant uses shared/tenant; user ID is added through
// logic.WithUserID so chat handlers can look it up.
func withTenant(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	if vals := md.Get("x-tenant-id"); len(vals) > 0 && vals[0] != "" {
		ctx = tenant.WithContext(ctx, vals[0])
	}
	if vals := md.Get("x-user-id"); len(vals) > 0 && vals[0] != "" {
		if uid, err := uuid.Parse(vals[0]); err == nil {
			ctx = logic.WithUserID(ctx, uid)
		}
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
		DocumentCount:      int32(resp.DocumentCount),
		CompletedDocuments: int32(resp.CompletedDocuments),
		PendingDocuments:   int32(resp.PendingDocuments),
		EmbeddingCount:     int32(resp.EmbeddingCount),
		TotalFileSizeBytes: resp.TotalFileSizeBytes,
	}, nil
}

// =============================================================================
// Chat / agentic RAG
// =============================================================================

// chatLogicAvailable returns a gRPC error if the chat logic is not configured.
func (s *Server) chatLogicAvailable() error {
	if s.chatLogic == nil {
		return status.Errorf(codes.Unimplemented, "chat is not enabled in this build")
	}
	return nil
}

// CreateSession starts a new chat session for the authenticated user.
func (s *Server) CreateSession(ctx context.Context, req *ragv1.CreateSessionRequest) (*ragv1.CreateSessionResponse, error) {
	if err := s.chatLogicAvailable(); err != nil {
		return nil, err
	}
	ctx = withTenant(ctx)

	session, err := s.chatLogic.CreateSession(ctx, logic.CreateSessionRequest{
		Title: req.Title,
	})
	if err != nil {
		return nil, mapChatError(err, "create session")
	}
	return &ragv1.CreateSessionResponse{Session: toProtoChatSession(session)}, nil
}

// ListSessions returns the authenticated user's recent chat sessions.
func (s *Server) ListSessions(ctx context.Context, req *ragv1.ListSessionsRequest) (*ragv1.ListSessionsResponse, error) {
	if err := s.chatLogicAvailable(); err != nil {
		return nil, err
	}
	ctx = withTenant(ctx)

	resp, err := s.chatLogic.ListSessions(ctx, logic.ListSessionsRequest{
		Page:     int(req.Page),
		PageSize: int(req.PageSize),
	})
	if err != nil {
		return nil, mapChatError(err, "list sessions")
	}

	out := make([]*ragv1.ChatSession, 0, len(resp.Sessions))
	for i := range resp.Sessions {
		out = append(out, toProtoChatSession(&resp.Sessions[i]))
	}
	return &ragv1.ListSessionsResponse{
		Sessions: out,
		Total:    int32(resp.Total),
		Page:     int32(resp.Page),
		PageSize: int32(resp.PageSize),
	}, nil
}

// GetSessionHistory returns paginated messages for a session owned by the user.
func (s *Server) GetSessionHistory(ctx context.Context, req *ragv1.GetSessionHistoryRequest) (*ragv1.GetSessionHistoryResponse, error) {
	if err := s.chatLogicAvailable(); err != nil {
		return nil, err
	}
	ctx = withTenant(ctx)

	sessionID, err := uuid.Parse(req.SessionId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid session_id: %v", err)
	}

	resp, err := s.chatLogic.GetSessionHistory(ctx, logic.GetSessionHistoryRequest{
		SessionID: sessionID,
		Page:      int(req.Page),
		PageSize:  int(req.PageSize),
	})
	if err != nil {
		return nil, mapChatError(err, "get session history")
	}

	messages := make([]*ragv1.ChatMessage, 0, len(resp.Messages))
	for i := range resp.Messages {
		messages = append(messages, toProtoChatMessage(&resp.Messages[i]))
	}
	return &ragv1.GetSessionHistoryResponse{
		Session:  toProtoChatSession(resp.Session),
		Messages: messages,
		Total:    int32(resp.Total),
		Page:     int32(resp.Page),
		PageSize: int32(resp.PageSize),
	}, nil
}

// DeleteSession deletes a chat session by ID.
func (s *Server) DeleteSession(ctx context.Context, req *ragv1.DeleteSessionRequest) (*ragv1.DeleteSessionResponse, error) {
	if err := s.chatLogicAvailable(); err != nil {
		return nil, err
	}
	ctx = withTenant(ctx)

	sessionID, err := uuid.Parse(req.SessionId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid session_id: %v", err)
	}

	if err := s.chatLogic.DeleteSession(ctx, logic.DeleteSessionRequest{
		SessionID: sessionID,
	}); err != nil {
		return nil, mapChatError(err, "delete session")
	}

	return &ragv1.DeleteSessionResponse{
		Success: true,
	}, nil
}

// Chat is the streaming RPC. It produces a sequence of ChatResponse events
// (progress / token / citations / done) terminated by either `done` or `error`.
func (s *Server) Chat(req *ragv1.ChatRequest, stream grpc.ServerStreamingServer[ragv1.ChatResponse]) error {
	if err := s.chatLogicAvailable(); err != nil {
		return err
	}
	ctx := withTenant(stream.Context())

	sessionID, err := uuid.Parse(req.SessionId)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid session_id: %v", err)
	}

	sink := &grpcChatSink{stream: stream}
	err = s.chatLogic.StreamChat(ctx, logic.StreamChatRequest{
		SessionID: sessionID,
		Message:   req.Message,
	}, sink)
	if err != nil {
		// Translate well-known logic errors into a structured error event so
		// the client can render an inline failure rather than a transport
		// error.
		if mapped, ok := chatErrorEvent(err); ok {
			_ = stream.Send(&ragv1.ChatResponse{Event: &ragv1.ChatResponse_Error{Error: mapped}})
			return nil
		}
		return mapChatError(err, "chat")
	}
	return nil
}

// grpcChatSink adapts the streaming gRPC server to the ChatStreamSink contract.
type grpcChatSink struct {
	stream grpc.ServerStreamingServer[ragv1.ChatResponse]
}

func (g *grpcChatSink) OnProgress(event orchestrator.ProgressEvent) error {
	return g.stream.Send(&ragv1.ChatResponse{
		Event: &ragv1.ChatResponse_Progress{Progress: &ragv1.ChatProgressEvent{
			Stage:  string(event.Stage),
			Detail: event.Message,
		}},
	})
}

func (g *grpcChatSink) OnToken(token string) error {
	return g.stream.Send(&ragv1.ChatResponse{
		Event: &ragv1.ChatResponse_Token{Token: &ragv1.ChatTokenEvent{Token: token}},
	})
}

func (g *grpcChatSink) OnCitations(citations []model.ChatCitation) error {
	return g.stream.Send(&ragv1.ChatResponse{
		Event: &ragv1.ChatResponse_Citations{Citations: &ragv1.ChatCitationsEvent{
			Citations: toProtoCitations(citations),
		}},
	})
}

func (g *grpcChatSink) OnDone(assistantMessageID uuid.UUID, finishReason string) error {
	return g.stream.Send(&ragv1.ChatResponse{
		Event: &ragv1.ChatResponse_Done{Done: &ragv1.ChatDoneEvent{
			AssistantMessageId: assistantMessageID.String(),
			FinishReason:       finishReason,
		}},
	})
}

// =============================================================================
// Mappers (chat side)
// =============================================================================

func toProtoChatSession(s *model.ChatSession) *ragv1.ChatSession {
	if s == nil {
		return nil
	}
	out := &ragv1.ChatSession{
		Id:           s.ID.String(),
		TenantId:     s.TenantID.String(),
		UserId:       s.UserID.String(),
		Title:        s.Title,
		MessageCount: int32(s.MessageCount),
	}
	if !s.CreatedAt.IsZero() {
		out.CreatedAt = timestamppb.New(s.CreatedAt)
	}
	if !s.UpdatedAt.IsZero() {
		out.UpdatedAt = timestamppb.New(s.UpdatedAt)
	}
	return out
}

func toProtoChatMessage(m *model.ChatMessage) *ragv1.ChatMessage {
	if m == nil {
		return nil
	}
	out := &ragv1.ChatMessage{
		Id:        m.ID.String(),
		SessionId: m.SessionID.String(),
		TenantId:  m.TenantID.String(),
		Role:      roleToProto(m.Role),
		Content:   m.Content,
		Citations: toProtoCitations(m.Citations),
	}
	if !m.CreatedAt.IsZero() {
		out.CreatedAt = timestamppb.New(m.CreatedAt)
	}
	return out
}

func toProtoCitations(citations []model.ChatCitation) []*ragv1.Citation {
	out := make([]*ragv1.Citation, 0, len(citations))
	for _, c := range citations {
		out = append(out, &ragv1.Citation{
			DocumentId:   c.DocumentID,
			DocumentName: c.DocumentName,
			ChunkId:      c.ChunkID,
			Snippet:      c.Snippet,
			Similarity:   c.Similarity,
		})
	}
	return out
}

func roleToProto(role string) ragv1.ChatRole {
	switch role {
	case model.ChatRoleUser:
		return ragv1.ChatRole_CHAT_ROLE_USER
	case model.ChatRoleAssistant:
		return ragv1.ChatRole_CHAT_ROLE_ASSISTANT
	case model.ChatRoleSystem:
		return ragv1.ChatRole_CHAT_ROLE_SYSTEM
	default:
		return ragv1.ChatRole_CHAT_ROLE_UNSPECIFIED
	}
}

// chatErrorEvent maps a logic-layer error onto a structured ChatErrorEvent so
// the client gets a graceful in-stream failure rather than a transport error.
// Returns false when the error is not user-facing (the caller should map it to
// a gRPC status instead).
func chatErrorEvent(err error) (*ragv1.ChatErrorEvent, bool) {
	switch {
	case errors.Is(err, logic.ErrRateLimited):
		return &ragv1.ChatErrorEvent{
			Code:    "rate_limited",
			Message: "Too many chat requests. Please slow down and try again.",
		}, true
	case errors.Is(err, logic.ErrSessionLimitReached):
		return &ragv1.ChatErrorEvent{
			Code:    "session_limit",
			Message: "This conversation has reached its maximum length. Please start a new chat.",
		}, true
	case errors.Is(err, logic.ErrEmptyChatMessage):
		return &ragv1.ChatErrorEvent{
			Code:    "empty_message",
			Message: "Message cannot be empty.",
		}, true
	default:
		return nil, false
	}
}

// mapChatError translates a logic-layer error into a gRPC status code with a
// helpful message. Used by unary chat RPCs and as the fallback path for the
// streaming Chat RPC.
func mapChatError(err error, op string) error {
	switch {
	case errors.Is(err, logic.ErrTenantContextRequired):
		return status.Errorf(codes.Unauthenticated, "tenant context required")
	case errors.Is(err, logic.ErrUserContextRequired):
		return status.Errorf(codes.Unauthenticated, "user context required")
	case errors.Is(err, logic.ErrSessionNotFound):
		return status.Errorf(codes.NotFound, "chat session not found")
	case errors.Is(err, logic.ErrRateLimited):
		return status.Errorf(codes.ResourceExhausted, "rate limit exceeded")
	case errors.Is(err, logic.ErrSessionLimitReached):
		return status.Errorf(codes.FailedPrecondition, "conversation length limit reached")
	case errors.Is(err, logic.ErrEmptyChatMessage):
		return status.Errorf(codes.InvalidArgument, "message cannot be empty")
	case errors.Is(err, logic.ErrChatPipelineUnavailable):
		return status.Errorf(codes.Unavailable, "chat pipeline is not configured")
	default:
		return status.Errorf(codes.Internal, "%s: %v", op, err)
	}
}
