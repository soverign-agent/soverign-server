// Package repo provides data access layer for rag-service with RLS support.
package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"sovereign-ai-compliance/rag-service/model"
)

// ErrSessionNotFound is returned when a chat session does not exist or is not
// visible to the current tenant.
var ErrSessionNotFound = errors.New("chat session not found")

// CreateChatSession inserts a new session for the current tenant/user.
func (r *SQLRepository) CreateChatSession(ctx context.Context, session *model.ChatSession) error {
	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	const query = `
		INSERT INTO chat_sessions (id, tenant_id, user_id, title)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at, updated_at`

	if err := tx.QueryRowContext(
		ctx, query,
		session.ID, session.TenantID, session.UserID, session.Title,
	).Scan(&session.CreatedAt, &session.UpdatedAt); err != nil {
		return fmt.Errorf("insert chat session: %w", err)
	}

	return tx.Commit()
}

// GetChatSessionByID returns a single session, or ErrSessionNotFound when the
// session does not exist or is not owned by the current tenant.
// The optional userID, when non-zero, additionally restricts visibility to a
// specific user within the tenant (sessions are user-private).
func (r *SQLRepository) GetChatSessionByID(ctx context.Context, sessionID uuid.UUID, userID uuid.UUID) (*model.ChatSession, error) {
	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	var (
		session model.ChatSession
		query   = `
			SELECT s.id, s.tenant_id, s.user_id, s.title, s.created_at, s.updated_at,
			       COALESCE((SELECT COUNT(*) FROM chat_messages m WHERE m.session_id = s.id), 0)
			FROM chat_sessions s
			WHERE s.id = $1`
		args = []interface{}{sessionID}
	)
	if userID != uuid.Nil {
		query += " AND s.user_id = $2"
		args = append(args, userID)
	}

	row := tx.QueryRowContext(ctx, query, args...)
	err = row.Scan(
		&session.ID, &session.TenantID, &session.UserID, &session.Title,
		&session.CreatedAt, &session.UpdatedAt, &session.MessageCount,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get chat session: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit chat session read: %w", err)
	}
	return &session, nil
}

// ListChatSessions returns sessions for the (tenant, user) ordered by most
// recently updated. When userID is uuid.Nil only the tenant scope is applied.
func (r *SQLRepository) ListChatSessions(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]model.ChatSession, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	whereClause := ""
	args := []interface{}{}
	if userID != uuid.Nil {
		whereClause = "WHERE user_id = $1"
		args = append(args, userID)
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM chat_sessions " + whereClause
	if err := tx.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count chat sessions: %w", err)
	}

	listQuery := `
		SELECT s.id, s.tenant_id, s.user_id, s.title, s.created_at, s.updated_at,
		       COALESCE((SELECT COUNT(*) FROM chat_messages m WHERE m.session_id = s.id), 0)
		FROM chat_sessions s ` + whereClause + `
		ORDER BY s.updated_at DESC
		LIMIT $` + fmt.Sprintf("%d", len(args)+1) +
		` OFFSET $` + fmt.Sprintf("%d", len(args)+2)

	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := tx.QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list chat sessions: %w", err)
	}
	defer rows.Close()

	sessions := make([]model.ChatSession, 0, pageSize)
	for rows.Next() {
		var s model.ChatSession
		if err := rows.Scan(
			&s.ID, &s.TenantID, &s.UserID, &s.Title,
			&s.CreatedAt, &s.UpdatedAt, &s.MessageCount,
		); err != nil {
			return nil, 0, fmt.Errorf("scan chat session: %w", err)
		}
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate chat sessions: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, 0, fmt.Errorf("commit chat session list: %w", err)
	}
	return sessions, total, nil
}

// TouchChatSession bumps updated_at on the session so list ordering stays
// fresh. Title can also be updated when non-empty.
func (r *SQLRepository) TouchChatSession(ctx context.Context, sessionID uuid.UUID, newTitle string) error {
	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	if newTitle == "" {
		if _, err := tx.ExecContext(ctx,
			`UPDATE chat_sessions SET updated_at = NOW() WHERE id = $1`,
			sessionID,
		); err != nil {
			return fmt.Errorf("touch chat session: %w", err)
		}
	} else {
		if _, err := tx.ExecContext(ctx,
			`UPDATE chat_sessions SET title = $1, updated_at = NOW() WHERE id = $2`,
			newTitle, sessionID,
		); err != nil {
			return fmt.Errorf("touch chat session title: %w", err)
		}
	}

	return tx.Commit()
}

