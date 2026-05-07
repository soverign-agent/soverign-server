// Package logic contains the business logic for rag-service.
package logic

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"sovereign-ai-compliance/rag-service/internal/orchestrator"
	"sovereign-ai-compliance/rag-service/model"
	"sovereign-ai-compliance/rag-service/repo"
	"sovereign-ai-compliance/shared/tenant"
)

var chatTracer = otel.Tracer("rag-service/logic/chat")

// Sentinel errors surfaced by ChatLogic. Handlers translate these to gRPC codes.
var (
	ErrUserContextRequired     = errors.New("user context required")
	ErrSessionNotFound         = errors.New("chat session not found")
	ErrSessionLimitReached     = errors.New("conversation length limit reached")
	ErrRateLimited             = errors.New("rate limit exceeded for tenant")
	ErrEmptyChatMessage        = errors.New("chat message must not be empty")
	ErrChatPipelineUnavailable = errors.New("chat pipeline is not configured")
)

// ChatPipeline abstracts the agentic RAG pipeline. The concrete implementation
// lives in the orchestrator package; ChatLogic depends on this interface so it
// can be unit-tested without invoking real LLM/vector calls.
type ChatPipeline interface {
	StreamAnswer(
		ctx context.Context,
		input orchestrator.PipelineInput,
		onDelta func(token string),
		onProgress func(event orchestrator.ProgressEvent),
	) (*orchestrator.PipelineResult, error)
}

// ChatRateLimiterConfig controls per-tenant request throttling for the
// streaming Chat RPC.
type ChatRateLimiterConfig struct {
	// RequestsPerMinute is the steady-state rate for a single tenant.
	RequestsPerMinute int
	// Burst is the maximum number of concurrent in-flight requests allowed
	// to bypass the steady rate (e.g. user smashing the send button).
	Burst int
}

// DefaultChatRateLimiterConfig returns conservative defaults appropriate for a
// shared production deployment.
func DefaultChatRateLimiterConfig() ChatRateLimiterConfig {
	return ChatRateLimiterConfig{
		RequestsPerMinute: 30,
		Burst:             5,
	}
}

// ChatLogic encapsulates session lifecycle and streaming chat orchestration.
type ChatLogic struct {
	repo     *repo.SQLRepository
	pipeline ChatPipeline
	logger   *zap.Logger

	rlCfg ChatRateLimiterConfig
	rlMu  sync.Mutex
	rl    map[string]*rate.Limiter

	maxTurns int
}

// NewChatLogic constructs a ChatLogic. pipeline may be nil; in that case the
// streaming Chat method returns ErrChatPipelineUnavailable. This keeps session
// management functional even before the orchestrator is fully wired in.
func NewChatLogic(
	repository *repo.SQLRepository,
	pipeline ChatPipeline,
	logger *zap.Logger,
	rlCfg ChatRateLimiterConfig,
) *ChatLogic {
	if rlCfg.RequestsPerMinute <= 0 {
		rlCfg.RequestsPerMinute = 30
	}
	if rlCfg.Burst <= 0 {
		rlCfg.Burst = 5
	}

	return &ChatLogic{
		repo:     repository,
		pipeline: pipeline,
		logger:   logger,
		rlCfg:    rlCfg,
		rl:       make(map[string]*rate.Limiter),
		maxTurns: model.MaxChatTurns,
	}
}

// SetMaxTurns overrides the per-session turn cap. Mainly for tests.
func (l *ChatLogic) SetMaxTurns(n int) {
	if n <= 0 {
		return
	}
	l.maxTurns = n
}

// limiterFor returns the rate.Limiter for a tenant, creating one lazily.
func (l *ChatLogic) limiterFor(tenantID string) *rate.Limiter {
	l.rlMu.Lock()
	defer l.rlMu.Unlock()

	if lim, ok := l.rl[tenantID]; ok {
		return lim
	}
	// rate.Limit(per second). RequestsPerMinute / 60 → per-second rate.
	lim := rate.NewLimiter(
		rate.Limit(float64(l.rlCfg.RequestsPerMinute)/60.0),
		l.rlCfg.Burst,
	)
	l.rl[tenantID] = lim
	return lim
}

