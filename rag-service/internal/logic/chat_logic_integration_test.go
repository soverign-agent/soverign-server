//go:build integration

// Package logic_integration_test exercises the chat handlers against a real
// PostgreSQL database. It is excluded from the default `go test` run via the
// `integration` build tag and the RAG_SERVICE_DB_DSN environment variable so
// CI workflows that don't have PostgreSQL available are not affected.
//
// To run locally:
//
//	docker-compose -f server/deploy/docker-compose.yml up -d postgres
//	export RAG_SERVICE_DB_DSN="host=localhost port=5432 user=sovereign \
//	    password=sovereign-dev-password dbname=sovereign sslmode=disable"
//	go test -tags integration ./rag-service/internal/logic/...
package logic_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"sovereign-ai-compliance/rag-service/internal/logic"
	"sovereign-ai-compliance/rag-service/internal/orchestrator"
	"sovereign-ai-compliance/rag-service/model"
	"sovereign-ai-compliance/rag-service/repo"
	"sovereign-ai-compliance/shared/tenant"
)

type stubPipeline struct {
	tokens    []string
	citations []orchestrator.Citation
}

func (p *stubPipeline) StreamAnswer(
	ctx context.Context,
	in orchestrator.PipelineInput,
	onDelta func(token string),
	onProgress func(event orchestrator.ProgressEvent),
) (*orchestrator.PipelineResult, error) {
	if onProgress != nil {
		onProgress(orchestrator.ProgressEvent{Stage: orchestrator.StageRetrieving, Message: "stub retrieval"})
	}
	for _, t := range p.tokens {
		if onDelta != nil {
			onDelta(t)
		}
	}
	answer := ""
	for _, t := range p.tokens {
		answer += t
	}
	return &orchestrator.PipelineResult{
		Synthesis: orchestrator.SynthesisResult{Answer: answer, Citations: p.citations},
	}, nil
}

type integrationSink struct {
	progress  []orchestrator.ProgressEvent
	tokens    []string
	citations []model.ChatCitation
	doneID    uuid.UUID
	finish    string
}

func (s *integrationSink) OnProgress(e orchestrator.ProgressEvent) error {
	s.progress = append(s.progress, e)
	return nil
}
func (s *integrationSink) OnToken(t string) error { s.tokens = append(s.tokens, t); return nil }
func (s *integrationSink) OnCitations(c []model.ChatCitation) error {
	s.citations = append([]model.ChatCitation{}, c...)
	return nil
}
func (s *integrationSink) OnDone(id uuid.UUID, fr string) error {
	s.doneID = id
	s.finish = fr
	return nil
}

// applyChatMigration runs the up.sql against the connected database.
func applyChatMigration(t *testing.T, db *sql.DB) {
	t.Helper()
	migPath := filepath.Join("..", "..", "migrations", "006_chat_sessions_messages.up.sql")
	raw, err := os.ReadFile(migPath)
	require.NoError(t, err)
	_, err = db.Exec(string(raw))
	require.NoError(t, err, "apply chat migration")
}

