// Package manifest provides types and functions for the build manifest JSON schema.
// Schema matches the spec from the cloudflared-fips product prompt.
package manifest

// BuildManifest represents the build-manifest.json produced by every build.
// This schema is consumed by the compliance dashboard and AO documentation.
type BuildManifest struct {
	Version                    string            `json:"version"`
	Commit                     string            `json:"commit"`
	BuildTime                  string            `json:"build_time"`
	CloudflaredUpstreamVersion string            `json:"cloudflared_upstream_version"`
	CloudflaredUpstreamCommit  string            `json:"cloudflared_upstream_commit"`
	CryptoEngine               string            `json:"crypto_engine"`
	BoringSSlVersion           string            `json:"boringssl_version"`
	FIPSCertificates           []FIPSCertificate `json:"fips_certificates"`
	TargetPlatform             string            `json:"target_platform"`
	PackageFormat              string            `json:"package_format"`
	SBOMsha256                 string            `json:"sbom_sha256"`
	BinarySHA256               string            `json:"binary_sha256"`
}

// FIPSCertificate references a FIPS 140 validation certificate.
type FIPSCertificate struct {
	Module      string   `json:"module"`
	Certificate string   `json:"certificate"`
	Algorithms  []string `json:"algorithms"`
}
