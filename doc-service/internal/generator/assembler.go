// Package generator provides document generation capabilities for doc-service.
package generator

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	auditv1 "sovereign-ai-compliance/shared/proto/audit/v1"
	orgv1 "sovereign-ai-compliance/shared/proto/org/v1"
	ragv1 "sovereign-ai-compliance/shared/proto/rag/v1"
)

// AssembledData holds all data collected from external services for document generation.
type AssembledData struct {
	// System metadata
	SystemName     string            `json:"system_name"`
	SystemVersion  string            `json:"system_version"`
	SystemPurpose  string            `json:"system_purpose"`
	SystemMetadata map[string]string `json:"system_metadata"`

	// Organization details (from org-service)
	OrganizationName string            `json:"organization_name"`
	OrganizationInfo map[string]string `json:"organization_info"`

	// Audit findings (from audit-service)
	AuditFindings  []AuditFinding `json:"audit_findings"`
	RiskScore      float64        `json:"risk_score"`
	RiskLevel      string         `json:"risk_level"`
	SeverityCounts map[string]int `json:"severity_counts"`

	// Knowledge base (from rag-service)
	KnowledgeChunks  []KnowledgeChunk  `json:"knowledge_chunks"`
	ArchitectureInfo map[string]string `json:"architecture_info"`
	DataGovernance   map[string]string `json:"data_governance"`
}

// AuditFinding represents a single audit finding.
type AuditFinding struct {
	Severity    string `json:"severity"`
	Category    string `json:"category"`
	Description string `json:"description"`
	RuleID      string `json:"rule_id"`
}

// KnowledgeChunk represents a knowledge retrieval result.
type KnowledgeChunk struct {
	Source    string  `json:"source"`
	Content   string  `json:"content"`
	Relevance float64 `json:"relevance"`
	Category  string  `json:"category"`
}

// DownstreamClients holds gRPC clients for services that DataAssembler calls.
type DownstreamClients struct {
	OrgClient   orgv1.OrgServiceClient
	AuditClient auditv1.AuditServiceClient
	RAGClient   ragv1.RAGServiceClient
}

// DataAssembler collects and assembles data from external services.
type DataAssembler struct {
	clients *DownstreamClients
}

// NewDataAssembler creates a new DataAssembler.
// If clients is nil, the assembler falls back to stub data.
func NewDataAssembler(clients *DownstreamClients) *DataAssembler {
	return &DataAssembler{clients: clients}
}

// Assemble collects data from all relevant services and returns assembled data.
func (a *DataAssembler) Assemble(ctx context.Context, aiSystemID string, auditJobID *string) (*AssembledData, error) {
	if a.clients == nil {
		return defaultStubData(), nil
	}

	data := &AssembledData{
		SystemName:    aiSystemID,
		SystemVersion: "1.0.0",
		SystemPurpose: "Automated compliance monitoring and documentation generation for AI systems",
		SystemMetadata: map[string]string{
			"domain":     "regulatory_compliance",
			"industry":   "technology",
			"deployment": "cloud",
		},
	}

	// 1. Fetch organization details from org-service
	if err := a.fetchOrganization(ctx, data); err != nil {
		// Log and continue with stub org data
		data.OrganizationName = "Unknown Organization"
		data.OrganizationInfo = map[string]string{"note": "failed to fetch from org-service: " + err.Error()}
	}

	// 2. Fetch AI system metadata from org-service
	if err := a.fetchAISystem(ctx, aiSystemID, data); err != nil {
		data.SystemMetadata["fetch_error"] = err.Error()
	}

	// 3. Fetch audit findings from audit-service
	if auditJobID != nil && *auditJobID != "" {
		if err := a.fetchAuditFindings(ctx, *auditJobID, data); err != nil {
			data.AuditFindings = []AuditFinding{
				{Severity: "medium", Category: "audit", Description: "Failed to fetch audit findings: " + err.Error(), RuleID: "FETCH-001"},
			}
		}
	}

	// 4. Fetch knowledge chunks from rag-service
	if err := a.fetchKnowledgeChunks(ctx, aiSystemID, data); err != nil {
		data.KnowledgeChunks = []KnowledgeChunk{
			{Source: "error", Content: "Failed to fetch knowledge: " + err.Error(), Relevance: 0, Category: "error"},
		}
	}

	return data, nil
}

func (a *DataAssembler) fetchOrganization(ctx context.Context, data *AssembledData) error {
	resp, err := a.clients.OrgClient.GetTenant(ctx, &orgv1.GetTenantRequest{})
	if err != nil {
		return err
	}
	data.OrganizationName = resp.GetName()
	data.OrganizationInfo = map[string]string{
		"legal_name": resp.GetName(),
		"domain":     resp.GetDomain(),
		"settings":   resp.GetSettings(),
	}
	return nil
}

