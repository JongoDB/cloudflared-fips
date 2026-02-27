package signing

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestHashFileKnownContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "testfile.txt")

	content := []byte("hello, FIPS world\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	expected := sha256.Sum256(content)
	expectedHex := hex.EncodeToString(expected[:])

	got, err := HashFile(path)
	if err != nil {
		t.Fatalf("HashFile failed: %v", err)
	}

	if got != expectedHex {
		t.Errorf("HashFile = %q, want %q", got, expectedHex)
	}
}

func TestHashFileEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.bin")

	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatalf("failed to write empty file: %v", err)
	}

	// SHA-256 of empty input is the well-known constant
	const emptySHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	got, err := HashFile(path)
	if err != nil {
		t.Fatalf("HashFile failed: %v", err)
	}

	if got != emptySHA256 {
		t.Errorf("HashFile(empty) = %q, want %q", got, emptySHA256)
	}
}

func TestHashFileMissingFile(t *testing.T) {
	_, err := HashFile("/nonexistent/path/to/file.bin")
	if err == nil {
		t.Fatal("HashFile on missing file should return an error")
	}
}

func TestWriteAndReadSignatureManifestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "signatures.json")

	original := &SignatureManifest{
		Version:   "1.0.0",
		BuildTime: "2026-02-20T00:00:00Z",
		Signatures: []SignatureInfo{
			{
				ArtifactPath:    "/opt/cloudflared-fips/bin/cloudflared",
				ArtifactSHA256:  "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
				SignaturePath:   "/opt/cloudflared-fips/bin/cloudflared.sig",
				SignatureMethod: "gpg",
				SignedAt:        "2026-02-20T01:00:00Z",
				SignerIdentity:  "release@cloudflared-fips.dev",
				Verified:        true,
			},
			{
				ArtifactPath:    "ghcr.io/cloudflared-fips/cloudflared-fips:latest",
				ArtifactSHA256:  "",
				SignatureMethod: "cosign",
				SignedAt:        "2026-02-20T01:05:00Z",
				Verified:        true,
			},
		},
		PublicKey: "https://example.com/public-key.asc",
	}

	if err := WriteSignatureManifest(path, original); err != nil {
		t.Fatalf("WriteSignatureManifest failed: %v", err)
	}

	got, err := ReadSignatureManifest(path)
	if err != nil {
		t.Fatalf("ReadSignatureManifest failed: %v", err)
	}

	if got.Version != original.Version {
		t.Errorf("Version = %q, want %q", got.Version, original.Version)
	}
	if got.BuildTime != original.BuildTime {
		t.Errorf("BuildTime = %q, want %q", got.BuildTime, original.BuildTime)
	}
	if got.PublicKey != original.PublicKey {
		t.Errorf("PublicKey = %q, want %q", got.PublicKey, original.PublicKey)
	}

	if len(got.Signatures) != len(original.Signatures) {
		t.Fatalf("Signatures length = %d, want %d", len(got.Signatures), len(original.Signatures))
	}

	for i, sig := range got.Signatures {
		orig := original.Signatures[i]
		if sig.ArtifactPath != orig.ArtifactPath {
			t.Errorf("Signatures[%d].ArtifactPath = %q, want %q", i, sig.ArtifactPath, orig.ArtifactPath)
		}
		if sig.ArtifactSHA256 != orig.ArtifactSHA256 {
			t.Errorf("Signatures[%d].ArtifactSHA256 = %q, want %q", i, sig.ArtifactSHA256, orig.ArtifactSHA256)
		}
		if sig.SignatureMethod != orig.SignatureMethod {
			t.Errorf("Signatures[%d].SignatureMethod = %q, want %q", i, sig.SignatureMethod, orig.SignatureMethod)
		}
		if sig.SignedAt != orig.SignedAt {
			t.Errorf("Signatures[%d].SignedAt = %q, want %q", i, sig.SignedAt, orig.SignedAt)
		}
		if sig.Verified != orig.Verified {
			t.Errorf("Signatures[%d].Verified = %v, want %v", i, sig.Verified, orig.Verified)
		}
	}
}

func TestReadSignatureManifestNonExistentPath(t *testing.T) {
	_, err := ReadSignatureManifest("/nonexistent/path/to/signatures.json")
	if err == nil {
		t.Fatal("ReadSignatureManifest on non-existent path should return an error")
	}
}