// CountChatMessages returns the total number of messages in a session.
func (r *SQLRepository) CountChatMessages(ctx context.Context, sessionID uuid.UUID) (int, error) {
	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	var total int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM chat_messages WHERE session_id = $1`,
		sessionID,
	).Scan(&total); err != nil {
		return 0, fmt.Errorf("count chat messages: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit chat message count: %w", err)
	}
	return total, nil
}

// AppendChatMessage inserts a message into chat_messages and bumps the parent
// session's updated_at in the same transaction so list ordering reflects the
// new activity.
func (r *SQLRepository) AppendChatMessage(ctx context.Context, msg *model.ChatMessage) error {
	citationsJSON, err := model.MarshalCitations(msg.Citations)
	if err != nil {
		return fmt.Errorf("marshal citations: %w", err)
	}

	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	const insertQuery = `
		INSERT INTO chat_messages (id, session_id, tenant_id, role, content, citations)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb)
		RETURNING created_at`

	if err := tx.QueryRowContext(
		ctx, insertQuery,
		msg.ID, msg.SessionID, msg.TenantID, msg.Role, msg.Content, citationsJSON,
	).Scan(&msg.CreatedAt); err != nil {
		return fmt.Errorf("insert chat message: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE chat_sessions SET updated_at = NOW() WHERE id = $1`,
		msg.SessionID,
	); err != nil {
		return fmt.Errorf("touch chat session on append: %w", err)
	}

	return tx.Commit()
}

// ListChatMessages returns messages for a session ordered chronologically. The
// caller is responsible for verifying that the session is visible to the user
// before calling this method (see GetChatSessionByID).
func (r *SQLRepository) ListChatMessages(ctx context.Context, sessionID uuid.UUID, page, pageSize int) ([]model.ChatMessage, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}

	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	var total int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM chat_messages WHERE session_id = $1`,
		sessionID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count chat messages: %w", err)
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT id, session_id, tenant_id, role, content, citations, created_at
		FROM chat_messages
		WHERE session_id = $1
		ORDER BY created_at ASC, id ASC
		LIMIT $2 OFFSET $3`,
		sessionID, pageSize, (page-1)*pageSize,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list chat messages: %w", err)
	}
	defer rows.Close()

	out := make([]model.ChatMessage, 0, pageSize)
	for rows.Next() {
		var (
			m       model.ChatMessage
			rawJSON []byte
		)
		if err := rows.Scan(
			&m.ID, &m.SessionID, &m.TenantID, &m.Role, &m.Content, &rawJSON, &m.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan chat message: %w", err)
		}
		citations, err := model.UnmarshalCitations(rawJSON)
		if err != nil {
			return nil, 0, fmt.Errorf("decode citations: %w", err)
		}
		m.Citations = citations
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate chat messages: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, 0, fmt.Errorf("commit chat message list: %w", err)
	}
	return out, total, nil
}

// LoadRecentChatMessages returns the last `limit` messages for a session in
// chronological order. Used to construct the LLM prompt context. Limit is
// clamped to [1, 50].
func (r *SQLRepository) LoadRecentChatMessages(ctx context.Context, sessionID uuid.UUID, limit int) ([]model.ChatMessage, error) {
	if limit < 1 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
		SELECT id, session_id, tenant_id, role, content, citations, created_at
		FROM (
		  SELECT id, session_id, tenant_id, role, content, citations, created_at
		  FROM chat_messages
		  WHERE session_id = $1
		  ORDER BY created_at DESC, id DESC
		  LIMIT $2
		) recent
		ORDER BY created_at ASC, id ASC`,
		sessionID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("load recent chat messages: %w", err)
	}
	defer rows.Close()

	out := make([]model.ChatMessage, 0, limit)
	for rows.Next() {
		var (
			m       model.ChatMessage
			rawJSON []byte
		)
		if err := rows.Scan(
			&m.ID, &m.SessionID, &m.TenantID, &m.Role, &m.Content, &rawJSON, &m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan recent chat message: %w", err)
		}
		citations, err := model.UnmarshalCitations(rawJSON)
		if err != nil {
			return nil, fmt.Errorf("decode citations: %w", err)
		}
		m.Citations = citations
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent chat messages: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit recent chat read: %w", err)
	}
	return out, nil
}

// DeleteChatSession removes a chat session and its messages (cascading) after
// verifying that the session exists and is owned by the given user.
func (r *SQLRepository) DeleteChatSession(ctx context.Context, sessionID uuid.UUID, userID uuid.UUID) error {
	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tenant tx: %w", err)
	}
	defer tx.Rollback()

	var exists bool
	var query string
	var args []interface{}

	if userID != uuid.Nil {
		query = `SELECT EXISTS(SELECT 1 FROM chat_sessions WHERE id = $1 AND user_id = $2)`
		args = []interface{}{sessionID, userID}
	} else {
		query = `SELECT EXISTS(SELECT 1 FROM chat_sessions WHERE id = $1)`
		args = []interface{}{sessionID}
	}

	if err := tx.QueryRowContext(ctx, query, args...).Scan(&exists); err != nil {
		return fmt.Errorf("check session existence: %w", err)
	}
	if !exists {
		return ErrSessionNotFound
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM chat_sessions WHERE id = $1`, sessionID); err != nil {
		return fmt.Errorf("delete chat session: %w", err)
	}

	return tx.Commit()
}
