package signing

import (
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// GPGSign
// ---------------------------------------------------------------------------

func TestGPGSign_NonexistentFile(t *testing.T) {
	info, err := GPGSign("/nonexistent/artifact.bin", "")
	if err == nil {
		t.Fatal("expected error for nonexistent artifact")
	}
	if info == nil {
		t.Fatal("info should be non-nil even on error")
	}
	if info.SignatureMethod != "gpg" {
		t.Errorf("SignatureMethod = %q, want gpg", info.SignatureMethod)
	}
	if info.Error == "" {
		t.Error("info.Error should be set on failure")
	}
}

func TestGPGSign_GpgNotInstalled(t *testing.T) {
	// Create a real file, but gpg may not be installed in CI
	dir := t.TempDir()
	artifact := filepath.Join(dir, "test.bin")
	if err := os.WriteFile(artifact, []byte("test content"), 0644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}

	info, err := GPGSign(artifact, "nonexistent-key-id")
	// If gpg is not installed, this should fail with a gpg-related error
	// If gpg IS installed but has no key, it should also fail
	// Either way, we should get a non-nil info with an error
	if info == nil {
		t.Fatal("info should be non-nil")
	}
	if info.ArtifactSHA256 == "" {
		t.Error("ArtifactSHA256 should be computed even if signing fails")
	}
	if info.SignatureMethod != "gpg" {
		t.Errorf("SignatureMethod = %q, want gpg", info.SignatureMethod)
	}

	// gpg is very unlikely to be configured in CI, so expect failure
	if err == nil {
		// gpg succeeded — that's fine in a dev environment, but verify fields
		if !info.Verified {
			t.Error("Verified should be true when signing succeeds")
		}
		if info.SignaturePath == "" {
			t.Error("SignaturePath should be set on success")
		}
		if info.SignedAt == "" {
			t.Error("SignedAt should be set on success")
		}
	} else {
		// Expected: gpg not available or no key
		if info.Verified {
			t.Error("Verified should be false when signing fails")
		}
		if info.Error == "" {
			t.Error("Error should describe the failure")
		}
	}
}

func TestGPGSign_WithKeyID(t *testing.T) {
	dir := t.TempDir()
	artifact := filepath.Join(dir, "test.bin")
	if err := os.WriteFile(artifact, []byte("test"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	info, err := GPGSign(artifact, "release@test.example.com")
	if err == nil {
		// GPG succeeded — SignerIdentity should be set
		if info.SignerIdentity != "release@test.example.com" {
			t.Errorf("SignerIdentity = %q, want release@test.example.com", info.SignerIdentity)
		}
	} else {
		// GPG not available — SignerIdentity is not set on failure path
		if info.SignerIdentity != "" {
			t.Errorf("SignerIdentity on failure = %q, want empty", info.SignerIdentity)
		}
	}
}

func TestGPGSign_WithoutKeyID(t *testing.T) {
	dir := t.TempDir()
	artifact := filepath.Join(dir, "test.bin")
	if err := os.WriteFile(artifact, []byte("test"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	info, _ := GPGSign(artifact, "")
	if info.SignerIdentity != "" {
		t.Errorf("SignerIdentity = %q, want empty (no key ID)", info.SignerIdentity)
	}
}

// ---------------------------------------------------------------------------
// GPGVerify
// ---------------------------------------------------------------------------

func TestGPGVerify_NonexistentArtifact(t *testing.T) {
	info, err := GPGVerify("/nonexistent/artifact.bin", "/nonexistent/artifact.bin.sig")
	if err == nil {
		t.Fatal("expected error for nonexistent artifact")
	}
	if info == nil {
		t.Fatal("info should be non-nil")
	}
	if info.SignatureMethod != "gpg" {
		t.Errorf("SignatureMethod = %q, want gpg", info.SignatureMethod)
	}
}

func TestGPGVerify_BadSignature(t *testing.T) {
	dir := t.TempDir()
	artifact := filepath.Join(dir, "test.bin")
	sigFile := filepath.Join(dir, "test.bin.sig")

	if err := os.WriteFile(artifact, []byte("test content"), 0644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	if err := os.WriteFile(sigFile, []byte("not a real signature"), 0644); err != nil {
		t.Fatalf("write sig: %v", err)
	}

	info, err := GPGVerify(artifact, sigFile)
	if err == nil {
		t.Fatal("expected error for bad signature")
	}
	if info == nil {
		t.Fatal("info should be non-nil")
	}
	if info.Verified {
		t.Error("Verified should be false for bad signature")
	}
	if info.ArtifactSHA256 == "" {
		t.Error("ArtifactSHA256 should still be computed")
	}
}

func TestGPGVerify_FieldsPopulated(t *testing.T) {
	dir := t.TempDir()
	artifact := filepath.Join(dir, "test.bin")
	sigFile := filepath.Join(dir, "test.bin.sig")

	if err := os.WriteFile(artifact, []byte("test content"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(sigFile, []byte("fake"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	info, _ := GPGVerify(artifact, sigFile)
	if info.ArtifactPath != artifact {
		t.Errorf("ArtifactPath = %q, want %q", info.ArtifactPath, artifact)
	}
	if info.SignaturePath != sigFile {
		t.Errorf("SignaturePath = %q, want %q", info.SignaturePath, sigFile)
	}
}

// ---------------------------------------------------------------------------
// CosignSign
// ---------------------------------------------------------------------------

func TestCosignSign_NotInstalled(t *testing.T) {
	// cosign is almost certainly not installed in CI
	info, err := CosignSign("ghcr.io/test/image:latest", "")
	if info == nil {
		t.Fatal("info should be non-nil")
	}
	if info.SignatureMethod != "cosign" {
		t.Errorf("SignatureMethod = %q, want cosign", info.SignatureMethod)
	}
	if info.ArtifactPath != "ghcr.io/test/image:latest" {
		t.Errorf("ArtifactPath = %q, want image ref", info.ArtifactPath)
	}

	if err == nil {
		// cosign unexpectedly available and working
		if !info.Verified {
			t.Error("Verified should be true on success")
		}
	} else {
		if info.Verified {
			t.Error("Verified should be false on failure")
		}
		if info.Error == "" {
			t.Error("Error should describe the failure")
		}
	}
}

func TestCosignSign_WithKeyPath(t *testing.T) {
	info, _ := CosignSign("ghcr.io/test/image:latest", "/path/to/cosign.key")
	if info.SignatureMethod != "cosign" {
		t.Errorf("SignatureMethod = %q, want cosign", info.SignatureMethod)
	}
}

func TestCosignSign_KeylessMode(t *testing.T) {
	info, _ := CosignSign("ghcr.io/test/image:latest", "")
	if info.SignatureMethod != "cosign" {
		t.Errorf("SignatureMethod = %q, want cosign", info.SignatureMethod)
	}
}

// ---------------------------------------------------------------------------
// CosignVerify
// ---------------------------------------------------------------------------

func TestCosignVerify_NotInstalled(t *testing.T) {
	info, err := CosignVerify("ghcr.io/test/image:latest", "")
	if info == nil {
		t.Fatal("info should be non-nil")
	}
	if info.SignatureMethod != "cosign" {
		t.Errorf("SignatureMethod = %q, want cosign", info.SignatureMethod)
	}

	if err == nil {
		if !info.Verified {
			t.Error("Verified should be true on success")
		}
	} else {
		if info.Verified {
			t.Error("Verified should be false on failure")
		}
	}
}

func TestCosignVerify_WithKeyPath(t *testing.T) {
	info, _ := CosignVerify("ghcr.io/test/image:latest", "/path/to/cosign.pub")
	if info.ArtifactPath != "ghcr.io/test/image:latest" {
		t.Errorf("ArtifactPath = %q, want image ref", info.ArtifactPath)
	}
}

func TestCosignVerify_WithoutKeyPath(t *testing.T) {
	info, _ := CosignVerify("ghcr.io/test/image:latest", "")
	if info.ArtifactPath != "ghcr.io/test/image:latest" {
		t.Errorf("ArtifactPath = %q, want image ref", info.ArtifactPath)
	}
}

// ---------------------------------------------------------------------------
// ReadSignatureManifest — invalid JSON
// ---------------------------------------------------------------------------

func TestReadSignatureManifest_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("{invalid json"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := ReadSignatureManifest(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// ---------------------------------------------------------------------------
// WriteSignatureManifest — round-trip with error fields
// ---------------------------------------------------------------------------

func TestWriteSignatureManifest_WithErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sigs.json")

	manifest := &SignatureManifest{
		Version:   "1.0.0",
		BuildTime: "2026-02-28T00:00:00Z",
		Signatures: []SignatureInfo{
			{
				ArtifactPath:    "/bin/test",
				ArtifactSHA256:  "abc123",
				SignatureMethod: "gpg",
				Error:           "gpg not found",
				Verified:        false,
			},
		},
	}

	if err := WriteSignatureManifest(path, manifest); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := ReadSignatureManifest(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if len(got.Signatures) != 1 {
		t.Fatalf("expected 1 signature, got %d", len(got.Signatures))
	}
	if got.Signatures[0].Error != "gpg not found" {
		t.Errorf("Error = %q, want 'gpg not found'", got.Signatures[0].Error)
	}
	if got.Signatures[0].Verified {
		t.Error("Verified should be false")
	}
}
