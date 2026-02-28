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

// ---------------------------------------------------------------------------
// verifyAESGCM — KAT for AES-128-GCM and AES-256-GCM
// ---------------------------------------------------------------------------

func TestVerifyAESGCM_128(t *testing.T) {
	vec := KATVector{
		Algorithm: "AES-128-GCM",
		Key:       "cf063a34d4a9a76c2c86787d3f96db71",
		Input:     "10aa0a348aeb884c3e1588e6c71bab0a",
		IV:        "113b9785971864c83b01c787",
		AAD:       "",
		Expected:  "d0313c831f850fda25b5454998058e59cf0ab9169136a778734c33c8718541e6",
	}
	if err := verifyAESGCM(vec); err != nil {
		t.Errorf("AES-128-GCM KAT failed: %v", err)
	}
}

func TestVerifyAESGCM_256(t *testing.T) {
	vec := KATVector{
		Algorithm: "AES-256-GCM",
		Key:       "e5a03e42e4552e0560ac34c91aab0897a04b7a05f0b9b80447e1d4e30e1e6509",
		Input:     "000000000000000000000000",
		IV:        "000000000000000000000000",
		AAD:       "",
		Expected:  "89a607e42e930df963b6e3269289dc904021d1cf4445abcc406e8b22",
	}
	if err := verifyAESGCM(vec); err != nil {
		t.Errorf("AES-256-GCM KAT failed: %v", err)
	}
}

func TestVerifyAESGCM_BadExpected(t *testing.T) {
	vec := KATVector{
		Algorithm: "AES-128-GCM",
		Key:       "cf063a34d4a9a76c2c86787d3f96db71",
		Input:     "10aa0a348aeb884c3e1588e6c71bab0a",
		IV:        "113b9785971864c83b01c787",
		AAD:       "",
		Expected:  "0000000000000000000000000000000000000000000000000000000000000000",
	}
	if err := verifyAESGCM(vec); err == nil {
		t.Error("expected AES-GCM KAT to fail with wrong expected value")
	}
}

// ---------------------------------------------------------------------------
// runSingleKAT — dispatch and unknown algorithm
// ---------------------------------------------------------------------------

func TestRunSingleKAT_SHA256(t *testing.T) {
	vec := KATVector{
		Algorithm: "SHA-256",
		Input:     "616263",
		Expected:  "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
	}
	result := runSingleKAT(vec)
	if result.Status != StatusPass {
		t.Errorf("status = %q, want pass. Details: %s", result.Status, result.Details)
	}
	if result.Name != "kat_SHA-256" {
		t.Errorf("name = %q, want kat_SHA-256", result.Name)
	}
}

func TestRunSingleKAT_UnknownAlgorithm(t *testing.T) {
	vec := KATVector{
		Algorithm: "UNKNOWN-ALGO",
		Input:     "deadbeef",
		Expected:  "cafe",
	}
	result := runSingleKAT(vec)
	if result.Status != StatusSkip {
		t.Errorf("status = %q, want skip for unknown algorithm", result.Status)
	}
	if result.Name != "kat_UNKNOWN-ALGO" {
		t.Errorf("name = %q, want kat_UNKNOWN-ALGO", result.Name)
	}
}

func TestRunSingleKAT_FailedVector(t *testing.T) {
	vec := KATVector{
		Algorithm: "SHA-256",
		Input:     "616263",
		Expected:  "0000000000000000000000000000000000000000000000000000000000000000",
	}
	result := runSingleKAT(vec)
	if result.Status != StatusFail {
		t.Errorf("status = %q, want fail for wrong expected", result.Status)
	}
	if result.Remediation == "" {
		t.Error("failed KAT should include remediation")
	}
}

// ---------------------------------------------------------------------------
// runECDSATest — ECDSA P-256 sign/verify
// ---------------------------------------------------------------------------

func TestRunECDSATest(t *testing.T) {
	result := runECDSATest()
	if result.Name != "kat_ECDSA-P256" {
		t.Errorf("name = %q, want kat_ECDSA-P256", result.Name)
	}
	if result.Status != StatusPass {
		t.Errorf("ECDSA test status = %q, want pass. Message: %s", result.Status, result.Message)
	}
	if result.Severity != SeverityCritical {
		t.Errorf("severity = %q, want critical", result.Severity)
	}
}

// ---------------------------------------------------------------------------
// runRSATest — RSA-2048 sign/verify
// ---------------------------------------------------------------------------

func TestRunRSATest(t *testing.T) {
	result := runRSATest()
	if result.Name != "kat_RSA-2048" {
		t.Errorf("name = %q, want kat_RSA-2048", result.Name)
	}
	if result.Status != StatusPass {
		t.Errorf("RSA test status = %q, want pass. Message: %s", result.Status, result.Message)
	}
	if result.Severity != SeverityCritical {
		t.Errorf("severity = %q, want critical", result.Severity)
	}
}

// ---------------------------------------------------------------------------
// runKnownAnswerTests — aggregator
// ---------------------------------------------------------------------------

