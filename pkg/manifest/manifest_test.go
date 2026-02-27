package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func sampleManifest() *BuildManifest {
	return &BuildManifest{
		Version:                    "1.0.0",
		Commit:                     "abc123def456",
		BuildTime:                  "2026-02-20T00:00:00Z",
		CloudflaredUpstreamVersion: "2025.2.1",
		CloudflaredUpstreamCommit:  "def456abc789",
		CryptoEngine:               "boringcrypto",
		BoringSSlVersion:           "fips-20230428",
		FIPSCertificates: []FIPSCertificate{
			{
				Module:      "BoringSSL",
				Certificate: "#4735",
				Algorithms:  []string{"AES-GCM-128", "AES-GCM-256", "SHA-256", "SHA-384", "ECDSA-P256", "RSA-2048"},
			},
			{
				Module:      "RHEL OpenSSL",
				Certificate: "#4349",
				Algorithms:  []string{"AES-256-CBC", "SHA-512"},
			},
		},
		TargetPlatform: "linux/amd64",
		PackageFormat:  "rpm",
		SBOMsha256:     "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		BinarySHA256:   "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
	}
}

func TestWriteAndReadManifestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "build-manifest.json")

	original := sampleManifest()

	if err := WriteManifest(path, original); err != nil {
		t.Fatalf("WriteManifest failed: %v", err)
	}

	got, err := ReadManifest(path)
	if err != nil {
		t.Fatalf("ReadManifest failed: %v", err)
	}

	if got.Version != original.Version {
		t.Errorf("Version = %q, want %q", got.Version, original.Version)
	}
	if got.Commit != original.Commit {
		t.Errorf("Commit = %q, want %q", got.Commit, original.Commit)
	}
	if got.BuildTime != original.BuildTime {
		t.Errorf("BuildTime = %q, want %q", got.BuildTime, original.BuildTime)
	}
	if got.CloudflaredUpstreamVersion != original.CloudflaredUpstreamVersion {
		t.Errorf("CloudflaredUpstreamVersion = %q, want %q", got.CloudflaredUpstreamVersion, original.CloudflaredUpstreamVersion)
	}
	if got.CloudflaredUpstreamCommit != original.CloudflaredUpstreamCommit {
		t.Errorf("CloudflaredUpstreamCommit = %q, want %q", got.CloudflaredUpstreamCommit, original.CloudflaredUpstreamCommit)
	}
	if got.CryptoEngine != original.CryptoEngine {
		t.Errorf("CryptoEngine = %q, want %q", got.CryptoEngine, original.CryptoEngine)
	}
	if got.BoringSSlVersion != original.BoringSSlVersion {
		t.Errorf("BoringSSlVersion = %q, want %q", got.BoringSSlVersion, original.BoringSSlVersion)
	}
	if got.TargetPlatform != original.TargetPlatform {
		t.Errorf("TargetPlatform = %q, want %q", got.TargetPlatform, original.TargetPlatform)
	}
	if got.PackageFormat != original.PackageFormat {
		t.Errorf("PackageFormat = %q, want %q", got.PackageFormat, original.PackageFormat)
	}
	if got.SBOMsha256 != original.SBOMsha256 {
		t.Errorf("SBOMsha256 = %q, want %q", got.SBOMsha256, original.SBOMsha256)
	}
	if got.BinarySHA256 != original.BinarySHA256 {
		t.Errorf("BinarySHA256 = %q, want %q", got.BinarySHA256, original.BinarySHA256)
	}
}

func TestReadManifestFromKnownJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "known.json")

	content := `{
  "version": "2.0.0",
  "commit": "deadbeef",
  "build_time": "2026-01-15T12:00:00Z",
  "cloudflared_upstream_version": "2025.1.0",
  "cloudflared_upstream_commit": "cafebabe",
  "crypto_engine": "gonative",
  "boringssl_version": "",
  "fips_certificates": [
    {
      "module": "Go Native",
      "certificate": "A6650",
      "algorithms": ["AES-GCM-256", "SHA-256"]
    }
  ],
  "target_platform": "darwin/arm64",
  "package_format": "pkg",
  "sbom_sha256": "abcdef0123456789",
  "binary_sha256": "9876543210fedcba"
}`

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test JSON: %v", err)
	}

	m, err := ReadManifest(path)
	if err != nil {
		t.Fatalf("ReadManifest failed: %v", err)
	}

	if m.Version != "2.0.0" {
		t.Errorf("Version = %q, want %q", m.Version, "2.0.0")
	}
	if m.Commit != "deadbeef" {
		t.Errorf("Commit = %q, want %q", m.Commit, "deadbeef")
	}
	if m.CryptoEngine != "gonative" {
		t.Errorf("CryptoEngine = %q, want %q", m.CryptoEngine, "gonative")
	}
	if m.TargetPlatform != "darwin/arm64" {
		t.Errorf("TargetPlatform = %q, want %q", m.TargetPlatform, "darwin/arm64")
	}
	if m.PackageFormat != "pkg" {
		t.Errorf("PackageFormat = %q, want %q", m.PackageFormat, "pkg")
	}
	if len(m.FIPSCertificates) != 1 {
		t.Fatalf("FIPSCertificates length = %d, want 1", len(m.FIPSCertificates))
	}
	if m.FIPSCertificates[0].Module != "Go Native" {
		t.Errorf("FIPSCertificates[0].Module = %q, want %q", m.FIPSCertificates[0].Module, "Go Native")
	}
	if m.FIPSCertificates[0].Certificate != "A6650" {
		t.Errorf("FIPSCertificates[0].Certificate = %q, want %q", m.FIPSCertificates[0].Certificate, "A6650")
	}
}