func TestDefaultPublicKeyPathIsSet(t *testing.T) {
	if DefaultPublicKeyPath == "" {
		t.Error("DefaultPublicKeyPath should not be empty")
	}
	if DefaultPublicKeyPath != "configs/public-key.asc" {
		t.Errorf("DefaultPublicKeyPath = %q, want %q", DefaultPublicKeyPath, "configs/public-key.asc")
	}
}

func TestDefaultPublicKeyURLIsSet(t *testing.T) {
	if DefaultPublicKeyURL == "" {
		t.Error("DefaultPublicKeyURL should not be empty")
	}
	if DefaultPublicKeyURL != "https://github.com/cloudflared-fips/cloudflared-fips/releases/latest/download/public-key.asc" {
		t.Errorf("DefaultPublicKeyURL = %q, want expected URL", DefaultPublicKeyURL)
	}
}

func TestSignatureInfoJSONMarshaling(t *testing.T) {
	info := SignatureInfo{
		ArtifactPath:    "/opt/bin/cloudflared",
		ArtifactSHA256:  "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		SignaturePath:   "/opt/bin/cloudflared.sig",
		SignatureMethod: "gpg",
		SignedAt:        "2026-02-25T10:00:00Z",
		SignerIdentity:  "test@example.com",
		Verified:        true,
		Error:           "",
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var parsed SignatureInfo
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if parsed.ArtifactPath != info.ArtifactPath {
		t.Errorf("ArtifactPath = %q, want %q", parsed.ArtifactPath, info.ArtifactPath)
	}
	if parsed.ArtifactSHA256 != info.ArtifactSHA256 {
		t.Errorf("ArtifactSHA256 = %q, want %q", parsed.ArtifactSHA256, info.ArtifactSHA256)
	}
	if parsed.SignaturePath != info.SignaturePath {
		t.Errorf("SignaturePath = %q, want %q", parsed.SignaturePath, info.SignaturePath)
	}
	if parsed.SignatureMethod != info.SignatureMethod {
		t.Errorf("SignatureMethod = %q, want %q", parsed.SignatureMethod, info.SignatureMethod)
	}
	if parsed.SignedAt != info.SignedAt {
		t.Errorf("SignedAt = %q, want %q", parsed.SignedAt, info.SignedAt)
	}
	if parsed.SignerIdentity != info.SignerIdentity {
		t.Errorf("SignerIdentity = %q, want %q", parsed.SignerIdentity, info.SignerIdentity)
	}
	if parsed.Verified != info.Verified {
		t.Errorf("Verified = %v, want %v", parsed.Verified, info.Verified)
	}
}

func TestSignatureInfoJSONOmitsEmptyOptionalFields(t *testing.T) {
	info := SignatureInfo{
		ArtifactPath:    "/opt/bin/cloudflared",
		ArtifactSHA256:  "abcdef",
		SignatureMethod: "none",
		Verified:        false,
		// SignaturePath, SignedAt, SignerIdentity, Error are empty
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal to map failed: %v", err)
	}

	// Fields with omitempty should not appear when empty
	for _, field := range []string{"signature_path", "signed_at", "signer_identity", "error"} {
		if _, exists := raw[field]; exists {
			t.Errorf("expected field %q to be omitted from JSON when empty, but it was present", field)
		}
	}

	// Required fields should always be present
	for _, field := range []string{"artifact_path", "artifact_sha256", "signature_method", "verified"} {
		if _, exists := raw[field]; !exists {
			t.Errorf("expected field %q to be present in JSON, but it was missing", field)
		}
	}
}

func TestSignatureManifestEmptySignatures(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty-sigs.json")

	original := &SignatureManifest{
		Version:    "1.0.0",
		BuildTime:  "2026-02-20T00:00:00Z",
		Signatures: []SignatureInfo{},
	}

	if err := WriteSignatureManifest(path, original); err != nil {
		t.Fatalf("WriteSignatureManifest failed: %v", err)
	}

	got, err := ReadSignatureManifest(path)
	if err != nil {
		t.Fatalf("ReadSignatureManifest failed: %v", err)
	}

	if len(got.Signatures) != 0 {
		t.Errorf("Signatures length = %d, want 0", len(got.Signatures))
	}
}
