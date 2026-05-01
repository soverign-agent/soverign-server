// Package repo provides PostgreSQL-backed notification storage.
package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"sovereign-ai-compliance/notification-service/model"
	"sovereign-ai-compliance/shared/database"
	"sovereign-ai-compliance/shared/tenant"
)

// PostgresRepository implements Repository against the notification_events table.
type PostgresRepository struct {
	base *database.BaseRepository
}

// NewPostgresRepository creates a real PostgreSQL repository.
func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{
		base: database.NewBaseRepository(db),
	}
}

// InitSchema ensures the notification_events table has all columns needed by the model.
func (r *PostgresRepository) InitSchema(ctx context.Context) error {
	_, err := r.base.DB().ExecContext(ctx, `
		ALTER TABLE notification_events
		ADD COLUMN IF NOT EXISTS read_at TIMESTAMPTZ,
		ADD COLUMN IF NOT EXISTS priority VARCHAR(50) DEFAULT 'NORMAL',
		ADD COLUMN IF NOT EXISTS action_url VARCHAR(500);
	`)
	if err != nil {
		return fmt.Errorf("init schema: %w", err)
	}
	return nil
}

// CreateNotification inserts a new notification row.
func (r *PostgresRepository) CreateNotification(ctx context.Context, n *model.Notification) error {
	tenantID, ok := tenant.FromContext(ctx)
	if !ok || tenantID == "" {
		return fmt.Errorf("tenant context required")
	}

	metaJSON, err := json.Marshal(n.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	query := `
		INSERT INTO notification_events (
			id, tenant_id, event_type, channel, recipient, subject, body,
			status, priority, action_url, metadata, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	_, err = r.base.DB().ExecContext(ctx, query,
		n.ID,
		tenantID,
		eventTypeFromMetadata(n.Metadata),
		n.Channel,
		n.UserID.String(),
		n.Title,
		n.Body,
		mapStatusToDB(n.Status),
		n.Priority,
		nullString(n.ActionURL),
		metaJSON,
		n.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert notification: %w", err)
	}
	return nil
}

// ListNotifications returns paginated notifications for a tenant/user.
func (r *PostgresRepository) ListNotifications(ctx context.Context, tenantID, userID uuid.UUID, status, channel string, page, pageSize int) ([]model.Notification, int, error) {
	tid, ok := tenant.FromContext(ctx)
	if !ok || tid == "" {
		return nil, 0, fmt.Errorf("tenant context required")
	}

	tx, err := r.base.BeginTenantTx(ctx, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	where := "WHERE tenant_id = $1"
	args := []any{tenantID.String()}
	argIdx := 2

	if userID != uuid.Nil {
		where += fmt.Sprintf(" AND recipient = $%d", argIdx)
		args = append(args, userID.String())
		argIdx++
	}

	if status != "" {
		where += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}
	if channel != "" {
		where += fmt.Sprintf(" AND channel = $%d", argIdx)
		args = append(args, channel)
		argIdx++
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM notification_events " + where
	if err := tx.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count notifications: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT id, tenant_id, channel, recipient, subject, body,
		       status, priority, action_url, metadata, created_at, read_at
		FROM notification_events
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()

	var items []model.Notification
	for rows.Next() {
		var n model.Notification
		var actionURL sql.NullString
		var readAt sql.NullTime
		var metaBytes []byte
		var recipient string

		err := rows.Scan(
			&n.ID, &n.TenantID, &n.Channel, &recipient, &n.Title, &n.Body,
			&n.Status, &n.Priority, &actionURL, &metaBytes, &n.CreatedAt, &readAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan notification: %w", err)
		}

		n.UserID, _ = uuid.Parse(recipient)
		n.Status = mapStatusFromDB(n.Status)
		if actionURL.Valid {
			n.ActionURL = &actionURL.String
		}
		if readAt.Valid {
			n.ReadAt = &readAt.Time
		}
		if len(metaBytes) > 0 {
			_ = json.Unmarshal(metaBytes, &n.Metadata)
		}
		items = append(items, n)
	}

	_ = tx.Commit()
	return items, total, nil
}

// MarkAsRead marks one or all notifications as read.
func (r *PostgresRepository) MarkAsRead(ctx context.Context, tenantID, userID uuid.UUID, notificationID *uuid.UUID) (int, error) {
	tid, ok := tenant.FromContext(ctx)
	if !ok || tid == "" {
		return 0, fmt.Errorf("tenant context required")
	}

	query := `UPDATE notification_events SET status = 'sent', read_at = NOW() WHERE tenant_id = $1`
	args := []any{tenantID.String()}
	argIdx := 2

	if userID != uuid.Nil {
		query += fmt.Sprintf(" AND recipient = $%d", argIdx)
		args = append(args, userID.String())
		argIdx++
	}

	if notificationID != nil {
		query += fmt.Sprintf(" AND id = $%d", argIdx)
		args = append(args, *notificationID)
		argIdx++
	} else {
		query += " AND read_at IS NULL"
	}

	res, err := r.base.DB().ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("mark as read: %w", err)
	}
	count, _ := res.RowsAffected()
	return int(count), nil
}

// GetPreferences returns empty preferences (placeholder until preferences table exists).
func (r *PostgresRepository) GetPreferences(ctx context.Context, tenantID, userID uuid.UUID) ([]model.NotificationPreference, error) {
	return []model.NotificationPreference{}, nil
}

// UpdatePreferences is a no-op placeholder.
func (r *PostgresRepository) UpdatePreferences(ctx context.Context, tenantID, userID uuid.UUID, prefs []model.NotificationPreference) error {
	return nil
}

func eventTypeFromMetadata(m map[string]string) string {
	if et := m["event_type"]; et != "" {
		return et
	}
	return "notification"
}

func mapStatusToDB(status string) string {
	switch status {
	case "unread":
		return "pending"
	case "read":
		return "sent"
	default:
		return status
	}
}

func mapStatusFromDB(status string) string {
	switch status {
	case "pending":
		return "unread"
	case "sent":
		return "read"
	default:
		return status
	}
}

func nullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: *s, Valid: true}
}