func TestReadManifestNonExistentPath(t *testing.T) {
	_, err := ReadManifest("/nonexistent/path/to/manifest.json")
	if err == nil {
		t.Fatal("ReadManifest on non-existent path should return an error")
	}
}

func TestReadManifestInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")

	if err := os.WriteFile(path, []byte("{not valid json!!!}"), 0644); err != nil {
		t.Fatalf("failed to write bad JSON: %v", err)
	}

	_, err := ReadManifest(path)
	if err == nil {
		t.Fatal("ReadManifest on invalid JSON should return an error")
	}
}

func TestWriteManifestCreatesCorrectJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "output.json")

	m := &BuildManifest{
		Version:        "3.0.0",
		Commit:         "1234abcd",
		BuildTime:      "2026-03-01T08:30:00Z",
		CryptoEngine:   "boringcrypto",
		TargetPlatform: "linux/arm64",
		PackageFormat:  "deb",
	}

	if err := WriteManifest(path, m); err != nil {
		t.Fatalf("WriteManifest failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("written file is not valid JSON: %v", err)
	}

	if v, ok := parsed["version"].(string); !ok || v != "3.0.0" {
		t.Errorf("JSON version = %v, want %q", parsed["version"], "3.0.0")
	}
	if v, ok := parsed["commit"].(string); !ok || v != "1234abcd" {
		t.Errorf("JSON commit = %v, want %q", parsed["commit"], "1234abcd")
	}
	if v, ok := parsed["crypto_engine"].(string); !ok || v != "boringcrypto" {
		t.Errorf("JSON crypto_engine = %v, want %q", parsed["crypto_engine"], "boringcrypto")
	}
	if v, ok := parsed["target_platform"].(string); !ok || v != "linux/arm64" {
		t.Errorf("JSON target_platform = %v, want %q", parsed["target_platform"], "linux/arm64")
	}
	if v, ok := parsed["package_format"].(string); !ok || v != "deb" {
		t.Errorf("JSON package_format = %v, want %q", parsed["package_format"], "deb")
	}
}

func TestFIPSCertificateSliceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "certs.json")

	original := &BuildManifest{
		Version: "1.0.0",
		FIPSCertificates: []FIPSCertificate{
			{
				Module:      "BoringSSL",
				Certificate: "#4735",
				Algorithms:  []string{"AES-GCM-128", "AES-GCM-256", "SHA-256"},
			},
			{
				Module:      "RHEL OpenSSL",
				Certificate: "#4857",
				Algorithms:  []string{"AES-256-CBC", "SHA-512", "RSA-4096"},
			},
			{
				Module:      "Go Native",
				Certificate: "pending",
				Algorithms:  []string{"AES-GCM-256"},
			},
		},
	}

	if err := WriteManifest(path, original); err != nil {
		t.Fatalf("WriteManifest failed: %v", err)
	}

	got, err := ReadManifest(path)
	if err != nil {
		t.Fatalf("ReadManifest failed: %v", err)
	}

	if len(got.FIPSCertificates) != len(original.FIPSCertificates) {
		t.Fatalf("FIPSCertificates length = %d, want %d", len(got.FIPSCertificates), len(original.FIPSCertificates))
	}

	for i, cert := range got.FIPSCertificates {
		orig := original.FIPSCertificates[i]
		if cert.Module != orig.Module {
			t.Errorf("FIPSCertificates[%d].Module = %q, want %q", i, cert.Module, orig.Module)
		}
		if cert.Certificate != orig.Certificate {
			t.Errorf("FIPSCertificates[%d].Certificate = %q, want %q", i, cert.Certificate, orig.Certificate)
		}
		if len(cert.Algorithms) != len(orig.Algorithms) {
			t.Errorf("FIPSCertificates[%d].Algorithms length = %d, want %d", i, len(cert.Algorithms), len(orig.Algorithms))
			continue
		}
		for j, alg := range cert.Algorithms {
			if alg != orig.Algorithms[j] {
				t.Errorf("FIPSCertificates[%d].Algorithms[%d] = %q, want %q", i, j, alg, orig.Algorithms[j])
			}
		}
	}
}

func TestWriteManifestEmptyFIPSCertificates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty-certs.json")

	m := &BuildManifest{
		Version:          "1.0.0",
		FIPSCertificates: []FIPSCertificate{},
	}

	if err := WriteManifest(path, m); err != nil {
		t.Fatalf("WriteManifest failed: %v", err)
	}

	got, err := ReadManifest(path)
	if err != nil {
		t.Fatalf("ReadManifest failed: %v", err)
	}

	if len(got.FIPSCertificates) != 0 {
		t.Errorf("FIPSCertificates length = %d, want 0", len(got.FIPSCertificates))
	}
}
