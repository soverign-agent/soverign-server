package tests

import (
	"context"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"sovereign-ai-compliance/agent-orchestrator/internal/client"
	"sovereign-ai-compliance/agent-orchestrator/internal/orchestrator"
	auditv1 "sovereign-ai-compliance/shared/proto/audit/v1"
	docv1 "sovereign-ai-compliance/shared/proto/doc/v1"
	ragv1 "sovereign-ai-compliance/shared/proto/rag/v1"
	"sovereign-ai-compliance/shared/tenant"
)

// mockDocServer implements the subset of docv1.DocServiceServer that the
// activity test exercises. It records tenant metadata so we can assert
// propagation, and its responses are hard-coded for determinism.
type mockDocServer struct {
	// TenantID is populated by each gRPC handler from incoming metadata.
	TenantID string
	docv1.UnimplementedDocServiceServer
}

func (m *mockDocServer) GenerateDocument(ctx context.Context, req *docv1.GenerateDocumentRequest) (*docv1.GenerateDocumentResponse, error) {
	m.TenantID = tenantFromMetadata(ctx)
	return &docv1.GenerateDocumentResponse{
		DocumentId: "doc-" + req.AiSystemId,
		Status:     docv1.DocumentStatus_DOCUMENT_STATUS_GENERATING,
	}, nil
}

func (m *mockDocServer) GetDocument(ctx context.Context, req *docv1.GetDocumentRequest) (*docv1.GetDocumentResponse, error) {
	m.TenantID = tenantFromMetadata(ctx)
	return &docv1.GetDocumentResponse{
		Document: &docv1.GeneratedDocument{
			Id:         req.DocumentId,
			AiSystemId: "sys-1",
			DocType:    docv1.DocType_DOC_TYPE_ANNEX_IV,
			Title:      "Test Document",
			Status:     docv1.DocumentStatus_DOCUMENT_STATUS_APPROVED,
			Version:    1,
			Content: &docv1.DocumentContent{
				Sections: []*docv1.Section{
					{Id: "sec-1", Title: "Section 1", Content: "Hello", Order: 1},
				},
			},
		},
	}, nil
}

// mockAuditServer implements the audit-service gRPC surface used by
// orchestrator activities. The second GetAudit call flips status to COMPLETED
// so that callers can observe a progression.
type mockAuditServer struct {
	TenantID   string
	GetCount   int
	auditv1.UnimplementedAuditServiceServer
}

func (m *mockAuditServer) TriggerAudit(ctx context.Context, req *auditv1.TriggerAuditRequest) (*auditv1.TriggerAuditResponse, error) {
	m.TenantID = tenantFromMetadata(ctx)
	return &auditv1.TriggerAuditResponse{
		AuditId:    "audit-" + req.RepositoryId,
		Status:     auditv1.AuditJobStatus_AUDIT_JOB_STATUS_PENDING,
		WorkflowId: "audit-wf-1",
	}, nil
}

func (m *mockAuditServer) GetAudit(ctx context.Context, req *auditv1.GetAuditRequest) (*auditv1.GetAuditResponse, error) {
	m.TenantID = tenantFromMetadata(ctx)
	m.GetCount++
	status := "running"
	if m.GetCount >= 2 {
		status = "completed"
	}
	return &auditv1.GetAuditResponse{
		Audit: &auditv1.AuditJob{
			Id:                  req.AuditId,
			RepositoryId:        "repo-1",
			Name:                "test-audit",
			Status:              status,
			ProgressPercentage:  int32(m.GetCount * 50),
			RiskScore:           42,
			RiskSeverity:        auditv1.RiskSeverity_RISK_SEVERITY_MEDIUM,
			FindingsCount:       3,
			CurrentStep:         "test-step",
			WorkflowId:          "wf-1",
		},
	}, nil
}

// mockRAGServer implements the rag-service Search endpoint. It records the
// query and topK from the request, and injects a deterministic hit so the
// activity under test has something to marshal.
type mockRAGServer struct {
	TenantID string
	Query    string
	TopK     int32
	ragv1.UnimplementedRAGServiceServer
}

func (m *mockRAGServer) Search(ctx context.Context, req *ragv1.SearchRequest) (*ragv1.SearchResponse, error) {
	m.TenantID = tenantFromMetadata(ctx)
	m.Query = req.Query
	m.TopK = req.TopK
	return &ragv1.SearchResponse{
		Query: req.Query,
		TopK:  req.TopK,
		Results: []*ragv1.SearchResult{
			{
				DocumentId:   "doc-1",
				DocumentName: "EU AI Act Overview",
				Text:         "The EU AI Act establishes requirements for high-risk AI systems.",
				Similarity:   0.95,
			},
		},
	}, nil
}

