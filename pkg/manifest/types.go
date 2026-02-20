// Package manifest provides types and functions for the build manifest JSON schema.
package manifest

// BuildManifest represents the complete build manifest as specified in the project schema.
type BuildManifest struct {
	SchemaVersion    string            `json:"schema_version"`
	BuildID          string            `json:"build_id"`
	Timestamp        string            `json:"timestamp"`
	Source           Source            `json:"source"`
	Platform         Platform          `json:"platform"`
	GoVersion        string            `json:"go_version"`
	GoExperiment     string            `json:"go_experiment"`
	CGOEnabled       bool              `json:"cgo_enabled"`
	FIPSCertificates []FIPSCertificate `json:"fips_certificates"`
	Binary           Binary            `json:"binary"`
	SBOM             SBOM              `json:"sbom"`
	Verification     Verification      `json:"verification"`
}

// Source identifies the source code used for the build.
type Source struct {
	Repository string `json:"repository"`
	Branch     string `json:"branch"`
	Commit     string `json:"commit"`
	Tag        string `json:"tag"`
}

// Platform describes the build and target platform.
type Platform struct {
	BuildOS   string `json:"build_os"`
	BuildArch string `json:"build_arch"`
	TargetOS  string `json:"target_os"`
	TargetArch string `json:"target_arch"`
	Builder   string `json:"builder"`
}

// FIPSCertificate references a FIPS 140 validation certificate.
type FIPSCertificate struct {
	Module      string `json:"module"`
	CertNumber  string `json:"cert_number"`
	Level       string `json:"level"`
	ValidatedOn string `json:"validated_on"`
	ExpiresOn   string `json:"expires_on"`
	CMVPURL     string `json:"cmvp_url"`
}

// Binary contains metadata about the built binary.
type Binary struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	SHA256   string `json:"sha256"`
	Size     int64  `json:"size"`
	Stripped bool   `json:"stripped"`
}

// SBOM references the Software Bill of Materials.
type SBOM struct {
	Format string `json:"format"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// Verification contains post-build verification results.
type Verification struct {
	BoringCryptoSymbols bool     `json:"boring_crypto_symbols"`
	SelfTestPassed      bool     `json:"self_test_passed"`
	BannedCiphers       []string `json:"banned_ciphers"`
	StaticLinked        bool     `json:"static_linked"`
}
