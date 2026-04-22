// Package repo provides repository interfaces for the notification service.
package repo

import (
	"context"

	"github.com/google/uuid"
	"sovereign-ai-compliance/notification-service/model"
)

// Repository defines notification data access.
type Repository interface {
	ListNotifications(ctx context.Context, tenantID, userID uuid.UUID, status, channel string, page, pageSize int) ([]model.Notification, int, error)
	MarkAsRead(ctx context.Context, tenantID, userID uuid.UUID, notificationID *uuid.UUID) (int, error)
	CreateNotification(ctx context.Context, n *model.Notification) error
	GetPreferences(ctx context.Context, tenantID, userID uuid.UUID) ([]model.NotificationPreference, error)
	UpdatePreferences(ctx context.Context, tenantID, userID uuid.UUID, prefs []model.NotificationPreference) error
}

// InMemoryRepository is a stub in-memory implementation.
type InMemoryRepository struct{}

// NewInMemoryRepository creates a new in-memory repository.
func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{}
}

// ListNotifications returns empty results.
func (r *InMemoryRepository) ListNotifications(ctx context.Context, tenantID, userID uuid.UUID, status, channel string, page, pageSize int) ([]model.Notification, int, error) {
	return nil, 0, nil
}

// MarkAsRead does nothing.
func (r *InMemoryRepository) MarkAsRead(ctx context.Context, tenantID, userID uuid.UUID, notificationID *uuid.UUID) (int, error) {
	return 0, nil
}

// CreateNotification does nothing.
func (r *InMemoryRepository) CreateNotification(ctx context.Context, n *model.Notification) error {
	return nil
}

// GetPreferences returns empty results.
func (r *InMemoryRepository) GetPreferences(ctx context.Context, tenantID, userID uuid.UUID) ([]model.NotificationPreference, error) {
	return nil, nil
}

// UpdatePreferences does nothing.
func (r *InMemoryRepository) UpdatePreferences(ctx context.Context, tenantID, userID uuid.UUID, prefs []model.NotificationPreference) error {
	return nil
}
