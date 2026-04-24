// Package client provides gRPC clients for downstream services used by doc-service.
package client

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	auditv1 "sovereign-ai-compliance/shared/proto/audit/v1"
	orgv1 "sovereign-ai-compliance/shared/proto/org/v1"
	ragv1 "sovereign-ai-compliance/shared/proto/rag/v1"
	"sovereign-ai-compliance/shared/tenant"
)

// Clients holds gRPC connections and typed clients for downstream services.
type Clients struct {
	OrgConn   *grpc.ClientConn
	AuditConn *grpc.ClientConn
	RAGConn   *grpc.ClientConn

	OrgClient   orgv1.OrgServiceClient
	AuditClient auditv1.AuditServiceClient
	RAGClient   ragv1.RAGServiceClient
}

// Config holds connection addresses for downstream services.
type Config struct {
	OrgAddr     string
	AuditAddr   string
	RAGAddr     string
	TLSCertFile string `json:",optional"`
	TLSKeyFile  string `json:",optional"`
	Insecure    bool
}

func dial(addr string, cfg Config) (*grpc.ClientConn, error) {
	var opts []grpc.DialOption
	if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
		creds, err := credentials.NewClientTLSFromFile(cfg.TLSCertFile, "")
		if err != nil {
			return nil, fmt.Errorf("load TLS credentials: %w", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else if cfg.Insecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		return nil, fmt.Errorf("TLS config required; set tls_cert_file/tls_key_file or insecure=true for local dev")
	}
	return grpc.NewClient(addr, opts...)
}

// NewClients creates gRPC connections and typed clients for all downstream services.
func NewClients(cfg Config) (*Clients, error) {
	orgConn, err := dial(cfg.OrgAddr, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect to org-service: %w", err)
	}

	auditConn, err := dial(cfg.AuditAddr, cfg)
	if err != nil {
		_ = orgConn.Close()
		return nil, fmt.Errorf("connect to audit-service: %w", err)
	}

	ragConn, err := dial(cfg.RAGAddr, cfg)
	if err != nil {
		_ = orgConn.Close()
		_ = auditConn.Close()
		return nil, fmt.Errorf("connect to rag-service: %w", err)
	}

	return &Clients{
		OrgConn:     orgConn,
		AuditConn:   auditConn,
		RAGConn:     ragConn,
		OrgClient:   orgv1.NewOrgServiceClient(orgConn),
		AuditClient: auditv1.NewAuditServiceClient(auditConn),
		RAGClient:   ragv1.NewRAGServiceClient(ragConn),
	}, nil
}

// Close closes all gRPC connections.
func (c *Clients) Close() error {
	var errs []error
	if c.OrgConn != nil {
		errs = append(errs, c.OrgConn.Close())
	}
	if c.AuditConn != nil {
		errs = append(errs, c.AuditConn.Close())
	}
	if c.RAGConn != nil {
		errs = append(errs, c.RAGConn.Close())
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// withTenantMetadata injects tenant ID from context into gRPC outgoing metadata.
func withTenantMetadata(ctx context.Context) context.Context {
	tenantID, ok := tenant.FromContext(ctx)
	if !ok || tenantID == "" {
		return ctx
	}
	md := metadata.Pairs("x-tenant-id", tenantID)
	return metadata.NewOutgoingContext(ctx, md)
}
