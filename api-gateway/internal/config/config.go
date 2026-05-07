// Package config holds the API gateway configuration.
package config

// Config defines the API gateway configuration.
type Config struct {
	Host            string
	Port            int
	Auth            AuthConfig
	Upstream        UpstreamConfig
	GRPCUpstream    GRPCUpstreamConfig `json:"grpc_upstream"`
	PublicEndpoints []string
}

// AuthConfig holds JWT authentication settings.
type AuthConfig struct {
	PublicKeyPath string
}

// UpstreamConfig holds backend service URLs.
type UpstreamConfig struct {
	Auth         string
	Org          string
	Repo         string
	RAG          string
	Doc          string
	Audit        string
	Notification string
}

// GRPCUpstreamConfig holds gRPC backend addresses and TLS settings.
type GRPCUpstreamConfig struct {
	Auth             string
	AuthCert         string `json:"auth_cert_file"`
	Org              string
	OrgCert          string `json:"org_cert_file"`
	Repo             string
	RepoCert         string `json:"repo_cert_file"`
	RAG              string
	RAGCert          string `json:"rag_cert_file"`
	Doc              string
	DocCert          string `json:"doc_cert_file"`
	Audit            string
	AuditCert        string `json:"audit_cert_file"`
	Notification     string
	NotifCert        string `json:"notif_cert_file"`
	Monitoring       string
	MonitoringCert   string `json:"monitoring_cert_file"`
	Orchestrator     string
	OrchestratorCert string `json:"orchestrator_cert_file"`
	Insecure         bool   `json:"insecure"`
}
