// Package compliance provides types and logic for FIPS compliance state aggregation.
package compliance

// Status represents the compliance status of a checklist item.
type Status string

const (
	StatusPass    Status = "pass"
	StatusFail    Status = "fail"
	StatusWarning Status = "warning"
	StatusUnknown Status = "unknown"
)

// Section represents a named group of compliance checklist items.
type Section struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Items       []ChecklistItem `json:"items"`
}

// ChecklistItem represents a single compliance checklist entry.
type ChecklistItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Status      Status `json:"status"`
	Severity    string `json:"severity"`
	What        string `json:"what"`
	Why         string `json:"why"`
	Remediation string `json:"remediation"`
	NISTRef     string `json:"nist_ref"`
}

// ComplianceReport is the top-level compliance state.
type ComplianceReport struct {
	Timestamp string    `json:"timestamp"`
	Sections  []Section `json:"sections"`
	Summary   Summary   `json:"summary"`
}

// Summary holds aggregate compliance counts.
type Summary struct {
	Total    int `json:"total"`
	Passed   int `json:"passed"`
	Failed   int `json:"failed"`
	Warnings int `json:"warnings"`
	Unknown  int `json:"unknown"`
}
