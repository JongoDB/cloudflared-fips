package compliance

import (
	"testing"
	"time"
)

// --- Test helpers ---

// makeItem creates a ChecklistItem with the given id and status.
// Other fields are filled with sensible defaults.
func makeItem(id string, status Status) ChecklistItem {
	return ChecklistItem{
		ID:                 id,
		Name:               "Check " + id,
		Status:             status,
		Severity:           "high",
		VerificationMethod: VerifyDirect,
		What:               "Verifies " + id,
		Why:                "Required for FIPS compliance",
		Remediation:        "Fix " + id,
		NISTRef:            "SC-13",
	}
}

// makeSection creates a Section containing the given items.
func makeSection(id, name string, items ...ChecklistItem) Section {
	return Section{
		ID:          id,
		Name:        name,
		Description: "Test section: " + name,
		Items:       items,
	}
}

// --- Tests ---

func TestNewChecker(t *testing.T) {
	c := NewChecker()
	if c == nil {
		t.Fatal("NewChecker returned nil")
	}
	if len(c.sections) != 0 {
		t.Errorf("expected 0 sections, got %d", len(c.sections))
	}
}

func TestAddSection(t *testing.T) {
	c := NewChecker()

	s1 := makeSection("s1", "Section One", makeItem("item-1", StatusPass))
	s2 := makeSection("s2", "Section Two", makeItem("item-2", StatusFail))

	c.AddSection(s1)
	if len(c.sections) != 1 {
		t.Fatalf("expected 1 section after first add, got %d", len(c.sections))
	}
	if c.sections[0].ID != "s1" {
		t.Errorf("expected section id %q, got %q", "s1", c.sections[0].ID)
	}

	c.AddSection(s2)
	if len(c.sections) != 2 {
		t.Fatalf("expected 2 sections after second add, got %d", len(c.sections))
	}
	if c.sections[1].ID != "s2" {
		t.Errorf("expected section id %q, got %q", "s2", c.sections[1].ID)
	}
}

func TestGenerateReport_NoSections(t *testing.T) {
	c := NewChecker()
	report := c.GenerateReport()

	if report == nil {
		t.Fatal("GenerateReport returned nil")
	}
	if len(report.Sections) != 0 {
		t.Errorf("expected 0 sections, got %d", len(report.Sections))
	}
	if report.Summary.Total != 0 {
		t.Errorf("expected Total=0, got %d", report.Summary.Total)
	}
	if report.Summary.Passed != 0 {
		t.Errorf("expected Passed=0, got %d", report.Summary.Passed)
	}
	if report.Summary.Failed != 0 {
		t.Errorf("expected Failed=0, got %d", report.Summary.Failed)
	}
	if report.Summary.Warnings != 0 {
		t.Errorf("expected Warnings=0, got %d", report.Summary.Warnings)
	}
	if report.Summary.Unknown != 0 {
		t.Errorf("expected Unknown=0, got %d", report.Summary.Unknown)
	}
}

func TestGenerateReport_MixedStatuses(t *testing.T) {
	c := NewChecker()
	c.AddSection(makeSection("s1", "Mixed",
		makeItem("p1", StatusPass),
		makeItem("p2", StatusPass),
		makeItem("f1", StatusFail),
		makeItem("w1", StatusWarning),
		makeItem("u1", StatusUnknown),
	))

	report := c.GenerateReport()

	if report.Summary.Total != 5 {
		t.Errorf("expected Total=5, got %d", report.Summary.Total)
	}
	if report.Summary.Passed != 2 {
		t.Errorf("expected Passed=2, got %d", report.Summary.Passed)
	}
	if report.Summary.Failed != 1 {
		t.Errorf("expected Failed=1, got %d", report.Summary.Failed)
	}
	if report.Summary.Warnings != 1 {
		t.Errorf("expected Warnings=1, got %d", report.Summary.Warnings)
	}
	if report.Summary.Unknown != 1 {
		t.Errorf("expected Unknown=1, got %d", report.Summary.Unknown)
	}
}

