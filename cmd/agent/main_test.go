package main

import (
	"os"
	"testing"

	"github.com/cloudflared-fips/cloudflared-fips/internal/compliance"
)

// ---------------------------------------------------------------------------
// envOrFlag
// ---------------------------------------------------------------------------

func TestEnvOrFlag_FlagPriority(t *testing.T) {
	os.Setenv("TEST_ENVORFLAG_KEY", "env-value")
	defer os.Unsetenv("TEST_ENVORFLAG_KEY")

	got := envOrFlag("flag-value", "TEST_ENVORFLAG_KEY")
	if got != "flag-value" {
		t.Errorf("envOrFlag with flag set = %q, want flag-value", got)
	}
}

func TestEnvOrFlag_EnvFallback(t *testing.T) {
	os.Setenv("TEST_ENVORFLAG_KEY2", "env-value")
	defer os.Unsetenv("TEST_ENVORFLAG_KEY2")

	got := envOrFlag("", "TEST_ENVORFLAG_KEY2")
	if got != "env-value" {
		t.Errorf("envOrFlag with empty flag = %q, want env-value", got)
	}
}

func TestEnvOrFlag_BothEmpty(t *testing.T) {
	os.Unsetenv("TEST_ENVORFLAG_NOEXIST")
	got := envOrFlag("", "TEST_ENVORFLAG_NOEXIST")
	if got != "" {
		t.Errorf("envOrFlag both empty = %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// printSection — verify it doesn't panic on various inputs
// ---------------------------------------------------------------------------

func TestPrintSection_NoItems(t *testing.T) {
	section := compliance.Section{
		Name:  "Empty",
		Items: nil,
	}
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printSection(section)

	w.Close()
	os.Stdout = old

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if output == "" {
		t.Error("printSection should produce output even with empty section")
	}
	if !contains(output, "Empty") {
		t.Errorf("output should contain section name 'Empty', got: %q", output)
	}
	if !contains(output, "0 passed") {
		t.Errorf("output should show 0 passed, got: %q", output)
	}
}

func TestPrintSection_MixedStatuses(t *testing.T) {
	section := compliance.Section{
		Name: "Mixed",
		Items: []compliance.ChecklistItem{
			{Name: "check1", Status: compliance.StatusPass},
			{Name: "check2", Status: compliance.StatusFail, Remediation: "fix it"},
			{Name: "check3", Status: compliance.StatusWarning, Remediation: "review"},
		},
	}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printSection(section)

	w.Close()
	os.Stdout = old

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if !contains(output, "1 passed") {
		t.Errorf("should show 1 passed, got: %q", output)
	}
	if !contains(output, "1 failed") {
		t.Errorf("should show 1 failed, got: %q", output)
	}
	if !contains(output, "1 warnings") {
		t.Errorf("should show 1 warnings, got: %q", output)
	}
	if !contains(output, "fix it") {
		t.Errorf("should show remediation for failed item, got: %q", output)
	}
	if !contains(output, "review") {
		t.Errorf("should show remediation for warn item, got: %q", output)
	}
}

func TestPrintSection_PassingItemsNoRemediation(t *testing.T) {
	section := compliance.Section{
		Name: "AllPass",
		Items: []compliance.ChecklistItem{
			{Name: "ok1", Status: compliance.StatusPass, Remediation: "should not show"},
		},
	}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printSection(section)

	w.Close()
	os.Stdout = old

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	// Passing items should NOT show remediation
	if contains(output, "should not show") {
		t.Errorf("passing items should not display remediation, got: %q", output)
	}
}

func TestPrintSection_StatusIcons(t *testing.T) {
	section := compliance.Section{
		Name: "Icons",
		Items: []compliance.ChecklistItem{
			{Name: "pass", Status: compliance.StatusPass},
			{Name: "fail", Status: compliance.StatusFail},
			{Name: "warn", Status: compliance.StatusWarning},
			{Name: "other", Status: "other"},
		},
	}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printSection(section)

	w.Close()
	os.Stdout = old

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if !contains(output, "✓") {
		t.Error("should contain ✓ for pass")
	}
	if !contains(output, "✗") {
		t.Error("should contain ✗ for fail")
	}
	if !contains(output, "!") {
		t.Error("should contain ! for warning")
	}
	if !contains(output, "?") {
		t.Error("should contain ? for unknown status")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