// tenantFromMetadata reads the tenant propagated by withTenantMetadata from
// the client package. This is the key assertion for the tenant-propagation
// test case.
func tenantFromMetadata(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	vals := md.Get("x-tenant-id")
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

// mockClients spins up three in-process gRPC servers (doc, audit, rag) and
// returns a fully-wired client.Clients pointing at them. The cleanup func
// stops all listeners and servers. This gives every activity test an isolated
// set of downstream services without needing real deployments.
func mockClients(t *testing.T) (*client.Clients, *mockDocServer, *mockAuditServer, *mockRAGServer, func()) {
	t.Helper()

	docListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	auditListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ragListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	docServer := grpc.NewServer()
	auditServer := grpc.NewServer()
	ragServer := grpc.NewServer()

	docMock := &mockDocServer{}
	auditMock := &mockAuditServer{}
	ragMock := &mockRAGServer{}

	docv1.RegisterDocServiceServer(docServer, docMock)
	auditv1.RegisterAuditServiceServer(auditServer, auditMock)
	ragv1.RegisterRAGServiceServer(ragServer, ragMock)

	go docServer.Serve(docListener)
	go auditServer.Serve(auditListener)
	go ragServer.Serve(ragListener)

	cfg := client.Config{
		DocAddr:   docListener.Addr().String(),
		AuditAddr: auditListener.Addr().String(),
		RAGAddr:   ragListener.Addr().String(),
		Insecure:  true,
	}
	clients, err := client.NewClients(cfg)
	require.NoError(t, err)

	cleanup := func() {
		clients.Close()
		docServer.Stop()
		auditServer.Stop()
		ragServer.Stop()
		docListener.Close()
		auditListener.Close()
		ragListener.Close()
	}
	return clients, docMock, auditMock, ragMock, cleanup
}

func TestActivities_SearchKnowledgeActivity(t *testing.T) {
	ctx := tenant.WithContext(context.Background(), "tenant-search")
	clients, _, _, ragMock, cleanup := mockClients(t)
	defer cleanup()

	acts := orchestrator.NewActivities(clients)
	result, err := acts.SearchKnowledgeActivity(ctx, "AI compliance", 5)
	require.NoError(t, err)
	require.Equal(t, "AI compliance", result.Query)
	require.Equal(t, int32(5), result.TopK)
	require.Len(t, result.Results, 1)
	require.Equal(t, "EU AI Act Overview", result.Results[0].DocumentName)
	require.Equal(t, "tenant-search", ragMock.TenantID)
}

func TestActivities_GenerateDocumentActivity(t *testing.T) {
	ctx := tenant.WithContext(context.Background(), "tenant-doc")
	clients, docMock, _, _, cleanup := mockClients(t)
	defer cleanup()

	acts := orchestrator.NewActivities(clients)
	result, err := acts.GenerateDocumentActivity(ctx, uuid.NewString(), "annex_iv", "Test Title")
	require.NoError(t, err)
	require.NotEmpty(t, result.DocumentID)
	require.Equal(t, docv1.DocumentStatus_DOCUMENT_STATUS_GENERATING.String(), result.Status)
	require.Equal(t, "tenant-doc", docMock.TenantID)
}

func TestActivities_GetDocumentActivity(t *testing.T) {
	ctx := tenant.WithContext(context.Background(), "tenant-getdoc")
	clients, docMock, _, _, cleanup := mockClients(t)
	defer cleanup()

	acts := orchestrator.NewActivities(clients)
	result, err := acts.GetDocumentActivity(ctx, uuid.NewString())
	require.NoError(t, err)
	require.Equal(t, docv1.DocumentStatus_DOCUMENT_STATUS_APPROVED.String(), result.Status)
	require.Equal(t, "Test Document", result.Title)
	require.Len(t, result.Sections, 1)
	require.Equal(t, "tenant-getdoc", docMock.TenantID)
}

func TestActivities_TriggerAuditActivity(t *testing.T) {
	ctx := tenant.WithContext(context.Background(), "tenant-audit")
	clients, _, auditMock, _, cleanup := mockClients(t)
	defer cleanup()

	acts := orchestrator.NewActivities(clients)
	result, err := acts.TriggerAuditActivity(ctx, uuid.NewString(), "Compliance Audit", "full")
	require.NoError(t, err)
	require.NotEmpty(t, result.AuditID)
	require.Equal(t, "audit-wf-1", result.WorkflowID)
	require.Equal(t, "tenant-audit", auditMock.TenantID)
}

func TestActivities_GetAuditActivity(t *testing.T) {
	ctx := tenant.WithContext(context.Background(), "tenant-getaudit")
	clients, _, auditMock, _, cleanup := mockClients(t)
	defer cleanup()

	acts := orchestrator.NewActivities(clients)
	auditID := uuid.NewString()
	result, err := acts.GetAuditActivity(ctx, auditID)
	require.NoError(t, err)
	require.Equal(t, auditID, result.AuditID)
	require.Equal(t, "running", result.Status)
	require.Equal(t, "tenant-getaudit", auditMock.TenantID)
}

func TestActivities_MissingClients(t *testing.T) {
	acts := orchestrator.NewActivities(nil)
	ctx := context.Background()

	_, err := acts.SearchKnowledgeActivity(ctx, "q", 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "rag client not configured")

	_, err = acts.GenerateDocumentActivity(ctx, uuid.NewString(), "t", "title")
	require.Error(t, err)
	require.Contains(t, err.Error(), "doc client not configured")

	_, err = acts.GetDocumentActivity(ctx, uuid.NewString())
	require.Error(t, err)
	require.Contains(t, err.Error(), "doc client not configured")

	_, err = acts.TriggerAuditActivity(ctx, uuid.NewString(), "n", "full")
	require.Error(t, err)
	require.Contains(t, err.Error(), "audit client not configured")

	_, err = acts.GetAuditActivity(ctx, "audit-1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "audit client not configured")
}

func TestActivities_InvalidIDs(t *testing.T) {
	clients, _, _, _, cleanup := mockClients(t)
	defer cleanup()
	acts := orchestrator.NewActivities(clients)
	ctx := context.Background()

	_, err := acts.GenerateDocumentActivity(ctx, "not-a-uuid", "t", "title")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid ai system id")

	_, err = acts.GetDocumentActivity(ctx, "not-a-uuid")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid document id")

	_, err = acts.TriggerAuditActivity(ctx, "not-a-uuid", "n", "full")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid repository id")

	_, err = acts.GetAuditActivity(ctx, "not-a-uuid")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid audit id")
}