// seedTenantAndUser creates a tenant + user row so the foreign keys on
// chat_sessions/chat_messages can be satisfied.
func seedTenantAndUser(t *testing.T, db *sql.DB) (uuid.UUID, uuid.UUID) {
	t.Helper()
	tenantID := uuid.New()
	userID := uuid.New()
	_, err := db.Exec(
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3)`,
		tenantID, "Integration Tenant "+tenantID.String()[:8], "it-"+tenantID.String()[:8],
	)
	require.NoError(t, err)
	_, err = db.Exec(
		`INSERT INTO users (id, tenant_id, email, password_hash, role)
		 VALUES ($1, $2, $3, $4, $5)`,
		userID, tenantID, "user-"+userID.String()[:8]+"@example.test", "x", "viewer",
	)
	require.NoError(t, err)
	return tenantID, userID
}

// TestChatLogic_FullPipeline_Integration drives the full chat surface
// (CreateSession → StreamChat → GetSessionHistory) against a real DB.
func TestChatLogic_FullPipeline_Integration(t *testing.T) {
	dsn := os.Getenv("RAG_SERVICE_DB_DSN")
	if dsn == "" {
		t.Skip("RAG_SERVICE_DB_DSN not set; skipping integration test")
	}

	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Ping())

	applyChatMigration(t, db)
	tenantID, userID := seedTenantAndUser(t, db)
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM tenants WHERE id = $1`, tenantID)
	})

	repository := repo.NewSQLRepository(db)
	pipeline := &stubPipeline{
		tokens: []string{"Hello", " ", "user."},
		citations: []orchestrator.Citation{
			{
				ChunkIndex:   1,
				DocumentID:   uuid.New(),
				DocumentName: "AnnexIV.pdf",
				Text:         "Article 11 mandates technical documentation",
				Similarity:   0.92,
			},
		},
	}
	cl := logic.NewChatLogic(repository, pipeline, zap.NewNop(), logic.DefaultChatRateLimiterConfig())

	ctx := tenant.WithContext(context.Background(), tenantID.String())
	ctx = logic.WithUserID(ctx, userID)

	// Create session.
	session, err := cl.CreateSession(ctx, logic.CreateSessionRequest{Title: ""})
	require.NoError(t, err)
	assert.Equal(t, "New Chat", session.Title)
	assert.Equal(t, tenantID, session.TenantID)
	assert.Equal(t, userID, session.UserID)

	// Stream a chat turn.
	sink := &integrationSink{}
	require.NoError(t, cl.StreamChat(ctx, logic.StreamChatRequest{
		SessionID: session.ID,
		Message:   "Summarize Annex IV requirements.",
	}, sink))
	assert.NotEmpty(t, sink.tokens, "tokens forwarded to sink")
	assert.NotEmpty(t, sink.citations, "citations forwarded to sink")
	assert.Equal(t, "stop", sink.finish)

	// Verify both messages persisted in chronological order.
	history, err := cl.GetSessionHistory(ctx, logic.GetSessionHistoryRequest{
		SessionID: session.ID,
		PageSize:  20,
	})
	require.NoError(t, err)
	require.Len(t, history.Messages, 2)
	assert.Equal(t, model.ChatRoleUser, history.Messages[0].Role)
	assert.Equal(t, "Summarize Annex IV requirements.", history.Messages[0].Content)
	assert.Equal(t, model.ChatRoleAssistant, history.Messages[1].Role)
	assert.Contains(t, history.Messages[1].Content, "Hello")
	require.Len(t, history.Messages[1].Citations, 1)
	assert.Equal(t, "AnnexIV.pdf", history.Messages[1].Citations[0].DocumentName)

	// Auto-titling kicked in: the session title should now reflect the first user message.
	updated, err := cl.GetSessionHistory(ctx, logic.GetSessionHistoryRequest{
		SessionID: session.ID,
		PageSize:  1,
	})
	require.NoError(t, err)
	assert.NotEqual(t, "New Chat", updated.Session.Title, "auto-title should override default")
}

// TestChatLogic_TenantIsolation_Integration confirms that a different tenant
// cannot read another tenant's sessions even when they guess the session UUID.
func TestChatLogic_TenantIsolation_Integration(t *testing.T) {
	dsn := os.Getenv("RAG_SERVICE_DB_DSN")
	if dsn == "" {
		t.Skip("RAG_SERVICE_DB_DSN not set; skipping integration test")
	}

	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Ping())

	applyChatMigration(t, db)
	tenantA, userA := seedTenantAndUser(t, db)
	tenantB, userB := seedTenantAndUser(t, db)
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM tenants WHERE id IN ($1, $2)`, tenantA, tenantB)
	})

	repository := repo.NewSQLRepository(db)
	cl := logic.NewChatLogic(repository, &stubPipeline{tokens: []string{"ok"}}, zap.NewNop(), logic.DefaultChatRateLimiterConfig())

	// Tenant A creates a session.
	ctxA := logic.WithUserID(tenant.WithContext(context.Background(), tenantA.String()), userA)
	sessionA, err := cl.CreateSession(ctxA, logic.CreateSessionRequest{Title: "Private to A"})
	require.NoError(t, err)

	// Tenant B tries to read Tenant A's session — RLS should hide it entirely.
	ctxB := logic.WithUserID(tenant.WithContext(context.Background(), tenantB.String()), userB)
	_, err = cl.GetSessionHistory(ctxB, logic.GetSessionHistoryRequest{
		SessionID: sessionA.ID,
	})
	require.ErrorIs(t, err, logic.ErrSessionNotFound)
}
