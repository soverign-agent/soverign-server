package repo

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sovereign-ai-compliance/rag-service/model"
	"sovereign-ai-compliance/shared/tenant"
)

func setupMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	return db, mock, func() { db.Close() }
}

func setupRegexMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New() // default QueryMatcherRegexp
	require.NoError(t, err)
	return db, mock, func() { db.Close() }
}

func expectTenantTx(mock sqlmock.Sqlmock, tenantID string) {
	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL app.current_tenant = '" + tenantID + "'").
		WillReturnResult(sqlmock.NewResult(0, 1))
}

func TestSQLRepository_CreateChatSession(t *testing.T) {
	db, mock, cleanup := setupRegexMockDB(t)
	defer cleanup()

	r := NewSQLRepository(db)
	tenantID := uuid.New()
	ctx := tenant.WithContext(context.Background(), tenantID.String())

	session := &model.ChatSession{
		ID:       uuid.New(),
		TenantID: tenantID,
		UserID:   uuid.New(),
		Title:    "First chat",
	}

	expectTenantTx(mock, tenantID.String())
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO chat_sessions")).
		WithArgs(session.ID, session.TenantID, session.UserID, session.Title).
		WillReturnRows(sqlmock.NewRows([]string{"created_at", "updated_at"}).
			AddRow(time.Now(), time.Now()))
	mock.ExpectCommit()

	err := r.CreateChatSession(ctx, session)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
	assert.False(t, session.CreatedAt.IsZero())
	assert.False(t, session.UpdatedAt.IsZero())
}

func TestSQLRepository_GetChatSessionByID_NotFound(t *testing.T) {
	db, mock, cleanup := setupRegexMockDB(t)
	defer cleanup()

	r := NewSQLRepository(db)
	tenantID := uuid.New()
	ctx := tenant.WithContext(context.Background(), tenantID.String())

	sessionID := uuid.New()
	userID := uuid.New()

	expectTenantTx(mock, tenantID.String())
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT s.id, s.tenant_id, s.user_id`)).
		WithArgs(sessionID, userID).
		WillReturnError(sql.ErrNoRows)

	_, err := r.GetChatSessionByID(ctx, sessionID, userID)
	require.ErrorIs(t, err, ErrSessionNotFound)
}

func TestSQLRepository_AppendChatMessage_PersistsAndTouchesSession(t *testing.T) {
	db, mock, cleanup := setupRegexMockDB(t)
	defer cleanup()

	r := NewSQLRepository(db)
	tenantID := uuid.New()
	ctx := tenant.WithContext(context.Background(), tenantID.String())

	msg := &model.ChatMessage{
		ID:        uuid.New(),
		SessionID: uuid.New(),
		TenantID:  tenantID,
		Role:      model.ChatRoleUser,
		Content:   "Hello",
		Citations: []model.ChatCitation{},
	}

	expectTenantTx(mock, tenantID.String())
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO chat_messages")).
		WithArgs(msg.ID, msg.SessionID, msg.TenantID, msg.Role, msg.Content, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(time.Now()))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE chat_sessions SET updated_at = NOW()")).
		WithArgs(msg.SessionID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := r.AppendChatMessage(ctx, msg)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLRepository_CountChatMessages(t *testing.T) {
	db, mock, cleanup := setupRegexMockDB(t)
	defer cleanup()

	r := NewSQLRepository(db)
	tenantID := uuid.New()
	ctx := tenant.WithContext(context.Background(), tenantID.String())

	sessionID := uuid.New()

	expectTenantTx(mock, tenantID.String())
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*) FROM chat_messages WHERE session_id`)).
		WithArgs(sessionID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(7))
	mock.ExpectCommit()

	got, err := r.CountChatMessages(ctx, sessionID)
	require.NoError(t, err)
	assert.Equal(t, 7, got)
}

func TestMarshalUnmarshalCitations(t *testing.T) {
	citations := []model.ChatCitation{
		{
			DocumentID:   "doc-1",
			DocumentName: "Annex IV.pdf",
			ChunkID:      "chunk:3",
			Snippet:      "GDPR alignment requires…",
			Similarity:   0.91,
		},
	}
	raw, err := model.MarshalCitations(citations)
	require.NoError(t, err)

	got, err := model.UnmarshalCitations(raw)
	require.NoError(t, err)
	require.Equal(t, citations, got)

	// Empty case → `[]` not null.
	emptyRaw, err := model.MarshalCitations(nil)
	require.NoError(t, err)
	assert.Equal(t, "[]", string(emptyRaw))
	emptyOut, err := model.UnmarshalCitations(emptyRaw)
	require.NoError(t, err)
	assert.Empty(t, emptyOut)
}
