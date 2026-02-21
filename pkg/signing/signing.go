// Package signing provides artifact signing and verification utilities
// for cloudflared-fips build outputs. Supports GPG for binaries/packages
// and cosign (Sigstore) for container images.
package signing

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// SignatureInfo holds metadata about a signed artifact.
type SignatureInfo struct {
	ArtifactPath    string `json:"artifact_path"`
	ArtifactSHA256  string `json:"artifact_sha256"`
	SignaturePath   string `json:"signature_path,omitempty"`
	SignatureMethod string `json:"signature_method"` // "gpg", "cosign", "none"
	SignedAt        string `json:"signed_at,omitempty"`
	SignerIdentity  string `json:"signer_identity,omitempty"`
	Verified        bool   `json:"verified"`
	Error           string `json:"error,omitempty"`
}

// SignatureManifest holds signatures for all artifacts in a build.
type SignatureManifest struct {
	Version    string          `json:"version"`
	BuildTime  string          `json:"build_time"`
	Signatures []SignatureInfo `json:"signatures"`
	PublicKey  string          `json:"public_key_url,omitempty"`
}

// DefaultPublicKeyPath is the path to the project's public key relative to the repo root.
const DefaultPublicKeyPath = "configs/public-key.asc"

// DefaultPublicKeyURL is the URL where the public key can be downloaded.
const DefaultPublicKeyURL = "https://github.com/cloudflared-fips/cloudflared-fips/releases/latest/download/public-key.asc"

// HashFile computes the SHA-256 hash of a file.
func HashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// GPGSign signs a file with GPG, producing a detached signature (.sig file).
// Requires gpg to be installed and the signing key to be available.
func GPGSign(artifactPath, keyID string) (*SignatureInfo, error) {
	info := &SignatureInfo{
		ArtifactPath:    artifactPath,
		SignatureMethod: "gpg",
	}

	hash, err := HashFile(artifactPath)
	if err != nil {
		info.Error = err.Error()
		return info, err
	}
	info.ArtifactSHA256 = hash

	sigPath := artifactPath + ".sig"
	args := []string{
		"--batch", "--yes",
		"--detach-sign", "--armor",
		"--output", sigPath,
	}
	if keyID != "" {
		args = append(args, "--local-user", keyID)
	}
	args = append(args, artifactPath)

	cmd := exec.Command("gpg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		info.Error = fmt.Sprintf("gpg sign failed: %s: %s", err, string(output))
		return info, fmt.Errorf("gpg sign: %w", err)
	}

	info.SignaturePath = sigPath
	info.SignedAt = time.Now().UTC().Format(time.RFC3339)
	info.SignerIdentity = keyID
	info.Verified = true
	return info, nil
}

// GPGVerify verifies a GPG detached signature.
func GPGVerify(artifactPath, sigPath string) (*SignatureInfo, error) {
	info := &SignatureInfo{
		ArtifactPath:    artifactPath,
		SignaturePath:   sigPath,
		SignatureMethod: "gpg",
	}

	hash, err := HashFile(artifactPath)
	if err != nil {
		info.Error = err.Error()
		return info, err
	}
	info.ArtifactSHA256 = hash

	cmd := exec.Command("gpg", "--batch", "--verify", sigPath, artifactPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		info.Error = fmt.Sprintf("gpg verify failed: %s: %s", err, string(output))
		info.Verified = false
		return info, fmt.Errorf("gpg verify: %w", err)
	}

	info.Verified = true
	return info, nil
}

// CosignSign signs a container image with cosign (Sigstore).
// Requires cosign to be installed. Uses keyless signing by default
// (OIDC identity from CI environment).
func CosignSign(imageRef string, keyPath string) (*SignatureInfo, error) {
	info := &SignatureInfo{
		ArtifactPath:    imageRef,
		SignatureMethod: "cosign",
	}

	args := []string{"sign"}
	if keyPath != "" {
		args = append(args, "--key", keyPath)
	} else {
		// Keyless signing (Sigstore/Fulcio)
		args = append(args, "--yes")
	}
	args = append(args, imageRef)

	cmd := exec.Command("cosign", args...)
	cmd.Env = append(os.Environ(), "COSIGN_EXPERIMENTAL=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		info.Error = fmt.Sprintf("cosign sign failed: %s: %s", err, string(output))
		return info, fmt.Errorf("cosign sign: %w", err)
	}

	info.SignedAt = time.Now().UTC().Format(time.RFC3339)
	info.Verified = true
	return info, nil
}

// CosignVerify verifies a container image signature with cosign.
func CosignVerify(imageRef string, keyPath string) (*SignatureInfo, error) {
	info := &SignatureInfo{
		ArtifactPath:    imageRef,
		SignatureMethod: "cosign",
	}

	args := []string{"verify"}
	if keyPath != "" {
		args = append(args, "--key", keyPath)
	}
	args = append(args, imageRef)

	cmd := exec.Command("cosign", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		info.Error = fmt.Sprintf("cosign verify failed: %s: %s", err, string(output))
		info.Verified = false
		return info, fmt.Errorf("cosign verify: %w", err)
	}

	info.Verified = true
	return info, nil
}

// WriteSignatureManifest writes the signature manifest to a JSON file.
func WriteSignatureManifest(path string, manifest *SignatureManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// ReadSignatureManifest reads a signature manifest from a JSON file.
func ReadSignatureManifest(path string) (*SignatureManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var manifest SignatureManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	return &manifest, nil
}