func TestGenerateReport_TimestampIsRFC3339(t *testing.T) {
	c := NewChecker()
	report := c.GenerateReport()

	_, err := time.Parse(time.RFC3339, report.Timestamp)
	if err != nil {
		t.Errorf("timestamp %q is not valid RFC3339: %v", report.Timestamp, err)
	}
}

func TestGenerateReport_SectionsPreserved(t *testing.T) {
	c := NewChecker()
	s1 := makeSection("sec-a", "Alpha", makeItem("a1", StatusPass))
	s2 := makeSection("sec-b", "Beta", makeItem("b1", StatusFail), makeItem("b2", StatusWarning))
	c.AddSection(s1)
	c.AddSection(s2)

	report := c.GenerateReport()

	if len(report.Sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(report.Sections))
	}
	if report.Sections[0].ID != "sec-a" {
		t.Errorf("expected first section id %q, got %q", "sec-a", report.Sections[0].ID)
	}
	if report.Sections[1].ID != "sec-b" {
		t.Errorf("expected second section id %q, got %q", "sec-b", report.Sections[1].ID)
	}
	if len(report.Sections[1].Items) != 2 {
		t.Errorf("expected 2 items in second section, got %d", len(report.Sections[1].Items))
	}
}

func TestOverallStatus_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		statuses []Status
		want     Status
	}{
		{
			name:     "all pass",
			statuses: []Status{StatusPass, StatusPass, StatusPass},
			want:     StatusPass,
		},
		{
			name:     "single pass",
			statuses: []Status{StatusPass},
			want:     StatusPass,
		},
		{
			name:     "fail overrides all",
			statuses: []Status{StatusPass, StatusWarning, StatusUnknown, StatusFail},
			want:     StatusFail,
		},
		{
			name:     "fail alone",
			statuses: []Status{StatusFail},
			want:     StatusFail,
		},
		{
			name:     "warning without fail",
			statuses: []Status{StatusPass, StatusWarning, StatusPass},
			want:     StatusWarning,
		},
		{
			name:     "warning and unknown without fail",
			statuses: []Status{StatusPass, StatusWarning, StatusUnknown},
			want:     StatusWarning,
		},
		{
			name:     "unknown without fail or warning",
			statuses: []Status{StatusPass, StatusUnknown, StatusPass},
			want:     StatusUnknown,
		},
		{
			name:     "unknown alone",
			statuses: []Status{StatusUnknown},
			want:     StatusUnknown,
		},
		{
			name:     "no items returns pass",
			statuses: []Status{},
			want:     StatusPass,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewChecker()
			items := make([]ChecklistItem, len(tt.statuses))
			for i, s := range tt.statuses {
				items[i] = makeItem("item-"+string(rune('a'+i)), s)
			}
			c.AddSection(makeSection("test", "Test", items...))

			got := c.OverallStatus()
			if got != tt.want {
				t.Errorf("OverallStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOverallStatus_EmptyChecker(t *testing.T) {
	c := NewChecker()
	got := c.OverallStatus()
	if got != StatusPass {
		t.Errorf("OverallStatus() on empty checker = %q, want %q", got, StatusPass)
	}
}

func TestMultipleSections_AggregateCorrectly(t *testing.T) {
	c := NewChecker()

	// Section 1: 2 pass, 1 warning
	c.AddSection(makeSection("client", "Client Posture",
		makeItem("c1", StatusPass),
		makeItem("c2", StatusPass),
		makeItem("c3", StatusWarning),
	))

	// Section 2: 1 pass, 1 fail
	c.AddSection(makeSection("tunnel", "Tunnel",
		makeItem("t1", StatusPass),
		makeItem("t2", StatusFail),
	))

	// Section 3: 1 unknown
	c.AddSection(makeSection("build", "Build",
		makeItem("b1", StatusUnknown),
	))

	report := c.GenerateReport()

	if report.Summary.Total != 6 {
		t.Errorf("expected Total=6, got %d", report.Summary.Total)
	}
	if report.Summary.Passed != 3 {
		t.Errorf("expected Passed=3, got %d", report.Summary.Passed)
	}
	if report.Summary.Failed != 1 {
		t.Errorf("expected Failed=1, got %d", report.Summary.Failed)
	}
	if report.Summary.Warnings != 1 {
		t.Errorf("expected Warnings=1, got %d", report.Summary.Warnings)
	}
	if report.Summary.Unknown != 1 {
		t.Errorf("expected Unknown=1, got %d", report.Summary.Unknown)
	}

	// Overall status should be fail (worst across all sections)
	overall := c.OverallStatus()
	if overall != StatusFail {
		t.Errorf("OverallStatus() = %q, want %q", overall, StatusFail)
	}
}

func TestOverallStatus_FailEarlyReturn(t *testing.T) {
	// Verify that a fail in the first section is detected even with
	// many subsequent items. This exercises the early-return path.
	c := NewChecker()
	c.AddSection(makeSection("s1", "First",
		makeItem("f1", StatusFail),
	))
	c.AddSection(makeSection("s2", "Second",
		makeItem("p1", StatusPass),
		makeItem("p2", StatusPass),
		makeItem("p3", StatusPass),
	))

	got := c.OverallStatus()
	if got != StatusFail {
		t.Errorf("OverallStatus() = %q, want %q (fail should short-circuit)", got, StatusFail)
	}
}

func TestGenerateReport_AllSameStatus(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		count  int
	}{
		{"all pass", StatusPass, 4},
		{"all fail", StatusFail, 3},
		{"all warning", StatusWarning, 2},
		{"all unknown", StatusUnknown, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewChecker()
			items := make([]ChecklistItem, tt.count)
			for i := range items {
				items[i] = makeItem("item", tt.status)
			}
			c.AddSection(makeSection("s", "Section", items...))

			report := c.GenerateReport()
			if report.Summary.Total != tt.count {
				t.Errorf("Total = %d, want %d", report.Summary.Total, tt.count)
			}

			var got int
			switch tt.status {
			case StatusPass:
				got = report.Summary.Passed
			case StatusFail:
				got = report.Summary.Failed
			case StatusWarning:
				got = report.Summary.Warnings
			case StatusUnknown:
				got = report.Summary.Unknown
			}
			if got != tt.count {
				t.Errorf("count for status %q = %d, want %d", tt.status, got, tt.count)
			}
		})
	}
}

func TestChecklistItemFieldsPreserved(t *testing.T) {
	item := ChecklistItem{
		ID:                 "bc-active",
		Name:               "BoringCrypto active",
		Status:             StatusPass,
		Severity:           "critical",
		VerificationMethod: VerifyDirect,
		What:               "Checks BoringCrypto is loaded",
		Why:                "FIPS 140-2 requires validated module",
		Remediation:        "Rebuild with GOEXPERIMENT=boringcrypto",
		NISTRef:            "SC-13",
	}

	c := NewChecker()
	c.AddSection(Section{
		ID:          "tunnel",
		Name:        "Tunnel",
		Description: "Tunnel compliance checks",
		Items:       []ChecklistItem{item},
	})

	report := c.GenerateReport()
	got := report.Sections[0].Items[0]

	if got.ID != item.ID {
		t.Errorf("ID = %q, want %q", got.ID, item.ID)
	}
	if got.Name != item.Name {
		t.Errorf("Name = %q, want %q", got.Name, item.Name)
	}
	if got.Status != item.Status {
		t.Errorf("Status = %q, want %q", got.Status, item.Status)
	}
	if got.Severity != item.Severity {
		t.Errorf("Severity = %q, want %q", got.Severity, item.Severity)
	}
	if got.VerificationMethod != item.VerificationMethod {
		t.Errorf("VerificationMethod = %q, want %q", got.VerificationMethod, item.VerificationMethod)
	}
	if got.What != item.What {
		t.Errorf("What = %q, want %q", got.What, item.What)
	}
	if got.Why != item.Why {
		t.Errorf("Why = %q, want %q", got.Why, item.Why)
	}
	if got.Remediation != item.Remediation {
		t.Errorf("Remediation = %q, want %q", got.Remediation, item.Remediation)
	}
	if got.NISTRef != item.NISTRef {
		t.Errorf("NISTRef = %q, want %q", got.NISTRef, item.NISTRef)
	}
}
