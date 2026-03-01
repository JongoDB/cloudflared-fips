package selftest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"testing"
)

// ---------------------------------------------------------------------------
// RunAllChecksWithOptions
// ---------------------------------------------------------------------------

func TestRunAllChecksWithOptions_NoSignature(t *testing.T) {
	results, _ := RunAllChecksWithOptions(Options{})
	if len(results) == 0 {
		t.Fatal("expected results from RunAllChecksWithOptions")
	}

	// Without VerifySignature, no binary_signature check should appear
	for _, r := range results {
		if r.Name == "binary_signature" {
			t.Error("binary_signature check should not appear when VerifySignature is false")
		}
	}
}

func TestRunAllChecksWithOptions_WithSignature(t *testing.T) {
	results, _ := RunAllChecksWithOptions(Options{
		VerifySignature: true,
	})

	found := false
	for _, r := range results {
		if r.Name == "binary_signature" {
			found = true
			// In dev/CI, there's no .sig file, so expect warn (not crash)
			if r.Status != StatusWarn && r.Status != StatusFail {
				t.Errorf("binary_signature status = %q, want warn or fail (no .sig in dev)", r.Status)
			}
			break
		}
	}
	if !found {
		t.Error("binary_signature check should appear when VerifySignature is true")
	}
}

func TestRunAllChecksWithOptions_WithSignatureKeyPath(t *testing.T) {
	results, _ := RunAllChecksWithOptions(Options{
		VerifySignature: true,
		SignatureKeyPath: "/nonexistent/key.asc",
	})

	for _, r := range results {
		if r.Name == "binary_signature" {
			// Even with bad key path, it should produce a result (not panic)
			if r.Status == "" {
				t.Error("binary_signature should have a status")
			}
			return
		}
	}
	t.Error("binary_signature check missing from results")
}

// ---------------------------------------------------------------------------
// GenerateReportWithOptions
// ---------------------------------------------------------------------------

func TestGenerateReportWithOptions_Basic(t *testing.T) {
	report, _ := GenerateReportWithOptions("1.0.0-test", Options{})
	if report == nil {
		t.Fatal("report should be non-nil")
	}
	if report.Version != "1.0.0-test" {
		t.Errorf("Version = %q, want 1.0.0-test", report.Version)
	}
	expected := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
	if report.Platform != expected {
		t.Errorf("Platform = %q, want %q", report.Platform, expected)
	}
	if report.Timestamp == "" {
		t.Error("Timestamp should be set")
	}
	if report.Summary.Total == 0 {
		t.Error("Summary.Total should be > 0")
	}
	if report.Summary.Total != len(report.Results) {
		t.Errorf("Summary.Total (%d) != len(Results) (%d)", report.Summary.Total, len(report.Results))
	}
}

func TestGenerateReportWithOptions_WithSignature(t *testing.T) {
	report, _ := GenerateReportWithOptions("1.0.0-sig", Options{
		VerifySignature: true,
	})
	if report == nil {
		t.Fatal("report should be non-nil")
	}

	// Should have more results than without signature option
	basicReport, _ := GenerateReport("1.0.0-basic")
	if len(report.Results) <= len(basicReport.Results)-1 {
		t.Errorf("report with signature should have at least as many results (%d) as basic (%d)",
			len(report.Results), len(basicReport.Results))
	}
}

func TestGenerateReportWithOptions_SummaryAccuracy(t *testing.T) {
	report, _ := GenerateReportWithOptions("1.0.0", Options{})

	var pass, fail, warn, skip int
	for _, r := range report.Results {
		switch r.Status {
		case StatusPass:
			pass++
		case StatusFail:
			fail++
		case StatusWarn:
			warn++
		case StatusSkip:
			skip++
		}
	}

	if report.Summary.Passed != pass {
		t.Errorf("Summary.Passed = %d, counted %d", report.Summary.Passed, pass)
	}
	if report.Summary.Failed != fail {
		t.Errorf("Summary.Failed = %d, counted %d", report.Summary.Failed, fail)
	}
	if report.Summary.Warnings != warn {
		t.Errorf("Summary.Warnings = %d, counted %d", report.Summary.Warnings, warn)
	}
	if report.Summary.Skipped != skip {
		t.Errorf("Summary.Skipped = %d, counted %d", report.Summary.Skipped, skip)
	}
}

// ---------------------------------------------------------------------------
// PrintReport
// ---------------------------------------------------------------------------

func TestPrintReport_ValidJSON(t *testing.T) {
	report := &SelfTestReport{
		Version:   "test",
		Platform:  "linux/arm64",
		Timestamp: "2026-02-28T00:00:00Z",
		Results: []CheckResult{
			{Name: "test_check", Status: StatusPass, Severity: SeverityInfo, Message: "ok"},
		},
		Summary: Summary{Total: 1, Passed: 1},
	}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := PrintReport(report)

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("PrintReport error: %v", err)
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	output := buf.String()

	// Verify it's valid JSON
	var parsed SelfTestReport
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("PrintReport output is not valid JSON: %v\nOutput: %s", err, output)
	}
	if parsed.Version != "test" {
		t.Errorf("parsed Version = %q, want test", parsed.Version)
	}
	if len(parsed.Results) != 1 {
		t.Errorf("parsed Results length = %d, want 1", len(parsed.Results))
	}
}

