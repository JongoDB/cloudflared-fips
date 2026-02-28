package wizard

import (
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
