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
