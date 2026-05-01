// Package grpcserver provides the gRPC server implementation for notification-service.
package grpcserver

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"sovereign-ai-compliance/notification-service/internal/logic"
	"sovereign-ai-compliance/notification-service/model"
	notificationv1 "sovereign-ai-compliance/shared/proto/notification/v1"
	"sovereign-ai-compliance/shared/tenant"
)

// Server implements notificationv1.NotificationServiceServer.
type Server struct {
	notificationv1.UnimplementedNotificationServiceServer

	logic *logic.NotificationLogic
}

// NewServer creates a new gRPC server for notification-service.
func NewServer(logic *logic.NotificationLogic) *Server {
	return &Server{
		logic: logic,
	}
}

// Register registers the server on the provided gRPC server.
func (s *Server) Register(grpcServer *grpc.Server) {
	notificationv1.RegisterNotificationServiceServer(grpcServer, s)
}

// withTenant extracts tenant ID from gRPC metadata and injects it into context.
func withTenant(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	vals := md.Get("x-tenant-id")
	if len(vals) > 0 && vals[0] != "" {
		return tenant.WithContext(ctx, vals[0])
	}
	return ctx
}

// tenantFromCtx parses the tenant UUID from the context.
func tenantFromCtx(ctx context.Context) (uuid.UUID, error) {
	tid, ok := tenant.FromContext(ctx)
	if !ok || tid == "" {
		return uuid.Nil, status.Error(codes.Unauthenticated, "tenant context required")
	}
	return uuid.Parse(tid)
}

// SendNotification creates and sends a notification.
func (s *Server) SendNotification(ctx context.Context, req *notificationv1.SendNotificationRequest) (*notificationv1.SendNotificationResponse, error) {
	ctx = withTenant(ctx)

	tenantID, err := tenantFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id: %v", err)
	}

	n := &model.Notification{
		UserID:   userID,
		Title:    req.Title,
		Body:     req.Body,
		Channel:  req.Channel.String(),
		Priority: req.Priority.String(),
		Metadata: req.Metadata,
	}
	if req.ActionUrl != "" {
		n.ActionURL = &req.ActionUrl
	}

	if err := s.logic.SendNotification(ctx, tenantID, n); err != nil {
		return nil, status.Errorf(codes.Internal, "send notification: %v", err)
	}

	return &notificationv1.SendNotificationResponse{
		NotificationId: n.ID.String(),
	}, nil
}

// ListNotifications returns paginated notifications.
func (s *Server) ListNotifications(ctx context.Context, req *notificationv1.ListNotificationsRequest) (*notificationv1.ListNotificationsResponse, error) {
	ctx = withTenant(ctx)

	tenantID, err := tenantFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	// For now, list all notifications for the tenant.  If a user_id filter is
	// needed it can be extracted from the request or JWT context later.
	channelFilter := req.Channel.String()
	if req.Channel == notificationv1.NotificationChannel_NOTIFICATION_CHANNEL_UNSPECIFIED {
		channelFilter = ""
	}
	items, total, err := s.logic.ListNotifications(ctx, tenantID, uuid.Nil, req.Status, channelFilter, int(req.Page), int(req.PageSize))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list notifications: %v", err)
	}

	protoItems := make([]*notificationv1.Notification, 0, len(items))
	var unreadCount int32
	for _, n := range items {
		protoItems = append(protoItems, mapNotificationToProto(n))
		if n.Status == "unread" {
			unreadCount++
		}
	}

	pages := 0
	if req.PageSize > 0 {
		pages = (total + int(req.PageSize) - 1) / int(req.PageSize)
	}

	return &notificationv1.ListNotificationsResponse{
		Items:       protoItems,
		Total:       int32(total),
		Page:        req.Page,
		Pages:       int32(pages),
		UnreadCount: unreadCount,
	}, nil
}

// MarkRead marks one or all notifications as read.
func (s *Server) MarkRead(ctx context.Context, req *notificationv1.MarkReadRequest) (*notificationv1.MarkReadResponse, error) {
	ctx = withTenant(ctx)

	tenantID, err := tenantFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	var nid *uuid.UUID
	if req.NotificationId != "" {
		parsed, err := uuid.Parse(req.NotificationId)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid notification_id: %v", err)
		}
		nid = &parsed
	}

	// List all notifications for the tenant to mark them.
	marked, err := s.logic.MarkAsRead(ctx, tenantID, uuid.Nil, nid)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "mark read: %v", err)
	}

	return &notificationv1.MarkReadResponse{
		Success:     true,
		MarkedCount: int32(marked),
	}, nil
}

// GetPreferences returns notification preferences.
func (s *Server) GetPreferences(ctx context.Context, req *notificationv1.GetPreferencesRequest) (*notificationv1.GetPreferencesResponse, error) {
	ctx = withTenant(ctx)

	tenantID, err := tenantFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	prefs, err := s.logic.GetPreferences(ctx, tenantID, uuid.Nil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get preferences: %v", err)
	}

	protoPrefs := make([]*notificationv1.NotificationPreference, 0, len(prefs))
	for _, p := range prefs {
		protoPrefs = append(protoPrefs, &notificationv1.NotificationPreference{
			UserId:    p.UserID.String(),
			TenantId:  p.TenantID.String(),
			Channel:   mapChannelProto(p.Channel),
			Enabled:   p.Enabled,
			Settings:  p.Settings,
			UpdatedAt: timestamppb.New(p.UpdatedAt),
		})
	}

	return &notificationv1.GetPreferencesResponse{
		Preferences: protoPrefs,
	}, nil
}