// userIDFromContext extracts the user ID set by the gRPC interceptor (passed
// via metadata as `x-user-id`). When missing it returns ErrUserContextRequired.
func userIDFromContext(ctx context.Context) (uuid.UUID, error) {
	v := ctx.Value(userIDContextKey{})
	if v == nil {
		return uuid.Nil, ErrUserContextRequired
	}
	id, ok := v.(uuid.UUID)
	if !ok || id == uuid.Nil {
		return uuid.Nil, ErrUserContextRequired
	}
	return id, nil
}

// userIDContextKey is the context key under which the gRPC server stores the
// authenticated user ID. Defined here so logic and grpcserver share the type.
type userIDContextKey struct{}

// WithUserID returns a copy of ctx that carries userID. Used by the gRPC
// server when it parses x-user-id from incoming metadata.
func WithUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDContextKey{}, userID)
}

// CreateSessionRequest creates a chat session for the authenticated user.
type CreateSessionRequest struct {
	Title string `json:"title"`
}

// CreateSession creates a new chat session.
func (l *ChatLogic) CreateSession(ctx context.Context, req CreateSessionRequest) (*model.ChatSession, error) {
	ctx, span := chatTracer.Start(ctx, "ChatLogic.CreateSession")
	defer span.End()

	tenantIDStr, ok := tenant.FromContext(ctx)
	if !ok {
		return nil, ErrTenantContextRequired
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		return nil, fmt.Errorf("parse tenant id: %w", err)
	}

	userID, err := userIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "New Chat"
	}

	session := &model.ChatSession{
		ID:       uuid.New(),
		TenantID: tenantID,
		UserID:   userID,
		Title:    title,
	}
	if err := l.repo.CreateChatSession(ctx, session); err != nil {
		return nil, err
	}
	span.SetAttributes(
		attribute.String("tenant_id", tenantIDStr),
		attribute.String("session_id", session.ID.String()),
	)

	l.logger.Info("chat session created",
		zap.String("session_id", session.ID.String()),
		zap.String("tenant_id", tenantIDStr),
		zap.String("user_id", userID.String()),
	)
	return session, nil
}

// ListSessionsRequest lists sessions for the authenticated user.
type ListSessionsRequest struct {
	Page     int
	PageSize int
}

// ListSessionsResponse paginates chat sessions.
type ListSessionsResponse struct {
	Sessions []model.ChatSession
	Total    int
	Page     int
	PageSize int
}

