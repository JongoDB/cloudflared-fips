package compliance

import "time"

// Checker aggregates compliance state from multiple sources.
type Checker struct {
	sections []Section
}

// NewChecker creates a new compliance checker.
func NewChecker() *Checker {
	return &Checker{}
}

// AddSection adds a compliance section to the checker.
func (c *Checker) AddSection(section Section) {
	c.sections = append(c.sections, section)
}

// GenerateReport produces a compliance report from all registered sections.
func (c *Checker) GenerateReport() *ComplianceReport {
	report := &ComplianceReport{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Sections:  c.sections,
	}

	for _, section := range c.sections {
		for _, item := range section.Items {
			report.Summary.Total++
			switch item.Status {
			case StatusPass:
				report.Summary.Passed++
			case StatusFail:
				report.Summary.Failed++
			case StatusWarning:
				report.Summary.Warnings++
			case StatusUnknown:
				report.Summary.Unknown++
			}
		}
	}

	return report
}

// OverallStatus returns the worst-case status across all items.
func (c *Checker) OverallStatus() Status {
	worst := StatusPass
	for _, section := range c.sections {
		for _, item := range section.Items {
			if item.Status == StatusFail {
				return StatusFail
			}
			if item.Status == StatusWarning && worst != StatusFail {
				worst = StatusWarning
			}
			if item.Status == StatusUnknown && worst == StatusPass {
				worst = StatusUnknown
			}
		}
	}
	return worst
}
