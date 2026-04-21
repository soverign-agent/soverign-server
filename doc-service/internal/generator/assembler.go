// Package generator provides document generation capabilities for doc-service.
package generator

import (
	"context"
	"fmt"
	"strings"
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

// DataAssembler collects and assembles data from external services.
// For M9 V1, this uses stub implementations since gRPC protos for
// other services may not be finalized. In production, this will call
// audit-service, rag-service, and org-service via gRPC.
type DataAssembler struct{}

// NewDataAssembler creates a new DataAssembler.
func NewDataAssembler() *DataAssembler {
	return &DataAssembler{}
}

// Assemble collects data from all relevant services and returns assembled data.
// TODO: Replace stub implementations with real gRPC clients when M7/M8 protos are ready.
func (a *DataAssembler) Assemble(ctx context.Context, aiSystemID string, auditJobID *string) (*AssembledData, error) {
	data := &AssembledData{
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

	// In a real implementation, we would:
	// 1. Call org-service gRPC to get organization details
	// 2. Call audit-service gRPC to get findings for aiSystemID/auditJobID
	// 3. Call rag-service gRPC to retrieve relevant knowledge chunks
	// For now, return realistic stub data that enables document generation.

	return data, nil
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
