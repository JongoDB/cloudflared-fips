package status

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cloudflared-fips/cloudflared-fips/internal/compliance"
)

// ---------------------------------------------------------------------------
// statusIcon
// ---------------------------------------------------------------------------

func TestStatusIcon(t *testing.T) {
	tests := []struct {
		status compliance.Status
		want   string // substring (the glyph)
	}{
		{compliance.StatusPass, "●"},
		{compliance.StatusWarning, "○"},
		{compliance.StatusFail, "✖"},
		{"unknown", "?"},
		{"", "?"},
	}
	for _, tt := range tests {
		got := statusIcon(tt.status)
		if !strings.Contains(got, tt.want) {
			t.Errorf("statusIcon(%q) = %q, want substring %q", tt.status, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// statusLabel
// ---------------------------------------------------------------------------

func TestStatusLabel(t *testing.T) {
	tests := []struct {
		status compliance.Status
		want   string
	}{
		{compliance.StatusPass, "PASS"},
		{compliance.StatusWarning, "WARN"},
		{compliance.StatusFail, "FAIL"},
		{"unknown", "UNKN"},
		{"", "UNKN"},
	}
	for _, tt := range tests {
		got := statusLabel(tt.status)
		if !strings.Contains(got, tt.want) {
			t.Errorf("statusLabel(%q) = %q, want substring %q", tt.status, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// renderItem
// ---------------------------------------------------------------------------

func TestRenderItem(t *testing.T) {
	tests := []struct {
		name   string
		status compliance.Status
	}{
		{"pass item", compliance.StatusPass},
		{"warn item", compliance.StatusWarning},
		{"fail item", compliance.StatusFail},
		{"unknown item", "unknown"},
	}
	for _, tt := range tests {
		item := compliance.ChecklistItem{
			Name:   tt.name,
			Status: tt.status,
		}
		got := renderItem(item)
		if got == "" {
			t.Errorf("renderItem(%s) returned empty string", tt.name)
		}
		// Should contain the name (possibly styled)
		if !strings.Contains(got, tt.name) {
			t.Errorf("renderItem(%s) missing item name in output", tt.name)
		}
	}
}

// ---------------------------------------------------------------------------
// renderSection
// ---------------------------------------------------------------------------

func TestRenderSection_Empty(t *testing.T) {
	section := compliance.Section{
		Name:  "Empty Section",
		Items: nil,
	}
	got := renderSection(section)
	if !strings.Contains(got, "Empty Section") {
		t.Error("renderSection should contain section name")
	}
	if !strings.Contains(got, "0/0 pass") {
		t.Errorf("renderSection should show 0/0 pass, got: %q", got)
	}
}

func TestRenderSection_MixedStatuses(t *testing.T) {
	section := compliance.Section{
		Name: "Mixed",
		Items: []compliance.ChecklistItem{
			{Name: "item1", Status: compliance.StatusPass},
			{Name: "item2", Status: compliance.StatusPass},
			{Name: "item3", Status: compliance.StatusFail},
		},
	}
	got := renderSection(section)
	if !strings.Contains(got, "Mixed") {
		t.Error("renderSection should contain section name")
	}
	if !strings.Contains(got, "2/3 pass") {
		t.Errorf("renderSection should show 2/3 pass, got: %q", got)
	}
}

func TestRenderSection_AllPassing(t *testing.T) {
	section := compliance.Section{
		Name: "All Pass",
		Items: []compliance.ChecklistItem{
			{Name: "a", Status: compliance.StatusPass},
			{Name: "b", Status: compliance.StatusPass},
		},
	}
	got := renderSection(section)
	if !strings.Contains(got, "2/2 pass") {
		t.Errorf("expected 2/2 pass, got: %q", got)
	}
}

// ---------------------------------------------------------------------------
// renderSummaryBar
// ---------------------------------------------------------------------------

func TestRenderSummaryBar_ZeroTotal(t *testing.T) {
	got := renderSummaryBar(compliance.Summary{}, 80)
	if !strings.Contains(got, "No compliance data") {
		t.Errorf("zero total should show 'No compliance data', got: %q", got)
	}
}

func TestRenderSummaryBar_AllPassing(t *testing.T) {
	summary := compliance.Summary{
		Total:  10,
		Passed: 10,
	}
	got := renderSummaryBar(summary, 80)
	if !strings.Contains(got, "10/10") {
		t.Errorf("should show 10/10, got: %q", got)
	}
	if !strings.Contains(got, "100%") {
		t.Errorf("should show 100%%, got: %q", got)
	}
}

func TestRenderSummaryBar_MixedStatuses(t *testing.T) {
	summary := compliance.Summary{
		Total:    10,
		Passed:   7,
		Warnings: 2,
		Failed:   1,
	}
	got := renderSummaryBar(summary, 80)
	if !strings.Contains(got, "7/10") {
		t.Errorf("should show 7/10, got: %q", got)
	}
	if !strings.Contains(got, "WARN") {
		t.Errorf("should show WARN count, got: %q", got)
	}
	if !strings.Contains(got, "FAIL") {
		t.Errorf("should show FAIL count, got: %q", got)
	}
}

func TestRenderSummaryBar_WideTerminal(t *testing.T) {
	summary := compliance.Summary{Total: 5, Passed: 5}
	got80 := renderSummaryBar(summary, 80)
	got120 := renderSummaryBar(summary, 120)
	// Wider terminal should produce wider bar (more █ characters)
	count80 := strings.Count(got80, "█")
	count120 := strings.Count(got120, "█")
	if count120 <= count80 {
		t.Errorf("wider terminal should produce wider bar: 80=%d chars, 120=%d chars", count80, count120)
	}
}

func TestRenderSummaryBar_WithUnknown(t *testing.T) {
	summary := compliance.Summary{
		Total:   5,
		Passed:  2,
		Unknown: 3,
	}
	got := renderSummaryBar(summary, 80)
	if !strings.Contains(got, "UNKN") {
		t.Errorf("should show UNKN count, got: %q", got)
	}
}

// ---------------------------------------------------------------------------
// NewStatusModel
// ---------------------------------------------------------------------------

func TestNewStatusModel(t *testing.T) {
	m := NewStatusModel("localhost:8080", 5*time.Second)
	if m.apiAddr != "localhost:8080" {
		t.Errorf("apiAddr = %q, want localhost:8080", m.apiAddr)
	}
	if m.interval != 5*time.Second {
		t.Errorf("interval = %v, want 5s", m.interval)
	}
	if m.ready {
		t.Error("should not be ready initially")
	}
	if m.report != nil {
		t.Error("report should be nil initially")
	}
}

// ---------------------------------------------------------------------------
// renderLastUpdate
// ---------------------------------------------------------------------------

func TestRenderLastUpdate_ZeroTime(t *testing.T) {
	m := NewStatusModel("localhost:8080", 5*time.Second)
	got := m.renderLastUpdate()
	if !strings.Contains(got, "Connecting") {
		t.Errorf("zero time should show 'Connecting...', got: %q", got)
	}
}

func TestRenderLastUpdate_WithTime(t *testing.T) {
	m := NewStatusModel("localhost:8080", 5*time.Second)
	m.lastPoll = time.Date(2026, 2, 28, 14, 30, 45, 0, time.UTC)
	got := m.renderLastUpdate()
	if !strings.Contains(got, "14:30:45") {
		t.Errorf("should show formatted time, got: %q", got)
	}
	if !strings.Contains(got, "Updated") {
		t.Errorf("should contain 'Updated', got: %q", got)
	}
}

// ---------------------------------------------------------------------------
// renderContent
// ---------------------------------------------------------------------------

func TestRenderContent_Error(t *testing.T) {
	m := NewStatusModel("localhost:8080", 5*time.Second)
	m.err = fmt.Errorf("connection refused")
	got := m.renderContent()
	if !strings.Contains(got, "connection refused") {
		t.Errorf("should show error message, got: %q", got)
	}
	if !strings.Contains(got, "retry") {
		t.Errorf("should suggest retry, got: %q", got)
	}
}

func TestRenderContent_NilReport(t *testing.T) {
	m := NewStatusModel("localhost:8080", 5*time.Second)
	got := m.renderContent()
	if !strings.Contains(got, "Waiting") {
		t.Errorf("nil report should show 'Waiting...', got: %q", got)
	}
}

func TestRenderContent_WithReport(t *testing.T) {
	m := NewStatusModel("localhost:8080", 5*time.Second)
	m.report = &compliance.ComplianceReport{
		Summary: compliance.Summary{
			Total:  3,
			Passed: 2,
			Failed: 1,
		},
		Sections: []compliance.Section{
			{
				Name: "Test Section",
				Items: []compliance.ChecklistItem{
					{Name: "check1", Status: compliance.StatusPass},
					{Name: "check2", Status: compliance.StatusPass},
					{Name: "check3", Status: compliance.StatusFail},
				},
			},
		},
	}
	got := m.renderContent()
	if !strings.Contains(got, "Test Section") {
		t.Errorf("should contain section name, got: %q", got)
	}
	if !strings.Contains(got, "2/3") {
		t.Errorf("should contain summary counts, got: %q", got)
	}
}

// ---------------------------------------------------------------------------
// renderFooter
// ---------------------------------------------------------------------------

func TestRenderFooter_Connected(t *testing.T) {
	m := NewStatusModel("localhost:8080", 10*time.Second)
	got := m.renderFooter()
	if !strings.Contains(got, "Connected") {
		t.Errorf("no error should show 'Connected', got: %q", got)
	}
	if !strings.Contains(got, "localhost:8080") {
		t.Errorf("should show API address, got: %q", got)
	}
	if !strings.Contains(got, "10s") {
		t.Errorf("should show polling interval, got: %q", got)
	}
}

func TestRenderFooter_Disconnected(t *testing.T) {
	m := NewStatusModel("localhost:8080", 10*time.Second)
	m.err = fmt.Errorf("timeout")
	got := m.renderFooter()
	if !strings.Contains(got, "Disconnected") {
		t.Errorf("with error should show 'Disconnected', got: %q", got)
	}
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func TestView_NotReady(t *testing.T) {
	m := NewStatusModel("localhost:8080", 5*time.Second)
	got := m.View()
	if !strings.Contains(got, "Initializing") {
		t.Errorf("not-ready model should show 'Initializing', got: %q", got)
	}
}

// ---------------------------------------------------------------------------
// Update — message handling
// ---------------------------------------------------------------------------

func TestUpdate_WindowSizeMsg(t *testing.T) {
	m := NewStatusModel("localhost:8080", 5*time.Second)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	model := updated.(StatusModel)
	if !model.ready {
		t.Error("should be ready after WindowSizeMsg")
	}
	if model.width != 100 {
		t.Errorf("width = %d, want 100", model.width)
	}
	if model.height != 40 {
		t.Errorf("height = %d, want 40", model.height)
	}
}

func TestUpdate_PollMsg_Success(t *testing.T) {
	m := NewStatusModel("localhost:8080", 5*time.Second)
	report := &compliance.ComplianceReport{
		Summary: compliance.Summary{Total: 1, Passed: 1},
	}
	updated, _ := m.Update(pollMsg{report: report})
	model := updated.(StatusModel)
	if model.report == nil {
		t.Error("report should be set after successful poll")
	}
	if model.err != nil {
		t.Error("err should be nil after successful poll")
	}
	if model.lastPoll.IsZero() {
		t.Error("lastPoll should be set")
	}
}

func TestUpdate_PollMsg_Error(t *testing.T) {
	m := NewStatusModel("localhost:8080", 5*time.Second)
	updated, _ := m.Update(pollMsg{err: fmt.Errorf("connection refused")})
	model := updated.(StatusModel)
	if model.err == nil {
		t.Error("err should be set after failed poll")
	}
	// lastPoll is always set, even on error -- no assertion needed
}

func TestUpdate_PollMsg_ClearsError(t *testing.T) {
	m := NewStatusModel("localhost:8080", 5*time.Second)
	m.err = fmt.Errorf("previous error")
	report := &compliance.ComplianceReport{
		Summary: compliance.Summary{Total: 1, Passed: 1},
	}
	updated, _ := m.Update(pollMsg{report: report})
	model := updated.(StatusModel)
	if model.err != nil {
		t.Error("successful poll should clear previous error")
	}
}

func TestUpdate_WindowSizeMsg_SmallHeight(t *testing.T) {
	m := NewStatusModel("localhost:8080", 5*time.Second)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 8})
	model := updated.(StatusModel)
	// contentH should be clamped to minimum 5
	if model.viewport.Height < 5 {
		t.Errorf("viewport height = %d, should be at least 5", model.viewport.Height)
	}
}

func TestUpdate_WindowSizeMsg_Resize(t *testing.T) {
	m := NewStatusModel("localhost:8080", 5*time.Second)
	// First size
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	model := updated.(StatusModel)
	if !model.ready {
		t.Fatal("should be ready")
	}
	// Resize
	updated2, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 50})
	model2 := updated2.(StatusModel)
	if model2.width != 120 {
		t.Errorf("width after resize = %d, want 120", model2.width)
	}
	if model2.viewport.Width != 120 {
		t.Errorf("viewport width after resize = %d, want 120", model2.viewport.Width)
	}
}
