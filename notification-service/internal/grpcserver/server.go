// Package grpcserver provides the gRPC server implementation for notification-service.
package grpcserver

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"sovereign-ai-compliance/notification-service/internal/logic"
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

// SendNotification creates and sends a notification.
func (s *Server) SendNotification(ctx context.Context, req *notificationv1.SendNotificationRequest) (*notificationv1.SendNotificationResponse, error) {
	ctx = withTenant(ctx)

	// For stub implementation, just return an empty success response.
	return &notificationv1.SendNotificationResponse{
		NotificationId: "",
	}, nil
}

// ListNotifications returns paginated notifications.
func (s *Server) ListNotifications(ctx context.Context, req *notificationv1.ListNotificationsRequest) (*notificationv1.ListNotificationsResponse, error) {
	ctx = withTenant(ctx)

	return &notificationv1.ListNotificationsResponse{
		Items:        []*notificationv1.Notification{},
		Total:        0,
		Page:         req.Page,
		Pages:        0,
		UnreadCount:  0,
	}, nil
}

// MarkRead marks one or all notifications as read.
func (s *Server) MarkRead(ctx context.Context, req *notificationv1.MarkReadRequest) (*notificationv1.MarkReadResponse, error) {
	ctx = withTenant(ctx)

	return &notificationv1.MarkReadResponse{
		Success:      true,
		MarkedCount:  0,
	}, nil
}

// GetPreferences returns notification preferences.
func (s *Server) GetPreferences(ctx context.Context, req *notificationv1.GetPreferencesRequest) (*notificationv1.GetPreferencesResponse, error) {
	ctx = withTenant(ctx)

	return &notificationv1.GetPreferencesResponse{
		Preferences: []*notificationv1.NotificationPreference{},
	}, nil
}

// UpdatePreferences updates notification preferences.
func (s *Server) UpdatePreferences(ctx context.Context, req *notificationv1.UpdatePreferencesRequest) (*notificationv1.UpdatePreferencesResponse, error) {
	ctx = withTenant(ctx)

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