// ListSessions returns paginated sessions for (tenant, user).
func (l *ChatLogic) ListSessions(ctx context.Context, req ListSessionsRequest) (*ListSessionsResponse, error) {
	if _, ok := tenant.FromContext(ctx); !ok {
		return nil, ErrTenantContextRequired
	}
	userID, err := userIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > 100 {
		req.PageSize = 20
	}

	sessions, total, err := l.repo.ListChatSessions(ctx, userID, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	return &ListSessionsResponse{
		Sessions: sessions,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// DeleteSessionRequest deletes a chat session.
type DeleteSessionRequest struct {
	SessionID uuid.UUID `json:"session_id"`
}

// GetSessionHistoryRequest fetches a paginated message history.
type GetSessionHistoryRequest struct {
	SessionID uuid.UUID
	Page      int
	PageSize  int
}

// DeleteSession removes a chat session owned by the authenticated user.
func (l *ChatLogic) DeleteSession(ctx context.Context, req DeleteSessionRequest) error {
	ctx, span := chatTracer.Start(ctx, "ChatLogic.DeleteSession")
	defer span.End()
	span.SetAttributes(attribute.String("session_id", req.SessionID.String()))

	if _, ok := tenant.FromContext(ctx); !ok {
		return ErrTenantContextRequired
	}
	userID, err := userIDFromContext(ctx)
	if err != nil {
		return err
	}

	if req.SessionID == uuid.Nil {
		return fmt.Errorf("session_id is required")
	}

	return l.repo.DeleteChatSession(ctx, req.SessionID, userID)
}

// GetSessionHistoryResponse contains the session metadata and its messages.
type GetSessionHistoryResponse struct {
	Session  *model.ChatSession
	Messages []model.ChatMessage
	Total    int
	Page     int
	PageSize int
}

// GetSessionHistory returns the message history for a session owned by the
// authenticated user. Pagination is required because long sessions can grow
// unboundedly within the per-session turn cap (each turn = 2 messages).
func (l *ChatLogic) GetSessionHistory(ctx context.Context, req GetSessionHistoryRequest) (*GetSessionHistoryResponse, error) {
	ctx, span := chatTracer.Start(ctx, "ChatLogic.GetSessionHistory")
	defer span.End()
	span.SetAttributes(
		attribute.String("session_id", req.SessionID.String()),
		attribute.Int("page", req.Page),
		attribute.Int("page_size", req.PageSize),
	)

	if _, ok := tenant.FromContext(ctx); !ok {
		return nil, ErrTenantContextRequired
	}
	userID, err := userIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if req.SessionID == uuid.Nil {
		return nil, fmt.Errorf("session_id is required")
	}
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > 200 {
		req.PageSize = 50
	}

	session, err := l.repo.GetChatSessionByID(ctx, req.SessionID, userID)
	if err != nil {
		if errors.Is(err, repo.ErrSessionNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}

	messages, total, err := l.repo.ListChatMessages(ctx, req.SessionID, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	return &GetSessionHistoryResponse{
		Session:  session,
		Messages: messages,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// StreamChatRequest carries the user's message into the streaming Chat method.
type StreamChatRequest struct {
	SessionID uuid.UUID
	Message   string
}

// ChatStreamSink receives the events produced by StreamChat in the order they
// occur. The handler is responsible for translating sink calls into protobuf
// ChatResponse messages on the gRPC stream.
type ChatStreamSink interface {
	OnProgress(event orchestrator.ProgressEvent) error
	OnToken(token string) error
	OnCitations(citations []model.ChatCitation) error
	OnDone(assistantMessageID uuid.UUID, finishReason string) error
}

// StreamChat is the streaming entry point. It enforces rate limiting and the
// per-session turn cap, persists the user message, runs the agentic pipeline
// (streaming progress + tokens via sink), and finally persists the assistant
// message with citations.
func (l *ChatLogic) StreamChat(ctx context.Context, req StreamChatRequest, sink ChatStreamSink) error {
	ctx, span := chatTracer.Start(ctx, "ChatLogic.StreamChat")
	defer span.End()
	span.SetAttributes(
		attribute.String("session_id", req.SessionID.String()),
		attribute.Int("message_length", len([]rune(req.Message))),
	)

	tenantIDStr, ok := tenant.FromContext(ctx)
	if !ok {
		return ErrTenantContextRequired
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		return fmt.Errorf("parse tenant id: %w", err)
	}
	userID, err := userIDFromContext(ctx)
	if err != nil {
		return err
	}

	if req.SessionID == uuid.Nil {
		return fmt.Errorf("session_id is required")
	}
	message := strings.TrimSpace(req.Message)
	if message == "" {
		return ErrEmptyChatMessage
	}

	// Per-tenant rate limit. Reserve(N=1) and check; do not block.
	if !l.limiterFor(tenantIDStr).AllowN(time.Now(), 1) {
		return ErrRateLimited
	}

	// Verify session ownership before doing any expensive work.
	session, err := l.repo.GetChatSessionByID(ctx, req.SessionID, userID)
	if err != nil {
		if errors.Is(err, repo.ErrSessionNotFound) {
			return ErrSessionNotFound
		}
		return err
	}

	// Enforce the conversation length cap. A "turn" is a (user, assistant)
	// pair, so the message limit is 2 * maxTurns. We count BEFORE inserting
	// the new user message so the cap is never exceeded even by one row.
	existing, err := l.repo.CountChatMessages(ctx, session.ID)
	if err != nil {
		return err
	}
	if existing >= 2*l.maxTurns {
		return ErrSessionLimitReached
	}

	// Persist the user message.
	userMsg := &model.ChatMessage{
		ID:        uuid.New(),
		SessionID: session.ID,
		TenantID:  tenantID,
		Role:      model.ChatRoleUser,
		Content:   message,
		Citations: []model.ChatCitation{},
	}
	if err := l.repo.AppendChatMessage(ctx, userMsg); err != nil {
		return fmt.Errorf("persist user message: %w", err)
	}

	// Build pipeline input: pull recent history (excluding the message we
	// just inserted; the pipeline receives the current `query` separately).
	historyMessages, err := l.repo.LoadRecentChatMessages(ctx, session.ID, 2*l.maxTurns)
	if err != nil {
		return fmt.Errorf("load history: %w", err)
	}
	history := buildOrchestratorHistory(historyMessages, userMsg.ID)

	if l.pipeline == nil {
		return ErrChatPipelineUnavailable
	}

	pipelineInput := orchestrator.PipelineInput{
		Query:     message,
		History:   history,
		TenantID:  tenantIDStr,
		UserID:    userID.String(),
		SessionID: session.ID.String(),
	}

	var assistantBuilder strings.Builder
	deltaCb := func(token string) {
		assistantBuilder.WriteString(token)
		_ = sink.OnToken(token) // sink errors abort the stream; ctx will cancel
	}
	progressCb := func(event orchestrator.ProgressEvent) {
		_ = sink.OnProgress(event)
	}

	result, err := l.pipeline.StreamAnswer(ctx, pipelineInput, deltaCb, progressCb)
	if err != nil {
		return err
	}

	// Map orchestrator citations onto our persistence shape.
	citations := citationsFromPipeline(result)
	if err := sink.OnCitations(citations); err != nil {
		return err
	}

	answer := assistantBuilder.String()
	if answer == "" && result != nil {
		answer = result.Synthesis.Answer
	}
	// Some LLM clients may return only the final aggregated content without
	// invoking the streaming delta callback. Emit that text once so the client
	// still receives assistant content before the terminal `done` event.
	if assistantBuilder.Len() == 0 && answer != "" {
		if err := sink.OnToken(answer); err != nil {
			return err
		}
	}

	assistantMsg := &model.ChatMessage{
		ID:        uuid.New(),
		SessionID: session.ID,
		TenantID:  tenantID,
		Role:      model.ChatRoleAssistant,
		Content:   answer,
		Citations: citations,
	}
	if err := l.repo.AppendChatMessage(ctx, assistantMsg); err != nil {
		return fmt.Errorf("persist assistant message: %w", err)
	}

	// Auto-title the session from the first user message if it's still the default.
	if strings.EqualFold(strings.TrimSpace(session.Title), "New Chat") {
		newTitle := truncateTitle(message, 80)
		if err := l.repo.TouchChatSession(ctx, session.ID, newTitle); err != nil {
			l.logger.Warn("touch chat session title failed",
				zap.String("session_id", session.ID.String()),
				zap.Error(err))
		}
	}

	finishReason := "stop"
	if result != nil && result.Error != "" {
		finishReason = "error"
	}
	return sink.OnDone(assistantMsg.ID, finishReason)
}

// buildOrchestratorHistory converts persisted ChatMessage rows into the
// orchestrator's lighter ChatMessage shape. The user message that triggered
// the current call is excluded so the pipeline receives the question only via
// `query`.
func buildOrchestratorHistory(messages []model.ChatMessage, currentUserMsgID uuid.UUID) []orchestrator.ChatMessage {
	out := make([]orchestrator.ChatMessage, 0, len(messages))
	for _, m := range messages {
		if m.ID == currentUserMsgID {
			continue
		}
		out = append(out, orchestrator.ChatMessage{
			Role:      m.Role,
			Content:   m.Content,
			Timestamp: m.CreatedAt,
		})
	}
	return out
}

// citationsFromPipeline maps the orchestrator's citation type onto the
// persistence shape. We pull a short snippet (≤240 chars) so the column does
// not balloon.
func citationsFromPipeline(result *orchestrator.PipelineResult) []model.ChatCitation {
	if result == nil {
		return []model.ChatCitation{}
	}
	out := make([]model.ChatCitation, 0, len(result.Synthesis.Citations))
	for _, c := range result.Synthesis.Citations {
		out = append(out, model.ChatCitation{
			DocumentID:   c.DocumentID.String(),
			DocumentName: c.DocumentName,
			ChunkID:      fmt.Sprintf("chunk:%d", c.ChunkIndex),
			Snippet:      truncateTitle(c.Text, 240),
			Similarity:   c.Similarity,
		})
	}
	return out
}

// truncateTitle returns s shortened to maxRunes with an ellipsis. It is rune-
// safe so multi-byte content (e.g. Chinese) is not chopped mid-codepoint.
func truncateTitle(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	if maxRunes <= 0 || len(s) == 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes-1]) + "…"
}
