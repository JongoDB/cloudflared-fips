package buildinfo

import (
	"strings"
	"testing"
)

func TestDefaultValues(t *testing.T) {
	// Default values are set in the var block (not via ldflags in tests).
	if Version != "dev" {
		t.Errorf("Version = %q, want %q", Version, "dev")
	}
	if GitCommit != "unknown" {
		t.Errorf("GitCommit = %q, want %q", GitCommit, "unknown")
	}
	if BuildDate != "unknown" {
		t.Errorf("BuildDate = %q, want %q", BuildDate, "unknown")
	}
	if FIPSBuild != "false" {
		t.Errorf("FIPSBuild = %q, want %q", FIPSBuild, "false")
	}
}

func TestIsFIPSReturnsFalseByDefault(t *testing.T) {
	// FIPSBuild defaults to "false", so IsFIPS() should return false.
	if IsFIPS() {
		t.Error("IsFIPS() = true, want false when FIPSBuild is not \"true\"")
	}
}

func TestIsFIPSWithVariousValues(t *testing.T) {
	original := FIPSBuild
	defer func() { FIPSBuild = original }()

	tests := []struct {
		value    string
		expected bool
	}{
		{"true", true},
		{"false", false},
		{"", false},
		{"TRUE", false},  // case-sensitive check
		{"True", false},  // case-sensitive check
		{"1", false},
		{"yes", false},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			FIPSBuild = tt.value
			if got := IsFIPS(); got != tt.expected {
				t.Errorf("IsFIPS() with FIPSBuild=%q = %v, want %v", tt.value, got, tt.expected)
			}
		})
	}
}

func TestStringReturnsNonEmpty(t *testing.T) {
	s := String()
	if s == "" {
		t.Error("String() returned empty string")
	}
}

func TestStringContainsBuildInfo(t *testing.T) {
	s := String()
	if !strings.Contains(s, "cloudflared-fips") {
		t.Errorf("String() = %q, expected it to contain \"cloudflared-fips\"", s)
	}
	if !strings.Contains(s, Version) {
		t.Errorf("String() = %q, expected it to contain Version %q", s, Version)
	}
	if !strings.Contains(s, GitCommit) {
		t.Errorf("String() = %q, expected it to contain GitCommit %q", s, GitCommit)
	}
	if !strings.Contains(s, BuildDate) {
		t.Errorf("String() = %q, expected it to contain BuildDate %q", s, BuildDate)
	}
	if !strings.Contains(s, FIPSBuild) {
		t.Errorf("String() = %q, expected it to contain FIPSBuild %q", s, FIPSBuild)
	}
}

func TestStringFormat(t *testing.T) {
	s := String()
	// Verify the format matches: "cloudflared-fips <version> (commit: <commit>, built: <date>, fips: <fips>)"
	if !strings.HasPrefix(s, "cloudflared-fips ") {
		t.Errorf("String() = %q, expected prefix \"cloudflared-fips \"", s)
	}
	if !strings.Contains(s, "commit:") {
		t.Errorf("String() = %q, expected it to contain \"commit:\"", s)
	}
	if !strings.Contains(s, "built:") {
		t.Errorf("String() = %q, expected it to contain \"built:\"", s)
	}
	if !strings.Contains(s, "fips:") {
		t.Errorf("String() = %q, expected it to contain \"fips:\"", s)
	}
}

func TestVariablesExist(t *testing.T) {
	// Verify that the package-level variables are accessible.
	// This is a compile-time check as much as a runtime check.
	_ = Version
	_ = GitCommit
	_ = BuildDate
	_ = FIPSBuild
}