func TestRunKnownAnswerTests(t *testing.T) {
	results := runKnownAnswerTests()

	// 5 KAT vectors + ECDSA + RSA = 7 total
	if len(results) != 7 {
		t.Errorf("expected 7 KAT results, got %d", len(results))
	}

	expectedNames := map[string]bool{
		"kat_AES-128-GCM":  false,
		"kat_AES-256-GCM":  false,
		"kat_SHA-256":      false,
		"kat_SHA-384":      false,
		"kat_HMAC-SHA-256": false,
		"kat_ECDSA-P256":   false,
		"kat_RSA-2048":     false,
	}
	for _, r := range results {
		if _, exists := expectedNames[r.Name]; exists {
			expectedNames[r.Name] = true
		}
		if r.Status != StatusPass {
			t.Errorf("KAT %s status = %q, want pass", r.Name, r.Status)
		}
	}
	for name, found := range expectedNames {
		if !found {
			t.Errorf("missing KAT result: %s", name)
		}
	}
}

// ---------------------------------------------------------------------------
// checkCipherSuites
// ---------------------------------------------------------------------------

func TestCheckCipherSuites_Structure(t *testing.T) {
	result := checkCipherSuites()
	if result.Name != "cipher_suites" {
		t.Errorf("Name = %q, want cipher_suites", result.Name)
	}
	if result.Severity != SeverityCritical {
		t.Errorf("Severity = %q, want critical", result.Severity)
	}
	// Should be pass (BoringCrypto or Go native handle non-FIPS suites)
	validStatuses := map[Status]bool{StatusPass: true, StatusFail: true}
	if !validStatuses[result.Status] {
		t.Errorf("status = %q, expected pass or fail", result.Status)
	}
}

// ---------------------------------------------------------------------------
// checkBoringCryptoLinked
// ---------------------------------------------------------------------------

func TestCheckBoringCryptoLinked_Structure(t *testing.T) {
	result := checkBoringCryptoLinked()
	if result.Name != "boring_crypto_linked" {
		t.Errorf("Name = %q, want boring_crypto_linked", result.Name)
	}
	if result.Severity != SeverityCritical {
		t.Errorf("Severity = %q, want critical", result.Severity)
	}
	// In non-BoringCrypto dev builds, this will fail; both are valid
	validStatuses := map[Status]bool{StatusPass: true, StatusFail: true}
	if !validStatuses[result.Status] {
		t.Errorf("status = %q, expected pass or fail", result.Status)
	}
}

// ---------------------------------------------------------------------------
// isBoringCryptoEnabled
// ---------------------------------------------------------------------------

func TestIsBoringCryptoEnabled(t *testing.T) {
	// Just ensure it doesn't panic and returns a bool
	_ = isBoringCryptoEnabled()
}

// ---------------------------------------------------------------------------
// IsFIPSApproved — additional edge cases
// ---------------------------------------------------------------------------

func TestIsFIPSApproved_TLS13Suites(t *testing.T) {
	// TLS 1.3 AES-256-GCM should be approved
	if !IsFIPSApproved(tls.TLS_AES_256_GCM_SHA384) {
		t.Error("TLS_AES_256_GCM_SHA384 should be FIPS approved")
	}

	// TLS 1.3 ChaCha20-Poly1305 should NOT be approved
	if IsFIPSApproved(tls.TLS_CHACHA20_POLY1305_SHA256) {
		t.Error("TLS_CHACHA20_POLY1305_SHA256 should NOT be FIPS approved")
	}
}

func TestIsFIPSApproved_CBCSuites(t *testing.T) {
	// CBC suites are FIPS-approved (GCM preferred but CBC valid)
	if !IsFIPSApproved(tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256) {
		t.Error("TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256 should be FIPS approved")
	}
	if !IsFIPSApproved(tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256) {
		t.Error("TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256 should be FIPS approved")
	}
}

func TestIsFIPSApproved_RSAKeyExchange(t *testing.T) {
	if !IsFIPSApproved(tls.TLS_RSA_WITH_AES_256_GCM_SHA384) {
		t.Error("TLS_RSA_WITH_AES_256_GCM_SHA384 should be FIPS approved")
	}
}

func TestIsFIPSApproved_ZeroValue(t *testing.T) {
	// Suite ID 0 should not be approved
	if IsFIPSApproved(0) {
		t.Error("suite ID 0 should not be FIPS approved")
	}
}

// ---------------------------------------------------------------------------
// GetFIPSTLSConfig — additional checks
// ---------------------------------------------------------------------------

func TestGetFIPSTLSConfig_CurvePreferences(t *testing.T) {
	cfg := GetFIPSTLSConfig()
	if len(cfg.CurvePreferences) != 2 {
		t.Fatalf("expected 2 curve preferences, got %d", len(cfg.CurvePreferences))
	}
	curves := map[tls.CurveID]bool{
		tls.CurveP256: false,
		tls.CurveP384: false,
	}
	for _, c := range cfg.CurvePreferences {
		curves[c] = true
	}
	if !curves[tls.CurveP256] {
		t.Error("expected P-256 in curve preferences")
	}
	if !curves[tls.CurveP384] {
		t.Error("expected P-384 in curve preferences")
	}
}

func TestGetFIPSTLSConfig_SuiteCount(t *testing.T) {
	cfg := GetFIPSTLSConfig()
	if len(cfg.CipherSuites) != len(FIPSApprovedCipherSuites) {
		t.Errorf("CipherSuites count = %d, want %d (all TLS 1.2 FIPS suites)",
			len(cfg.CipherSuites), len(FIPSApprovedCipherSuites))
	}
}
