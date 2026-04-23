// Package client provides gRPC clients for downstream services used by agent-orchestrator.
package client

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"sovereign-ai-compliance/shared/tenant"
)

// Clients holds gRPC connections and typed wrapper clients for downstream services.
type Clients struct {
	DocConn   *grpc.ClientConn
	AuditConn *grpc.ClientConn
	RAGConn   *grpc.ClientConn

	Doc   *DocClient
	Audit *AuditClient
	RAG   *RAGClient
}

// Config holds connection addresses for downstream services.
type Config struct {
	DocAddr     string `json:"doc_addr"`
	AuditAddr   string `json:"audit_addr"`
	RAGAddr     string `json:"rag_addr"`
	TLSCertFile string `json:"tls_cert_file,optional"`
	TLSKeyFile  string `json:"tls_key_file,optional"`
	Insecure    bool   `json:"insecure"`
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
	docConn, err := dial(cfg.DocAddr, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect to doc-service: %w", err)
	}

	auditConn, err := dial(cfg.AuditAddr, cfg)
	if err != nil {
		_ = docConn.Close()
		return nil, fmt.Errorf("connect to audit-service: %w", err)
	}

	ragConn, err := dial(cfg.RAGAddr, cfg)
	if err != nil {
		_ = docConn.Close()
		_ = auditConn.Close()
		return nil, fmt.Errorf("connect to rag-service: %w", err)
	}

	return &Clients{
		DocConn:   docConn,
		AuditConn: auditConn,
		RAGConn:   ragConn,
		Doc:       NewDocClient(docConn),
		Audit:     NewAuditClient(auditConn),
		RAG:       NewRAGClient(ragConn),
	}, nil
}

// Close closes all gRPC connections.
func (c *Clients) Close() error {
	var errs []error
	if c.DocConn != nil {
		errs = append(errs, c.DocConn.Close())
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
