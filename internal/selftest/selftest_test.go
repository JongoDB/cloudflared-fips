package selftest

import (
	"crypto/tls"
	"testing"
)

func TestRunAllChecks(t *testing.T) {
	results, _ := RunAllChecks()

	if len(results) == 0 {
		t.Fatal("expected at least one check result")
	}

	for _, r := range results {
		if r.Name == "" {
			t.Error("check result has empty name")
		}
		if r.Timestamp == "" {
			t.Error("check result has empty timestamp")
		}
		if r.Status != StatusPass && r.Status != StatusFail && r.Status != StatusWarn && r.Status != StatusSkip {
			t.Errorf("check %s has invalid status: %s", r.Name, r.Status)
		}
	}
}

func TestGenerateReport(t *testing.T) {
	report, _ := GenerateReport("test-version")

	if report.Version != "test-version" {
		t.Errorf("expected version test-version, got %s", report.Version)
	}
	if report.Platform == "" {
		t.Error("expected non-empty platform")
	}
	if report.Timestamp == "" {
		t.Error("expected non-empty timestamp")
	}
	if report.Summary.Total == 0 {
		t.Error("expected at least one total check")
	}
	if report.Summary.Total != report.Summary.Passed+report.Summary.Failed+report.Summary.Warnings+report.Summary.Skipped {
		t.Error("summary counts do not add up to total")
	}
}

func TestIsFIPSApproved(t *testing.T) {
	tests := []struct {
		name     string
		id       uint16
		approved bool
	}{
		{
			name:     "ECDHE-RSA-AES128-GCM",
			id:       tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			approved: true,
		},
		{
			name:     "ECDHE-ECDSA-AES256-GCM",
			id:       tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			approved: true,
		},
		{
			name:     "TLS13-AES128-GCM",
			id:       tls.TLS_AES_128_GCM_SHA256,
			approved: true,
		},
		{
			name:     "RSA-AES128-GCM",
			id:       tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
			approved: true,
		},
		{
			name:     "unknown suite",
			id:       0xFFFF,
			approved: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsFIPSApproved(tt.id)
			if got != tt.approved {
				t.Errorf("IsFIPSApproved(%#x) = %v, want %v", tt.id, got, tt.approved)
			}
		})
	}
}

func TestGetFIPSTLSConfig(t *testing.T) {
	cfg := GetFIPSTLSConfig()

	if cfg.MinVersion != tls.VersionTLS12 {
		t.Errorf("expected MinVersion TLS 1.2, got %d", cfg.MinVersion)
	}
	if cfg.MaxVersion != tls.VersionTLS13 {
		t.Errorf("expected MaxVersion TLS 1.3, got %d", cfg.MaxVersion)
	}
	if len(cfg.CipherSuites) == 0 {
		t.Error("expected at least one cipher suite")
	}

	for _, id := range cfg.CipherSuites {
		if !IsFIPSApproved(id) {
			t.Errorf("TLS config contains non-FIPS cipher suite: %#x", id)
		}
	}
}

func TestSHA256KAT(t *testing.T) {
	vec := KATVector{
		Algorithm: "SHA-256",
		Input:     "616263",
		Expected:  "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
	}

	if err := verifySHA256(vec); err != nil {
		t.Errorf("SHA-256 KAT failed: %v", err)
	}
}

func TestSHA384KAT(t *testing.T) {
	vec := KATVector{
		Algorithm: "SHA-384",
		Input:     "616263",
		Expected:  "cb00753f45a35e8bb5a03d699ac65007272c32ab0eded1631a8b605a43ff5bed8086072ba1e7cc2358baeca134c825a7",
	}

	if err := verifySHA384(vec); err != nil {
		t.Errorf("SHA-384 KAT failed: %v", err)
	}
}

func TestHMACSHA256KAT(t *testing.T) {
	vec := KATVector{
		Algorithm: "HMAC-SHA-256",
		Key:       "0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b",
		Input:     "4869205468657265",
		Expected:  "b0344c61d8db38535ca8afceaf0bf12b881dc200c9833da726e9376c2e32cff7",
	}

	if err := verifyHMACSHA256(vec); err != nil {
		t.Errorf("HMAC-SHA-256 KAT failed: %v", err)
	}
}

func TestBannedCipherPatterns(t *testing.T) {
	if len(BannedCipherPatterns) == 0 {
		t.Error("expected at least one banned cipher pattern")
	}

	expected := map[string]bool{
		"RC4":    true,
		"DES":    true,
		"3DES":   true,
		"NULL":   true,
		"EXPORT": true,
		"anon":   true,
	}

	for _, p := range BannedCipherPatterns {
		if !expected[p] {
			t.Errorf("unexpected banned pattern: %s", p)
		}
	}
}