func (a *DataAssembler) fetchAISystem(ctx context.Context, aiSystemID string, data *AssembledData) error {
	id, err := uuid.Parse(aiSystemID)
	if err != nil {
		return err
	}
	resp, err := a.clients.OrgClient.GetAISystem(ctx, &orgv1.GetAISystemRequest{SystemId: id.String()})
	if err != nil {
		return err
	}
	data.SystemName = resp.GetName()
	data.SystemPurpose = resp.GetDescription()
	data.SystemMetadata["risk_classification"] = resp.GetRiskClassification()
	data.SystemMetadata["status"] = resp.GetStatus()
	if meta := resp.GetMetadata(); meta != "" {
		data.SystemMetadata["raw_metadata"] = meta
	}
	return nil
}

func (a *DataAssembler) fetchAuditFindings(ctx context.Context, auditJobID string, data *AssembledData) error {
	resp, err := a.clients.AuditClient.GetAudit(ctx, &auditv1.GetAuditRequest{AuditId: auditJobID})
	if err != nil {
		return err
	}

	audit := resp.GetAudit()
	if audit != nil {
		data.RiskScore = float64(audit.GetRiskScore()) / 100.0
		data.RiskLevel = strings.ToLower(audit.GetRiskSeverity().String())
		data.SeverityCounts = map[string]int{
			"critical": int(audit.GetCriticalFindings()),
			"high":     int(audit.GetHighFindings()),
			"medium":   int(audit.GetMediumFindings()),
			"low":      int(audit.GetLowFindings()),
		}
	}

	findings := resp.GetFindings()
	data.AuditFindings = make([]AuditFinding, 0, len(findings))
	for _, f := range findings {
		data.AuditFindings = append(data.AuditFindings, AuditFinding{
			Severity:    strings.ToLower(f.GetSeverity().String()),
			Category:    strings.ToLower(f.GetIssueType().String()),
			Description: f.GetDescription(),
			RuleID:      f.GetId(),
		})
	}
	return nil
}

func (a *DataAssembler) fetchKnowledgeChunks(ctx context.Context, aiSystemID string, data *AssembledData) error {
	searchResp, err := a.clients.RAGClient.Search(ctx, &ragv1.SearchRequest{
		Query: fmt.Sprintf("AI system %s architecture data governance security", aiSystemID),
		TopK:  10,
	})
	if err != nil {
		return err
	}

	results := searchResp.GetResults()
	data.KnowledgeChunks = make([]KnowledgeChunk, 0, len(results))
	for _, r := range results {
		data.KnowledgeChunks = append(data.KnowledgeChunks, KnowledgeChunk{
			Source:    r.GetDocumentName(),
			Content:   r.GetText(),
			Relevance: r.GetSimilarity(),
			Category:  "general",
		})
	}

	// Also fetch stats for metadata
	statsResp, err := a.clients.RAGClient.GetStats(ctx, &ragv1.GetStatsRequest{})
	if err == nil && statsResp != nil {
		data.DataGovernance = map[string]string{
			"document_count":        fmt.Sprintf("%d", statsResp.GetDocumentCount()),
			"completed_documents":   fmt.Sprintf("%d", statsResp.GetCompletedDocuments()),
			"pending_documents":     fmt.Sprintf("%d", statsResp.GetPendingDocuments()),
			"embedding_count":       fmt.Sprintf("%d", statsResp.GetEmbeddingCount()),
			"total_file_size_bytes": fmt.Sprintf("%d", statsResp.GetTotalFileSizeBytes()),
		}
	}
	return nil
}

func defaultStubData() *AssembledData {
	return &AssembledData{
		SystemName:    "AI Compliance System",
		SystemVersion: "1.0.0",
		SystemPurpose: "Automated compliance monitoring and documentation generation for AI systems",
		SystemMetadata: map[string]string{
			"domain":     "regulatory_compliance",
			"industry":   "technology",
			"deployment": "cloud",
		},
		OrganizationName: "Example Organization",
		OrganizationInfo: map[string]string{
			"legal_name":    "Example Organization GmbH",
			"registration":  "DE123456789",
			"address":       "123 Compliance Street, Berlin, Germany",
			"contact_email": "compliance@example.org",
		},
		AuditFindings: []AuditFinding{
			{Severity: "high", Category: "data_governance", Description: "Training data lacks proper bias assessment documentation", RuleID: "AI-001"},
			{Severity: "medium", Category: "security", Description: "Model inference API lacks rate limiting", RuleID: "AI-042"},
			{Severity: "low", Category: "documentation", Description: "Missing update log for model version 1.2", RuleID: "AI-103"},
		},
		RiskScore: 0.72,
		RiskLevel: "medium",
		SeverityCounts: map[string]int{
			"critical": 0,
			"high":     1,
			"medium":   1,
			"low":      1,
		},
		KnowledgeChunks: []KnowledgeChunk{
			{Source: "architecture.md", Content: "The system uses a microservices architecture with containerized deployment on Kubernetes.", Relevance: 0.95, Category: "architecture"},
			{Source: "data-flow.pdf", Content: "Data flows from ingestion pipeline through preprocessing to model training and inference.", Relevance: 0.88, Category: "data_governance"},
			{Source: "security-policy.md", Content: "All data at rest is encrypted with AES-256 and in transit with TLS 1.3.", Relevance: 0.92, Category: "security"},
		},
		ArchitectureInfo: map[string]string{
			"framework":  "PyTorch 2.0",
			"deployment": "Kubernetes",
			"api_type":   "REST + gRPC",
			"hardware":   "GPU clusters (NVIDIA A100)",
		},
		DataGovernance: map[string]string{
			"training_data_size": "2.5M samples",
			"validation_split":   "80/10/10",
			"preprocessing":      "Standardization, outlier removal, deduplication",
			"privacy_technique":  "Differential privacy (epsilon=1.0)",
		},
	}
}