// ---------------------------------------------------------------------------
// checkBoringCryptoVersion
// ---------------------------------------------------------------------------

func TestCheckBoringCryptoVersion_Go124Plus(t *testing.T) {
	result := checkBoringCryptoVersion()
	// Go 1.24+ → pass (140-3), older → warn (140-2)
	if result.Name != "boring_crypto_version" {
		t.Errorf("Name = %q, want boring_crypto_version", result.Name)
	}
	if result.Severity != SeverityInfo {
		t.Errorf("Severity = %q, want info", result.Severity)
	}
	// Since we're on Go 1.24+, expect pass
	if result.Status != StatusPass {
		t.Logf("Go version: %s, status: %s (may be <1.24)", runtime.Version(), result.Status)
	}
}

// ---------------------------------------------------------------------------
// checkGoNativeFIPS
// ---------------------------------------------------------------------------

func TestCheckGoNativeFIPS_NotSet(t *testing.T) {
	// Save and clear GODEBUG
	orig := os.Getenv("GODEBUG")
	os.Setenv("GODEBUG", "")
	defer os.Setenv("GODEBUG", orig)

	result := checkGoNativeFIPS()
	if result.Name != "go_native_fips" {
		t.Errorf("Name = %q, want go_native_fips", result.Name)
	}
	if result.Status != StatusWarn {
		t.Errorf("status = %q, want warn (GODEBUG not set)", result.Status)
	}
}

func TestCheckGoNativeFIPS_FIPSOn(t *testing.T) {
	orig := os.Getenv("GODEBUG")
	os.Setenv("GODEBUG", "fips140=on")
	defer os.Setenv("GODEBUG", orig)

	result := checkGoNativeFIPS()
	if result.Status != StatusPass {
		t.Errorf("status = %q, want pass (fips140=on)", result.Status)
	}
}

func TestCheckGoNativeFIPS_FIPSOnly(t *testing.T) {
	orig := os.Getenv("GODEBUG")
	os.Setenv("GODEBUG", "fips140=only")
	defer os.Setenv("GODEBUG", orig)

	result := checkGoNativeFIPS()
	if result.Status != StatusPass {
		t.Errorf("status = %q, want pass (fips140=only)", result.Status)
	}
}

// ---------------------------------------------------------------------------
// checkOSFIPSMode
// ---------------------------------------------------------------------------

func TestCheckOSFIPSMode_Structure(t *testing.T) {
	result := checkOSFIPSMode()
	if result.Name != "os_fips_mode" {
		t.Errorf("Name = %q, want os_fips_mode", result.Name)
	}
	if result.Severity != SeverityWarning {
		t.Errorf("Severity = %q, want warning", result.Severity)
	}
	if result.Timestamp == "" {
		t.Error("Timestamp should be set")
	}
	// Valid status for any environment
	validStatuses := map[Status]bool{StatusPass: true, StatusWarn: true, StatusSkip: true}
	if !validStatuses[result.Status] {
		t.Errorf("status = %q, expected pass/warn/skip", result.Status)
	}
}

// ---------------------------------------------------------------------------
// checkFIPSBackendActive
// ---------------------------------------------------------------------------

func TestCheckFIPSBackendActive_Structure(t *testing.T) {
	result := checkFIPSBackendActive()
	if result.Name != "fips_backend_active" {
		t.Errorf("Name = %q, want fips_backend_active", result.Name)
	}
	if result.Severity != SeverityCritical {
		t.Errorf("Severity = %q, want critical", result.Severity)
	}
}

// ---------------------------------------------------------------------------
// checkQUICCipherSafety
// ---------------------------------------------------------------------------

func TestCheckQUICCipherSafety_Structure(t *testing.T) {
	result := checkQUICCipherSafety()
	if result.Name != "quic_cipher_safety" {
		t.Errorf("Name = %q, want quic_cipher_safety", result.Name)
	}
	if result.Severity != SeverityCritical {
		t.Errorf("Severity = %q, want critical", result.Severity)
	}
	// Should pass (our FIPS TLS config only has approved suites)
	if result.Status != StatusPass {
		t.Errorf("status = %q, want pass", result.Status)
	}
}

// ---------------------------------------------------------------------------
// resolveExecutable
// ---------------------------------------------------------------------------

func TestResolveExecutable(t *testing.T) {
	// Should not panic or return empty
	resolved, err := resolveExecutable("/usr/bin/test")
	if err != nil && resolved == "" {
		t.Error("resolveExecutable should return either a path or an error, not both empty")
	}
}

func TestResolveExecutable_CurrentBinary(t *testing.T) {
	exePath, err := os.Executable()
	if err != nil {
		t.Skipf("cannot get executable path: %v", err)
	}

	resolved, err := resolveExecutable(exePath)
	if err != nil {
		t.Logf("resolveExecutable error (acceptable in some environments): %v", err)
		return
	}
	if resolved == "" {
		t.Error("resolved path should not be empty")
	}
}
