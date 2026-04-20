// Package scoring provides risk scoring calculation for audit findings.
package scoring

import (
	"sovereign-ai-compliance/audit-service/internal/config"
	"sovereign-ai-compliance/audit-service/model"
)

// Calculator calculates risk scores from audit findings.
type Calculator struct {
	cfg config.RiskConfig
}

// NewCalculator creates a new risk score calculator.
func NewCalculator(cfg config.RiskConfig) *Calculator {
	return &Calculator{
		cfg: cfg,
	}
}

// CalculateCompositeScore calculates the composite risk score from finding counts.
// The score is a weighted sum: critical(50) + high(25) + medium(10) + low(1)
// Maximum score is clamped at 100.
func (c *Calculator) CalculateCompositeScore(critical, high, medium, low int) int {
	score := critical*model.SeverityWeightCritical +
		high*model.SeverityWeightHigh +
		medium*model.SeverityWeightMedium +
		low*model.SeverityWeightLow

	// Clamp at 100
	if score > 100 {
		score = 100
	}

	return score
}

// ClassifySeverity classifies the severity based on the total score.
func (c *Calculator) ClassifySeverity(score int) string {
	switch {
	case score >= c.cfg.CriticalThreshold:
		return model.RiskSeverityCritical
	case score >= c.cfg.HighThreshold:
		return model.RiskSeverityHigh
	case score >= c.cfg.MediumThreshold:
		return model.RiskSeverityMedium
	default:
		return model.RiskSeverityLow
	}
}

// IsApprovalRequired returns whether approval is required based on risk score.
// Currently requires approval for High or Critical severity.
func (c *Calculator) IsApprovalRequired(score int) bool {
	return score >= c.cfg.HighThreshold
}

// CalculateFromFindings calculates the total score and classifies severity from a list of findings.
func (c *Calculator) CalculateFromFindings(findings []model.Finding) (totalScore int, severity string, critical, high, medium, low int) {
	for _, f := range findings {
		switch f.Severity {
		case model.SeverityCritical:
			critical++
		case model.SeverityHigh:
			high++
		case model.SeverityMedium:
			medium++
		case model.SeverityLow:
			low++
		}
	}

	totalScore = c.CalculateCompositeScore(critical, high, medium, low)
	severity = c.ClassifySeverity(totalScore)

	return
}

// GroupFindingsBySeverity groups findings by severity in descending order.
func (c *Calculator) GroupFindingsBySeverity(findings []model.Finding) map[string][]model.Finding {
	groups := make(map[string][]model.Finding)
	groups[model.SeverityCritical] = make([]model.Finding, 0)
	groups[model.SeverityHigh] = make([]model.Finding, 0)
	groups[model.SeverityMedium] = make([]model.Finding, 0)
	groups[model.SeverityLow] = make([]model.Finding, 0)

	for _, f := range findings {
		groups[f.Severity] = append(groups[f.Severity], f)
	}

	return groups
}

// GroupFindingsByIssueType groups findings by EU AI Act issue type.
func (c *Calculator) GroupFindingsByIssueType(findings []model.Finding) map[string][]model.Finding {
	groups := make(map[string][]model.Finding)

	for _, f := range findings {
		groups[f.IssueType] = append(groups[f.IssueType], f)
	}

	return groups
}

// GetHighThreshold returns the high severity threshold.
func (c *Calculator) GetHighThreshold() int {
	return c.cfg.HighThreshold
}
