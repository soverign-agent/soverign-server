// Package generator provides document generation capabilities for doc-service.
package generator

// SectionTemplate defines the structure for a single document section.
type SectionTemplate struct {
	ID          string               `json:"id"`
	Title       string               `json:"title"`
	Description string               `json:"description"`
	Order       int                  `json:"order"`
	SubSections []SubSectionTemplate `json:"subsections,omitempty"`
	PromptHint  string               `json:"prompt_hint"` // Guidance for the LLM prompt
}

// SubSectionTemplate defines the structure for a document subsection.
type SubSectionTemplate struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Order       int    `json:"order"`
	PromptHint  string `json:"prompt_hint"`
}

// DocumentTemplate defines a complete document template.
type DocumentTemplate struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Sections    []SectionTemplate `json:"sections"`
}

// AnnexIVTemplate returns the EU AI Act Annex IV technical documentation template.
func AnnexIVTemplate() DocumentTemplate {
	return DocumentTemplate{
		ID:          "annex_iv",
		Name:        "EU AI Act Annex IV Technical Documentation",
		Description: "Comprehensive technical documentation required for high-risk AI systems under EU AI Act",
		Sections: []SectionTemplate{
			{
				ID:          "general_description",
				Title:       "1. General Description of the AI System",
				Description: "A general description of the AI system including its intended purpose, version, and key characteristics",
				Order:       1,
				PromptHint:  "Write a comprehensive general description of this AI system. Include: system name and version, intended purpose and use cases, key capabilities and limitations, target users and deployment context, and any specific domain or industry context. Base this on the system metadata provided.",
				SubSections: []SubSectionTemplate{
					{ID: "system_identity", Title: "1.1 System Identity", Order: 1, PromptHint: "Provide system name, version, and unique identifiers"},
					{ID: "intended_purpose", Title: "1.2 Intended Purpose", Order: 2, PromptHint: "Describe the specific purpose, use cases, and problem the AI system solves"},
					{ID: "capabilities", Title: "1.3 Capabilities and Limitations", Order: 3, PromptHint: "Detail what the system can and cannot do, including performance boundaries"},
				},
			},
			{
				ID:          "developer_info",
				Title:       "2. Developer and Distributor Information",
				Description: "Information about the organization developing and distributing the AI system",
				Order:       2,
				PromptHint:  "Document the developer and distributor information. Include: organization name and legal identity, contact information, role in the AI system lifecycle (developer, deployer, distributor), and any relevant certifications or registrations.",
				SubSections: []SubSectionTemplate{
					{ID: "organization", Title: "2.1 Organization Details", Order: 1, PromptHint: "Name, legal form, registration details of the developing organization"},
					{ID: "contacts", Title: "2.2 Contact Information", Order: 2, PromptHint: "Key contacts for technical and regulatory inquiries"},
					{ID: "roles", Title: "2.3 Roles and Responsibilities", Order: 3, PromptHint: "Roles of the organization in the AI value chain"},
				},
			},
			{
				ID:          "development_process",
				Title:       "3. Development Process and Methods",
				Description: "Description of the development process, design choices, and methodologies used",
				Order:       3,
				PromptHint:  "Describe the development process and methods used to create this AI system. Include: development lifecycle and phases, design choices and rationale, software engineering practices, version control and change management, testing and validation approaches, and any standards or frameworks followed.",
				SubSections: []SubSectionTemplate{
					{ID: "lifecycle", Title: "3.1 Development Lifecycle", Order: 1, PromptHint: "Phases from conception to deployment"},
					{ID: "design_choices", Title: "3.2 Design Choices", Order: 2, PromptHint: "Key architectural and algorithmic decisions"},
					{ID: "testing_validation", Title: "3.3 Testing and Validation", Order: 3, PromptHint: "Testing methodologies and validation results"},
				},
			},
			{
				ID:          "risk_management",
				Title:       "4. Risk Management System",
				Description: "Details of the risk management system implemented for the AI system",
				Order:       4,
				PromptHint:  "Document the risk management system. Include: risk identification and assessment methodology, risk mitigation measures implemented, residual risks and their management, risk monitoring and review processes, and integration with overall organizational risk management. Use the audit findings and risk scores provided.",
				SubSections: []SubSectionTemplate{
					{ID: "risk_identification", Title: "4.1 Risk Identification and Assessment", Order: 1, PromptHint: "Known and foreseeable risks associated with the AI system"},
					{ID: "mitigation", Title: "4.2 Risk Mitigation Measures", Order: 2, PromptHint: "Measures taken to eliminate or reduce identified risks"},
					{ID: "residual_risks", Title: "4.3 Residual Risks", Order: 3, PromptHint: "Remaining risks after mitigation and their management"},
				},
			},
			{
				ID:          "data_governance",
				Title:       "5. Data Governance and Management",
				Description: "Description of data used for training, validation, and testing",
				Order:       5,
				PromptHint:  "Describe the data governance and management practices. Include: data sources and collection methods, data preprocessing and cleaning procedures, training, validation, and test data splits, data quality assurance measures, bias detection and mitigation approaches, data protection and privacy measures, and any data labeling or annotation processes.",
				SubSections: []SubSectionTemplate{
					{ID: "data_sources", Title: "5.1 Data Sources", Order: 1, PromptHint: "Origin, type, and characteristics of training data"},
					{ID: "preprocessing", Title: "5.2 Data Preprocessing", Order: 2, PromptHint: "Cleaning, transformation, and augmentation procedures"},
					{ID: "quality_assurance", Title: "5.3 Data Quality Assurance", Order: 3, PromptHint: "Measures to ensure data quality and representativeness"},
					{ID: "bias_mitigation", Title: "5.4 Bias Detection and Mitigation", Order: 4, PromptHint: "Approaches to identify and address data bias"},
				},
			},
			{
				ID:          "system_architecture",
				Title:       "6. System Architecture and Design",
				Description: "Technical description of the AI system architecture",
				Order:       6,
				PromptHint:  "Provide a technical description of the system architecture and design. Include: overall system architecture and components, AI model architecture and algorithms, hardware and software requirements, interfaces and integrations, scalability and performance characteristics, and any third-party components or dependencies.",
				SubSections: []SubSectionTemplate{
					{ID: "architecture", Title: "6.1 Overall Architecture", Order: 1, PromptHint: "High-level system architecture and component diagram description"},
					{ID: "model_design", Title: "6.2 AI Model Design", Order: 2, PromptHint: "Model architecture, algorithms, and training approach"},
					{ID: "infrastructure", Title: "6.3 Infrastructure Requirements", Order: 3, PromptHint: "Hardware, software, and network requirements"},
				},
			},
			{
				ID:          "human_oversight",
				Title:       "7. Human Oversight Measures",
				Description: "Measures to facilitate human oversight of the AI system",
				Order:       7,
				PromptHint:  "Document the human oversight measures. Include: human-in-the-loop design points, operator training and qualification requirements, override and intervention mechanisms, alert and notification systems, decision review and appeal processes, and documentation of human oversight procedures.",
				SubSections: []SubSectionTemplate{
					{ID: "oversight_design", Title: "7.1 Oversight Design", Order: 1, PromptHint: "How humans are integrated into system operations"},
					{ID: "intervention", Title: "7.2 Intervention Mechanisms", Order: 2, PromptHint: "Procedures for human override and correction"},
					{ID: "training", Title: "7.3 Operator Training", Order: 3, PromptHint: "Training requirements and materials for human operators"},
				},
			},
			{
				ID:          "performance_metrics",
				Title:       "8. Performance and Accuracy Metrics",
				Description: "Expected performance, accuracy, and robustness metrics",
				Order:       8,
				PromptHint:  "Document the performance and accuracy metrics. Include: key performance indicators and targets, accuracy metrics and benchmarks, robustness and reliability measures, fairness and non-discrimination metrics, performance under varying conditions, and comparison with baseline or previous versions.",
				SubSections: []SubSectionTemplate{
					{ID: "kpis", Title: "8.1 Key Performance Indicators", Order: 1, PromptHint: "Primary metrics used to evaluate system performance"},
					{ID: "accuracy", Title: "8.2 Accuracy and Robustness", Order: 2, PromptHint: "Accuracy metrics and robustness testing results"},
					{ID: "fairness", Title: "8.3 Fairness and Non-discrimination", Order: 3, PromptHint: "Fairness metrics and demographic parity assessments"},
				},
			},
			{
				ID:          "security_measures",
				Title:       "9. Security and Data Protection Measures",
				Description: "Measures to ensure security and data protection",
				Order:       9,
				PromptHint:  "Document the security and data protection measures. Include: cybersecurity measures and controls, data encryption and protection, access control and authentication, vulnerability management, incident response procedures, data retention and deletion policies, and compliance with relevant security standards.",
				SubSections: []SubSectionTemplate{
					{ID: "cybersecurity", Title: "9.1 Cybersecurity Measures", Order: 1, PromptHint: "Technical and organizational security measures"},
					{ID: "data_protection", Title: "9.2 Data Protection", Order: 2, PromptHint: "Personal data handling and privacy measures"},
					{ID: "incident_response", Title: "9.3 Incident Response", Order: 3, PromptHint: "Procedures for security incidents and breaches"},
				},
			},
			{
				ID:          "record_keeping",
				Title:       "10. Record Keeping and Documentation",
				Description: "Documentation of post-market monitoring and record keeping",
				Order:       10,
				PromptHint:  "Document the record keeping and post-market monitoring procedures. Include: documentation maintenance procedures, post-market monitoring plan, incident reporting procedures, change management and version control, audit trail and logging, and retention periods for documentation.",
				SubSections: []SubSectionTemplate{
					{ID: "documentation", Title: "10.1 Documentation Maintenance", Order: 1, PromptHint: "Procedures for keeping documentation up to date"},
					{ID: "monitoring", Title: "10.2 Post-market Monitoring", Order: 2, PromptHint: "Plan for ongoing monitoring after deployment"},
					{ID: "change_management", Title: "10.3 Change Management", Order: 3, PromptHint: "Procedures for managing changes to the AI system"},
				},
			},
		},
	}
}

// GetTemplateByID returns a document template by its identifier.
func GetTemplateByID(id string) (DocumentTemplate, bool) {
	switch id {
	case "annex_iv":
		return AnnexIVTemplate(), true
	default:
		return DocumentTemplate{}, false
	}
}
