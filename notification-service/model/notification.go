// Package model defines database models for the notification service.
package model

import (
	"time"

	"github.com/google/uuid"
)

// Notification represents a single notification.
type Notification struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	UserID     uuid.UUID
	Title      string
	Body       string
	Channel    string
	Priority   string
	Status     string
	ActionURL  *string
	Metadata   map[string]string
	CreatedAt  time.Time
	ReadAt     *time.Time
}

// NotificationPreference controls per-channel settings for a user.
type NotificationPreference struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TenantID  uuid.UUID
	Channel   string
	Enabled   bool
	Settings  map[string]string
	UpdatedAt time.Time
}