// UpdatePreferences updates notification preferences.
func (s *Server) UpdatePreferences(ctx context.Context, req *notificationv1.UpdatePreferencesRequest) (*notificationv1.UpdatePreferencesResponse, error) {
	ctx = withTenant(ctx)

	tenantID, err := tenantFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	prefs := make([]model.NotificationPreference, 0, len(req.Preferences))
	for _, p := range req.Preferences {
		uid, _ := uuid.Parse(p.UserId)
		tid, _ := uuid.Parse(p.TenantId)
		prefs = append(prefs, model.NotificationPreference{
			UserID:   uid,
			TenantID: tid,
			Channel:  p.Channel.String(),
			Enabled:  p.Enabled,
			Settings: p.Settings,
		})
	}

	if err := s.logic.UpdatePreferences(ctx, tenantID, uuid.Nil, prefs); err != nil {
		return nil, status.Errorf(codes.Internal, "update preferences: %v", err)
	}

	return &notificationv1.UpdatePreferencesResponse{
		Success: true,
	}, nil
}

// Subscribe streams real-time notification events.
func (s *Server) Subscribe(req *notificationv1.SubscribeRequest, stream notificationv1.NotificationService_SubscribeServer) error {
	ctx := withTenant(stream.Context())
	_ = ctx

	// Send a connected event immediately so grpcurl shows output.
	if err := stream.Send(&notificationv1.SubscribeEvent{
		Notification: &notificationv1.Notification{
			Id:    "connected",
			Title: "Stream connected",
			Body:  "Waiting for notifications...",
		},
	}); err != nil {
		return status.Errorf(codes.Internal, "send connected event: %v", err)
	}

	// Keep stream open with periodic heartbeats until client disconnects.
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stream.Context().Done():
			return nil
		case <-ticker.C:
			if err := stream.Send(&notificationv1.SubscribeEvent{
				Notification: &notificationv1.Notification{
					Id:    "heartbeat",
					Title: "Heartbeat",
					Body:  "Connection alive",
				},
			}); err != nil {
				return nil // client disconnected
			}
		}
	}
}

func mapNotificationToProto(n model.Notification) *notificationv1.Notification {
	pn := &notificationv1.Notification{
		Id:        n.ID.String(),
		TenantId:  n.TenantID.String(),
		UserId:    n.UserID.String(),
		Title:     n.Title,
		Body:      n.Body,
		Channel:   mapChannelProto(n.Channel),
		Priority:  mapPriorityProto(n.Priority),
		Status:    mapStatusProto(n.Status),
		Metadata:  n.Metadata,
		CreatedAt: timestamppb.New(n.CreatedAt),
	}
	if n.ActionURL != nil {
		pn.ActionUrl = *n.ActionURL
	}
	if n.ReadAt != nil {
		pn.ReadAt = timestamppb.New(*n.ReadAt)
	}
	return pn
}

func mapChannelProto(channel string) notificationv1.NotificationChannel {
	switch channel {
	case "IN_APP":
		return notificationv1.NotificationChannel_NOTIFICATION_CHANNEL_IN_APP
	case "EMAIL":
		return notificationv1.NotificationChannel_NOTIFICATION_CHANNEL_EMAIL
	case "PUSH":
		return notificationv1.NotificationChannel_NOTIFICATION_CHANNEL_PUSH
	case "SMS":
		return notificationv1.NotificationChannel_NOTIFICATION_CHANNEL_SMS
	default:
		return notificationv1.NotificationChannel_NOTIFICATION_CHANNEL_UNSPECIFIED
	}
}

func mapPriorityProto(priority string) notificationv1.NotificationPriority {
	switch priority {
	case "LOW":
		return notificationv1.NotificationPriority_NOTIFICATION_PRIORITY_LOW
	case "NORMAL":
		return notificationv1.NotificationPriority_NOTIFICATION_PRIORITY_NORMAL
	case "HIGH":
		return notificationv1.NotificationPriority_NOTIFICATION_PRIORITY_HIGH
	case "URGENT":
		return notificationv1.NotificationPriority_NOTIFICATION_PRIORITY_URGENT
	default:
		return notificationv1.NotificationPriority_NOTIFICATION_PRIORITY_UNSPECIFIED
	}
}

func mapStatusProto(status string) notificationv1.NotificationStatus {
	switch status {
	case "unread":
		return notificationv1.NotificationStatus_NOTIFICATION_STATUS_UNREAD
	case "read":
		return notificationv1.NotificationStatus_NOTIFICATION_STATUS_READ
	case "archived":
		return notificationv1.NotificationStatus_NOTIFICATION_STATUS_ARCHIVED
	default:
		return notificationv1.NotificationStatus_NOTIFICATION_STATUS_UNSPECIFIED
	}
}
