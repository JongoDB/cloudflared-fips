package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateUUID(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"550e8400-e29b-41d4-a716-446655440000", true},
		{"ABCDEF12-3456-7890-abcd-ef1234567890", true},
		{"", false},
		{"not-a-uuid", false},
		{"550e8400e29b41d4a716446655440000", false},       // no dashes
		{"550e8400-e29b-41d4-a716-44665544000", false},     // too short
		{"550e8400-e29b-41d4-a716-4466554400000", false},   // too long
		{"550e8400-e29b-41d4-a716-44665544000g", false},    // non-hex char
	}
	for _, tt := range tests {
		err := ValidateUUID(tt.input)
		if tt.valid && err != nil {
			t.Errorf("ValidateUUID(%q) unexpected error: %v", tt.input, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("ValidateUUID(%q) expected error, got nil", tt.input)
		}
	}
}

func TestValidateHexID(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"abcdef1234567890abcdef1234567890", true},
		{"ABCDEF1234567890ABCDEF1234567890", true},
		{"", false},
		{"short", false},
		{"abcdef1234567890abcdef123456789", false},  // 31 chars
		{"abcdef1234567890abcdef12345678901", false}, // 33 chars
		{"abcdef1234567890abcdef123456789g", false},  // non-hex
	}
	for _, tt := range tests {
		err := ValidateHexID(tt.input)
		if tt.valid && err != nil {
			t.Errorf("ValidateHexID(%q) unexpected error: %v", tt.input, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("ValidateHexID(%q) expected error, got nil", tt.input)
		}
	}
}

func TestValidateHostPort(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"localhost:8080", true},
		{"127.0.0.1:2000", true},
		{"[::1]:443", true},
		{"example.com:443", true},
		{"", false},
		{"localhost", false},
		{":8080", false},
	}
	for _, tt := range tests {
		err := ValidateHostPort(tt.input)
		if tt.valid && err != nil {
			t.Errorf("ValidateHostPort(%q) unexpected error: %v", tt.input, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("ValidateHostPort(%q) expected error, got nil", tt.input)
		}
	}
}

func TestNewDefaultConfig(t *testing.T) {
	cfg := NewDefaultConfig()
	if cfg.Protocol != "quic" {
		t.Errorf("default protocol = %q, want %q", cfg.Protocol, "quic")
	}
	if cfg.DeploymentTier != "standard" {
		t.Errorf("default deployment tier = %q, want %q", cfg.DeploymentTier, "standard")
	}
	if !cfg.FIPS.SelfTestOnStart {
		t.Error("default self-test-on-start should be true")
	}
	if len(cfg.Ingress) < 1 || cfg.Ingress[len(cfg.Ingress)-1].Service != "http_status:404" {
		t.Error("default ingress should end with catch-all 404")
	}
}

func TestWriteAndReadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-config.yaml")

	cfg := NewDefaultConfig()
	cfg.Tunnel = "550e8400-e29b-41d4-a716-446655440000"
	cfg.Dashboard.ZoneID = "abcdef1234567890abcdef1234567890"

	if err := WriteConfig(path, cfg); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file not created: %v", err)
	}

	// Read back
	got, err := ReadConfig(path)
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	if got.Tunnel != cfg.Tunnel {
		t.Errorf("tunnel = %q, want %q", got.Tunnel, cfg.Tunnel)
	}
	if got.Dashboard.ZoneID != cfg.Dashboard.ZoneID {
		t.Errorf("zone-id = %q, want %q", got.Dashboard.ZoneID, cfg.Dashboard.ZoneID)
	}
	if got.Protocol != "quic" {
		t.Errorf("protocol = %q, want %q", got.Protocol, "quic")
	}
}

// ---------------------------------------------------------------------------
// ValidateNonEmpty
// ---------------------------------------------------------------------------

func TestValidateNonEmpty(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"hello", true},
		{"  spaced  ", true},
		{"", false},
		{"   ", false},
		{"\t\n", false},
	}
	for _, tt := range tests {
		err := ValidateNonEmpty(tt.input)
		if tt.valid && err != nil {
			t.Errorf("ValidateNonEmpty(%q) unexpected error: %v", tt.input, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("ValidateNonEmpty(%q) expected error, got nil", tt.input)
		}
	}
}

// ---------------------------------------------------------------------------
// ValidateOptionalHexID
// ---------------------------------------------------------------------------

func TestValidateOptionalHexID(t *testing.T) {
	// Empty is OK
	if err := ValidateOptionalHexID(""); err != nil {
		t.Errorf("empty should be valid: %v", err)
	}
	if err := ValidateOptionalHexID("  "); err != nil {
		t.Errorf("whitespace should be valid: %v", err)
	}
	// Valid hex ID
	if err := ValidateOptionalHexID("abcdef1234567890abcdef1234567890"); err != nil {
		t.Errorf("valid hex ID should pass: %v", err)
	}
	// Invalid hex ID
	if err := ValidateOptionalHexID("short"); err == nil {
		t.Error("invalid hex ID should fail")
	}
}

// ---------------------------------------------------------------------------
// ValidateOptionalHostPort
// ---------------------------------------------------------------------------

func TestValidateOptionalHostPort(t *testing.T) {
	if err := ValidateOptionalHostPort(""); err != nil {
		t.Errorf("empty should be valid: %v", err)
	}
	if err := ValidateOptionalHostPort("localhost:8080"); err != nil {
		t.Errorf("valid host:port should pass: %v", err)
	}
	if err := ValidateOptionalHostPort("invalid"); err == nil {
		t.Error("invalid host:port should fail")
	}
}

// ---------------------------------------------------------------------------
// ValidateFileExists
// ---------------------------------------------------------------------------

func TestValidateFileExists(t *testing.T) {
	// Empty path
	if err := ValidateFileExists(""); err == nil {
		t.Error("empty path should fail")
	}

	// Nonexistent path
	if err := ValidateFileExists("/nonexistent/file.txt"); err == nil {
		t.Error("nonexistent file should fail")
	}

	// Existing file
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateFileExists(path); err != nil {
		t.Errorf("existing file should pass: %v", err)
	}

	// Directory (not a file)
	if err := ValidateFileExists(dir); err == nil {
		t.Error("directory should fail (not a file)")
	}
}

// ---------------------------------------------------------------------------
// ReadConfig — invalid YAML
// ---------------------------------------------------------------------------

func TestReadConfig_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte("{{{{invalid yaml"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := ReadConfig(path)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestReadConfig_NonexistentFile(t *testing.T) {
	_, err := ReadConfig("/nonexistent/config.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
