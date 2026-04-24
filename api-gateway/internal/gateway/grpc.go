// Package gateway wires the grpc-gateway REST-to-gRPC translation layer.
package gateway

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"sovereign-ai-compliance/api-gateway/internal/config"
	"sovereign-ai-compliance/api-gateway/internal/middleware"

	auditv1 "sovereign-ai-compliance/shared/proto/audit/v1"
	authv1 "sovereign-ai-compliance/shared/proto/auth/v1"
	docv1 "sovereign-ai-compliance/shared/proto/doc/v1"
	notificationv1 "sovereign-ai-compliance/shared/proto/notification/v1"
	orgv1 "sovereign-ai-compliance/shared/proto/org/v1"
	ragv1 "sovereign-ai-compliance/shared/proto/rag/v1"
	repov1 "sovereign-ai-compliance/shared/proto/repo/v1"
	"sovereign-ai-compliance/shared/tenant"
)

// Mux holds the grpc-gateway runtime.ServeMux and its backing connections.
type Mux struct {
	handler *runtime.ServeMux
	conns   []*grpc.ClientConn
}

// Handler returns the underlying http.Handler.
func (m *Mux) Handler() http.Handler {
	return m.handler
}

// Close closes all gRPC connections.
func (m *Mux) Close() error {
	for _, c := range m.conns {
		_ = c.Close()
	}
	return nil
}

// New creates a grpc-gateway mux wired to all backend gRPC services.
func New(cfg config.Config) (*Mux, error) {
	mux := runtime.NewServeMux(
		runtime.WithMetadata(func(ctx context.Context, r *http.Request) metadata.MD {
			// Forward tenant context from request context (injected by gateway middleware)
			// to gRPC outgoing metadata so downstream services receive x-tenant-id.
			md := metadata.MD{}
			if tenantID, ok := tenant.FromContext(r.Context()); ok && tenantID != "" {
				md.Set("x-tenant-id", tenantID)
			}
			// Also forward user_id from JWT claims for endpoints like GetMe.
			if claims := middleware.ClaimsFromContext(r.Context()); claims != nil {
				if userID, ok := claims["user_id"].(string); ok && userID != "" {
					md.Set("x-user-id", userID)
				}
			}
			if len(md) > 0 {
				return md
			}
			return nil
		}),
		runtime.WithErrorHandler(middleware.CustomErrorHandler()),
	)

	var conns []*grpc.ClientConn
	ctx := context.Background()

	// Helper to dial a backend
	dial := func(name, addr, certFile string, insecureFlag bool) (*grpc.ClientConn, error) {
		var opts []grpc.DialOption
		if certFile != "" {
			creds, err := credentials.NewClientTLSFromFile(certFile, "")
			if err != nil {
				return nil, fmt.Errorf("[%s] load TLS: %w", name, err)
			}
			opts = append(opts, grpc.WithTransportCredentials(creds))
		} else if insecureFlag {
			// nosemgrep
			opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		} else {
			return nil, fmt.Errorf("[%s] TLS config required", name)
		}
		conn, err := grpc.NewClient(addr, opts...)
		if err != nil {
			return nil, fmt.Errorf("[%s] dial %s: %w", name, addr, err)
		}
		return conn, nil
	}

	// Auth service
	if cfg.GRPCUpstream.Auth != "" {
		conn, err := dial("auth", cfg.GRPCUpstream.Auth, cfg.GRPCUpstream.AuthCert, cfg.GRPCUpstream.Insecure)
		if err != nil {
			return nil, err
		}
		conns = append(conns, conn)
		if err := authv1.RegisterAuthServiceHandler(ctx, mux, conn); err != nil {
			return nil, fmt.Errorf("register auth handler: %w", err)
		}
	}

	// Org service
	if cfg.GRPCUpstream.Org != "" {
		conn, err := dial("org", cfg.GRPCUpstream.Org, cfg.GRPCUpstream.OrgCert, cfg.GRPCUpstream.Insecure)
		if err != nil {
			return nil, err
		}
		conns = append(conns, conn)
		if err := orgv1.RegisterOrgServiceHandler(ctx, mux, conn); err != nil {
			return nil, fmt.Errorf("register org handler: %w", err)
		}
	}

	// Repo service
	if cfg.GRPCUpstream.Repo != "" {
		conn, err := dial("repo", cfg.GRPCUpstream.Repo, cfg.GRPCUpstream.RepoCert, cfg.GRPCUpstream.Insecure)
		if err != nil {
			return nil, err
		}
		conns = append(conns, conn)
		if err := repov1.RegisterRepoServiceHandler(ctx, mux, conn); err != nil {
			return nil, fmt.Errorf("register repo handler: %w", err)
		}
	}

	// RAG service
	if cfg.GRPCUpstream.RAG != "" {
		conn, err := dial("rag", cfg.GRPCUpstream.RAG, cfg.GRPCUpstream.RAGCert, cfg.GRPCUpstream.Insecure)
		if err != nil {
			return nil, err
		}
		conns = append(conns, conn)
		if err := ragv1.RegisterRAGServiceHandler(ctx, mux, conn); err != nil {
			return nil, fmt.Errorf("register rag handler: %w", err)
		}
	}

	// Doc service
	if cfg.GRPCUpstream.Doc != "" {
		conn, err := dial("doc", cfg.GRPCUpstream.Doc, cfg.GRPCUpstream.DocCert, cfg.GRPCUpstream.Insecure)
		if err != nil {
			return nil, err
		}
		conns = append(conns, conn)
		if err := docv1.RegisterDocServiceHandler(ctx, mux, conn); err != nil {
			return nil, fmt.Errorf("register doc handler: %w", err)
		}
	}

	// Audit service
	if cfg.GRPCUpstream.Audit != "" {
		conn, err := dial("audit", cfg.GRPCUpstream.Audit, cfg.GRPCUpstream.AuditCert, cfg.GRPCUpstream.Insecure)
		if err != nil {
			return nil, err
		}
		conns = append(conns, conn)
		if err := auditv1.RegisterAuditServiceHandler(ctx, mux, conn); err != nil {
			return nil, fmt.Errorf("register audit handler: %w", err)
		}
	}

	// Notification service
	if cfg.GRPCUpstream.Notification != "" {
		conn, err := dial("notification", cfg.GRPCUpstream.Notification, cfg.GRPCUpstream.NotifCert, cfg.GRPCUpstream.Insecure)
		if err != nil {
			return nil, err
		}
		conns = append(conns, conn)
		if err := notificationv1.RegisterNotificationServiceHandler(ctx, mux, conn); err != nil {
			return nil, fmt.Errorf("register notification handler: %w", err)
		}
	}

	return &Mux{handler: mux, conns: conns}, nil
}

// ShouldHandle returns true if the request path should be handled by grpc-gateway.
func ShouldHandle(path string) bool {
	prefixes := []string{
		"/api/v1/auth/",
		"/api/v1/org/",
		"/api/v1/ai-systems/",
		"/api/v1/repositories/",
		"/api/v1/repository-webhooks/",
		"/api/v1/documents/",
		"/api/v1/rag/",
		"/api/v1/generated-documents/",
		"/api/v1/export-jobs/",
		"/api/v1/audit-jobs/",
		"/api/v1/notifications/",
		"/api/v1/approvals/",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}
