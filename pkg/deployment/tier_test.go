package deployment

import (
	"testing"
)

func TestParseTierValidInputs(t *testing.T) {
	tests := []struct {
		input    string
		expected Tier
	}{
		// Standard tier aliases
		{"standard", TierStandard},
		{"1", TierStandard},
		{"tier1", TierStandard},
		// Regional Keyless tier aliases
		{"regional_keyless", TierRegionalKeyless},
		{"regional-keyless", TierRegionalKeyless},
		{"2", TierRegionalKeyless},
		{"tier2", TierRegionalKeyless},
		// Self-Hosted tier aliases
		{"self_hosted", TierSelfHosted},
		{"self-hosted", TierSelfHosted},
		{"3", TierSelfHosted},
		{"tier3", TierSelfHosted},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tier, err := ParseTier(tt.input)
			if err != nil {
				t.Fatalf("ParseTier(%q) returned unexpected error: %v", tt.input, err)
			}
			if tier != tt.expected {
				t.Errorf("ParseTier(%q) = %q, want %q", tt.input, tier, tt.expected)
			}
		})
	}
}

func TestParseTierCaseInsensitive(t *testing.T) {
	tier, err := ParseTier("STANDARD")
	if err != nil {
		t.Fatalf("ParseTier(\"STANDARD\") returned unexpected error: %v", err)
	}
	if tier != TierStandard {
		t.Errorf("ParseTier(\"STANDARD\") = %q, want %q", tier, TierStandard)
	}
}

func TestParseTierTrimsWhitespace(t *testing.T) {
	tier, err := ParseTier("  standard  ")
	if err != nil {
		t.Fatalf("ParseTier with whitespace returned unexpected error: %v", err)
	}
	if tier != TierStandard {
		t.Errorf("ParseTier with whitespace = %q, want %q", tier, TierStandard)
	}
}

func TestParseTierInvalid(t *testing.T) {
	invalidInputs := []string{
		"",
		"invalid",
		"tier4",
		"4",
		"foo",
		"standardx",
	}

	for _, input := range invalidInputs {
		t.Run(input, func(t *testing.T) {
			_, err := ParseTier(input)
			if err == nil {
				t.Errorf("ParseTier(%q) expected error, got nil", input)
			}
		})
	}
}

func TestGetTierInfoStandard(t *testing.T) {
	info := GetTierInfo(TierStandard)
	if info.Tier != TierStandard {
		t.Errorf("Tier = %q, want %q", info.Tier, TierStandard)
	}
	if info.Name != "Standard Cloudflare Tunnel" {
		t.Errorf("Name = %q, want %q", info.Name, "Standard Cloudflare Tunnel")
	}
	if info.EdgeVerification != "inherited" {
		t.Errorf("EdgeVerification = %q, want %q", info.EdgeVerification, "inherited")
	}
	if !info.FedRAMPDependency {
		t.Error("FedRAMPDependency = false, want true for standard tier")
	}
}

func TestGetTierInfoRegionalKeyless(t *testing.T) {
	info := GetTierInfo(TierRegionalKeyless)
	if info.Tier != TierRegionalKeyless {
		t.Errorf("Tier = %q, want %q", info.Tier, TierRegionalKeyless)
	}
	if info.Name != "Cloudflare FIPS 140 Level 3 (Keyless SSL + HSM)" {
		t.Errorf("Name = %q, want %q", info.Name, "Cloudflare FIPS 140 Level 3 (Keyless SSL + HSM)")
	}
	if info.EdgeVerification != "api + inherited" {
		t.Errorf("EdgeVerification = %q, want %q", info.EdgeVerification, "api + inherited")
	}
	if !info.FedRAMPDependency {
		t.Error("FedRAMPDependency = false, want true for regional_keyless tier")
	}
}

func TestGetTierInfoSelfHosted(t *testing.T) {
	info := GetTierInfo(TierSelfHosted)
	if info.Tier != TierSelfHosted {
		t.Errorf("Tier = %q, want %q", info.Tier, TierSelfHosted)
	}
	if info.Name != "Self-Hosted FIPS Edge Proxy" {
		t.Errorf("Name = %q, want %q", info.Name, "Self-Hosted FIPS Edge Proxy")
	}
	if info.EdgeVerification != "direct" {
		t.Errorf("EdgeVerification = %q, want %q", info.EdgeVerification, "direct")
	}
	if info.FedRAMPDependency {
		t.Error("FedRAMPDependency = true, want false for self_hosted tier")
	}
}

func TestGetTierInfoUnknown(t *testing.T) {
	info := GetTierInfo(Tier("nonexistent"))
	if info.Name != "Unknown" {
		t.Errorf("Name = %q, want %q for unknown tier", info.Name, "Unknown")
	}
	if info.Description != "Unknown deployment tier" {
		t.Errorf("Description = %q, want %q", info.Description, "Unknown deployment tier")
	}
}

func TestGetTierDefaultsToStandardOnInvalid(t *testing.T) {
	info := GetTier("completely_invalid_tier")
	if info.Tier != TierStandard {
		t.Errorf("GetTier with invalid input: Tier = %q, want %q", info.Tier, TierStandard)
	}
	if info.Name != "Standard Cloudflare Tunnel" {
		t.Errorf("GetTier with invalid input: Name = %q, want %q", info.Name, "Standard Cloudflare Tunnel")
	}
}

func TestGetTierValidInput(t *testing.T) {
	info := GetTier("self_hosted")
	if info.Tier != TierSelfHosted {
		t.Errorf("GetTier(\"self_hosted\"): Tier = %q, want %q", info.Tier, TierSelfHosted)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Tier != TierStandard {
		t.Errorf("DefaultConfig().Tier = %q, want %q", cfg.Tier, TierStandard)
	}
	if cfg.RegionalServicesEnabled {
		t.Error("DefaultConfig().RegionalServicesEnabled = true, want false")
	}
	if cfg.KeylessSSLHost != "" {
		t.Errorf("DefaultConfig().KeylessSSLHost = %q, want empty", cfg.KeylessSSLHost)
	}
	if cfg.KeylessSSLPort != 0 {
		t.Errorf("DefaultConfig().KeylessSSLPort = %d, want 0", cfg.KeylessSSLPort)
	}
	if cfg.ProxyListenAddr != "" {
		t.Errorf("DefaultConfig().ProxyListenAddr = %q, want empty", cfg.ProxyListenAddr)
	}
	if cfg.ProxyCertFile != "" {
		t.Errorf("DefaultConfig().ProxyCertFile = %q, want empty", cfg.ProxyCertFile)
	}
	if cfg.ProxyKeyFile != "" {
		t.Errorf("DefaultConfig().ProxyKeyFile = %q, want empty", cfg.ProxyKeyFile)
	}
	if cfg.ProxyUpstream != "" {
		t.Errorf("DefaultConfig().ProxyUpstream = %q, want empty", cfg.ProxyUpstream)
	}
}

func TestTierConstants(t *testing.T) {
	if TierStandard != "standard" {
		t.Errorf("TierStandard = %q, want %q", TierStandard, "standard")
	}
	if TierRegionalKeyless != "regional_keyless" {
		t.Errorf("TierRegionalKeyless = %q, want %q", TierRegionalKeyless, "regional_keyless")
	}
	if TierSelfHosted != "self_hosted" {
		t.Errorf("TierSelfHosted = %q, want %q", TierSelfHosted, "self_hosted")
	}
}