// BuildSectionPrompt creates an LLM prompt for a specific section using assembled data.
func BuildSectionPrompt(template SectionTemplate, data *AssembledData, sectionIndex int) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("You are generating section %d of an EU AI Act Annex IV technical documentation.\n\n", sectionIndex))
	b.WriteString(fmt.Sprintf("Section Title: %s\n", template.Title))
	b.WriteString(fmt.Sprintf("Description: %s\n\n", template.Description))

	if template.PromptHint != "" {
		b.WriteString(fmt.Sprintf("Guidance: %s\n\n", template.PromptHint))
	}

	b.WriteString("Contextual Data:\n")
	b.WriteString(fmt.Sprintf("- System: %s (version %s)\n", data.SystemName, data.SystemVersion))
	b.WriteString(fmt.Sprintf("- Purpose: %s\n", data.SystemPurpose))
	b.WriteString(fmt.Sprintf("- Organization: %s\n", data.OrganizationName))
	b.WriteString(fmt.Sprintf("- Risk Score: %.0f%% (%s risk)\n", data.RiskScore*100, data.RiskLevel))

	if len(data.SeverityCounts) > 0 {
		b.WriteString("- Audit Findings by Severity:")
		for sev, count := range data.SeverityCounts {
			b.WriteString(fmt.Sprintf(" %s=%d", sev, count))
		}
		b.WriteString("\n")
	}

	if len(data.ArchitectureInfo) > 0 {
		b.WriteString("- Architecture:")
		for k, v := range data.ArchitectureInfo {
			b.WriteString(fmt.Sprintf(" %s=%s", k, v))
		}
		b.WriteString("\n")
	}

	b.WriteString("\nRequirements:\n")
	b.WriteString("1. Write comprehensive, professional technical documentation\n")
	b.WriteString("2. Use specific details from the contextual data above\n")
	b.WriteString("3. Include concrete examples and measurements where applicable\n")
	b.WriteString("4. Write in formal regulatory documentation style\n")
	b.WriteString("5. Return only the section content in markdown format\n")

	return b.String()
}

// BuildSubSectionPrompt creates an LLM prompt for a subsection.
func BuildSubSectionPrompt(parent SectionTemplate, sub SubSectionTemplate, data *AssembledData, sectionIndex int, subIndex int) string {
	prompt := fmt.Sprintf("You are generating section %d.%d of an EU AI Act Annex IV technical documentation.\n\n", sectionIndex, subIndex)
	prompt += fmt.Sprintf("Parent Section: %s\n", parent.Title)
	prompt += fmt.Sprintf("Subsection Title: %s\n", sub.Title)

	if sub.PromptHint != "" {
		prompt += fmt.Sprintf("Guidance: %s\n\n", sub.PromptHint)
	}

	prompt += "Contextual Data:\n"
	prompt += fmt.Sprintf("- System: %s (version %s)\n", data.SystemName, data.SystemVersion)
	prompt += fmt.Sprintf("- Organization: %s\n", data.OrganizationName)

	if len(data.DataGovernance) > 0 && (parent.ID == "data_governance" || sub.ID == "data_sources" || sub.ID == "preprocessing") {
		prompt += "- Data Governance:"
		for k, v := range data.DataGovernance {
			prompt += fmt.Sprintf(" %s=%s", k, v)
		}
		prompt += "\n"
	}

	if len(data.KnowledgeChunks) > 0 {
		for _, chunk := range data.KnowledgeChunks {
			if chunk.Category == parent.ID || chunk.Category == sub.ID {
				prompt += fmt.Sprintf("- Knowledge: %s\n", chunk.Content)
			}
		}
	}

	prompt += "\nWrite detailed, professional content for this subsection in markdown format.\n"

	return prompt
}
