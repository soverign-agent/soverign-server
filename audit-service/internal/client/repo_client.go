// Package client provides gRPC clients for downstream services used by audit-service.
package client

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"sovereign-ai-compliance/audit-service/temporal"
	repov1 "sovereign-ai-compliance/shared/proto/repo/v1"
	"sovereign-ai-compliance/shared/tenant"
)

// RepoClient wraps the repo-service gRPC client to fetch repositories and
// static-analysis results for audit workflows.
type RepoClient struct {
	client repov1.RepoServiceClient
}

// NewRepoClient creates a new RepoClient from an existing gRPC connection.
func NewRepoClient(conn *grpc.ClientConn) *RepoClient {
	return &RepoClient{
		client: repov1.NewRepoServiceClient(conn),
	}
}

// DialRepo creates a gRPC connection to the repo-service.
func DialRepo(addr string, insecureConn bool, tlsCertFile string) (*grpc.ClientConn, error) {
	var opts []grpc.DialOption
	if tlsCertFile != "" {
		creds, err := credentials.NewClientTLSFromFile(tlsCertFile, "")
		if err != nil {
			return nil, fmt.Errorf("load TLS credentials: %w", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else if insecureConn {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		return nil, fmt.Errorf("TLS config required; set tls_cert_file or insecure=true for local dev")
	}
	return grpc.NewClient(addr, opts...)
}

// FetchRepository verifies the repository exists in repo-service.
func (r *RepoClient) FetchRepository(ctx context.Context, repositoryID uuid.UUID) error {
	tenantID, _ := tenant.FromContext(ctx)
	ctx = withTenantMetadata(ctx, tenantID)

	_, err := r.client.GetRepository(ctx, &repov1.GetRepositoryRequest{
		RepositoryId: repositoryID.String(),
	})
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}
	return nil
}

// RunStaticAnalysis triggers a synchronous code scan via repo-service.
// The scan clones the repo, runs static analysis, and persists the results.
func (r *RepoClient) RunStaticAnalysis(ctx context.Context, repositoryID uuid.UUID) error {
	tenantID, _ := tenant.FromContext(ctx)
	ctx = withTenantMetadata(ctx, tenantID)

	_, err := r.client.TriggerScan(ctx, &repov1.TriggerScanRequest{
		RepositoryId: repositoryID.String(),
	})
	if err != nil {
		return fmt.Errorf("trigger scan: %w", err)
	}
	return nil
}

// GetAnalysisResults fetches the latest scan result for a repository and
// converts the JSON-encoded detections into audit findings.
func (r *RepoClient) GetAnalysisResults(ctx context.Context, repositoryID uuid.UUID) ([]temporal.AnalysisFinding, error) {
	tenantID, _ := tenant.FromContext(ctx)
	ctx = withTenantMetadata(ctx, tenantID)

	resp, err := r.client.ListScanResults(ctx, &repov1.ListScanResultsRequest{
		RepositoryId: repositoryID.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("list scan results: %w", err)
	}

	if len(resp.ScanResults) == 0 {
		return nil, nil
	}

	// Pick the most recent scan result by CreatedAt
	latest := resp.ScanResults[0]
	for _, sr := range resp.ScanResults[1:] {
		if sr.CreatedAt.AsTime().After(latest.CreatedAt.AsTime()) {
			latest = sr
		}
	}

	return convertScanResultToFindings(latest)
}

// convertScanResultToFindings parses the JSON arrays stored in ScanResult and
// maps each detection to an AnalysisFinding using the severity/issue-type
// taxonomy expected by the audit workflow.
func convertScanResultToFindings(sr *repov1.ScanResult) ([]temporal.AnalysisFinding, error) {
	var findings []temporal.AnalysisFinding

	// AI usage detections → transparency / record-keeping findings
	var aiDetections []struct {
		Package string `json:"package"`
		Import  string `json:"import"`
		File    string `json:"file"`
		Line    int    `json:"line"`
	}
	if err := json.Unmarshal([]byte(sr.AiUses), &aiDetections); err == nil {
		for _, d := range aiDetections {
			desc := d.Import
			if d.Package != "" {
				desc = fmt.Sprintf("Package: %s, Import: %s", d.Package, d.Import)
			}
			findings = append(findings, temporal.AnalysisFinding{
				FilePath:    d.File,
				LineNumber:  d.Line,
				IssueType:   "ai_usage",
				Severity:    "medium",
				Title:       "AI/LLM library usage detected",
				Description: desc,
			})
		}
	}

	// Data flows → data-privacy / transparency findings
	var dataFlows []struct {
		Source      string `json:"source"`
		Sink        string `json:"sink"`
		File        string `json:"file"`
		Line        int    `json:"line"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal([]byte(sr.DataFlows), &dataFlows); err == nil {
		for _, d := range dataFlows {
			findings = append(findings, temporal.AnalysisFinding{
				FilePath:    d.File,
				LineNumber:  d.Line,
				IssueType:   "data_flow",
				Severity:    "high",
				Title:       fmt.Sprintf("Data flow: %s → %s", d.Source, d.Sink),
				Description: d.Description,
			})
		}
	}

	// Sensitive data detections → security findings
	var sensitiveData []struct {
		Type        string `json:"type"`
		Location    string `json:"location"`
		File        string `json:"file"`
		Line        int    `json:"line"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal([]byte(sr.SensitiveData), &sensitiveData); err == nil {
		for _, d := range sensitiveData {
			findings = append(findings, temporal.AnalysisFinding{
				FilePath:    d.File,
				LineNumber:  d.Line,
				IssueType:   "sensitive_data",
				Severity:    "high",
				Title:       fmt.Sprintf("Sensitive data: %s", d.Type),
				Description: d.Description,
			})
		}
	}

	return findings, nil
}
