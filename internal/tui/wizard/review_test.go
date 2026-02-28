package wizard

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// last4
// ---------------------------------------------------------------------------

func TestLast4(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"abcdefghij", "ghij"},
		{"12345678", "5678"},
		{"abcd", "****"},
		{"abc", "***"},
		{"ab", "**"},
		{"a", "*"},
		{"", ""},
		{"secret-api-token-xyz", "-xyz"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := last4(tt.input)
			if got != tt.want {
				t.Errorf("last4(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// displayMDM
// ---------------------------------------------------------------------------

func TestDisplayMDM(t *testing.T) {
	tests := []struct {
		provider string
		want     string
	}{
		{"intune", "Microsoft Intune"},
		{"jamf", "Jamf Pro"},
		{"none", "None"},
		{"", "None"},
		{"custom", "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			got := displayMDM(tt.provider)
			if got != tt.want {
				t.Errorf("displayMDM(%q) = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// renderProgress
// ---------------------------------------------------------------------------

func TestRenderProgress(t *testing.T) {
	tests := []struct {
		current int
		total   int
		title   string
		wants   []string // substrings that should appear
	}{
		{1, 3, "Tunnel", []string{"Step 1 of 3", "Tunnel"}},
		{2, 5, "Dashboard", []string{"Step 2 of 5", "Dashboard"}},
		{3, 3, "Review", []string{"Step 3 of 3", "Review"}},
	}

	for _, tt := range tests {
		got := renderProgress(tt.current, tt.total, tt.title)
		for _, want := range tt.wants {
			if !strings.Contains(got, want) {
				t.Errorf("renderProgress(%d, %d, %q) missing %q in output: %q",
					tt.current, tt.total, tt.title, want, got)
			}
		}
	}
}

func TestRenderProgress_DotCount(t *testing.T) {
	got := renderProgress(2, 4, "Test")
	// Should contain both ● (filled) and ○ (empty) dot characters
	if !strings.Contains(got, "●") {
		t.Error("renderProgress should contain ● (filled dot)")
	}
	if !strings.Contains(got, "○") {
		t.Error("renderProgress should contain ○ (empty dot)")
	}
}

// ---------------------------------------------------------------------------
// renderNavHints
// ---------------------------------------------------------------------------

func TestRenderNavHints_First(t *testing.T) {
	got := renderNavHints(true, false)
	if strings.Contains(got, "Back") {
		t.Error("first page should not show Back hint")
	}
	if !strings.Contains(got, "Next") {
		t.Error("non-last page should show Next hint")
	}
	if !strings.Contains(got, "Quit") {
		t.Error("should always show Quit hint")
	}
}

func TestRenderNavHints_Middle(t *testing.T) {
	got := renderNavHints(false, false)
	if !strings.Contains(got, "Back") {
		t.Error("middle page should show Back hint")
	}
	if !strings.Contains(got, "Next") {
		t.Error("middle page should show Next hint")
	}
}

func TestRenderNavHints_Last(t *testing.T) {
	got := renderNavHints(false, true)
	if !strings.Contains(got, "Back") {
		t.Error("last page should show Back hint")
	}
	if !strings.Contains(got, "Write Config") {
		t.Error("last page should show Write Config hint")
	}
	if strings.Contains(got, "Tab/Enter=Next") {
		t.Error("last page should not show Next hint")
	}
}

func TestRenderNavHints_FirstAndLast(t *testing.T) {
	got := renderNavHints(true, true)
	if strings.Contains(got, "Back") {
		t.Error("first+last should not show Back")
	}
	if !strings.Contains(got, "Write Config") {
		t.Error("last should show Write Config")
	}
}

// ---------------------------------------------------------------------------
// parseInt
// ---------------------------------------------------------------------------

func TestParseInt(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"123", 123},
		{"0", 0},
		{"9999", 9999},
		{"", 0},
		{"abc", 0},
		{"12abc", 0},
		{"abc12", 0},
		{"42", 42},
		{"1", 1},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseInt(tt.input)
			if got != tt.want {
				t.Errorf("parseInt(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}
