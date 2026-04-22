// Package logic provides business logic for the notification service.
package logic

import (
	"context"
	"time"

	"github.com/google/uuid"
	"sovereign-ai-compliance/notification-service/model"
	"sovereign-ai-compliance/notification-service/repo"
)

// NotificationLogic handles notification business rules.
type NotificationLogic struct {
	repo repo.Repository
}

// NewNotificationLogic creates a new logic instance.
func NewNotificationLogic(r repo.Repository) *NotificationLogic {
	return &NotificationLogic{repo: r}
}

// ListNotifications returns paginated notifications.
func (l *NotificationLogic) ListNotifications(ctx context.Context, tenantID, userID uuid.UUID, status, channel string, page, pageSize int) ([]model.Notification, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return l.repo.ListNotifications(ctx, tenantID, userID, status, channel, page, pageSize)
}

// MarkAsRead marks one or all notifications as read.
func (l *NotificationLogic) MarkAsRead(ctx context.Context, tenantID, userID uuid.UUID, notificationID *uuid.UUID) (int, error) {
	return l.repo.MarkAsRead(ctx, tenantID, userID, notificationID)
}

// SendNotification creates and sends a notification.
func (l *NotificationLogic) SendNotification(ctx context.Context, tenantID uuid.UUID, n *model.Notification) error {
	n.ID = uuid.New()
	n.TenantID = tenantID
	n.Status = "unread"
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now()
	}
	return l.repo.CreateNotification(ctx, n)
}

// GetPreferences returns notification preferences.
func (l *NotificationLogic) GetPreferences(ctx context.Context, tenantID, userID uuid.UUID) ([]model.NotificationPreference, error) {
	return l.repo.GetPreferences(ctx, tenantID, userID)
}

// UpdatePreferences updates notification preferences.
func (l *NotificationLogic) UpdatePreferences(ctx context.Context, tenantID, userID uuid.UUID, prefs []model.NotificationPreference) error {
	return l.repo.UpdatePreferences(ctx, tenantID, userID, prefs)
}
