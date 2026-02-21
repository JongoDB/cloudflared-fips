package selftest

import "crypto/tls"

// FIPSApprovedCipherSuites lists TLS 1.2 cipher suites approved for FIPS 140-2
// per NIST SP 800-52 Rev. 2.
var FIPSApprovedCipherSuites = map[uint16]string{
	// TLS 1.2 AES-GCM suites
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256:   "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384:   "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256: "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384: "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",

	// TLS 1.2 AES-CBC suites (FIPS-approved but GCM preferred)
	tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256:   "TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256",
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256: "TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256",

	// RSA key exchange (FIPS-approved but not recommended — no forward secrecy)
	tls.TLS_RSA_WITH_AES_128_GCM_SHA256: "TLS_RSA_WITH_AES_128_GCM_SHA256",
	tls.TLS_RSA_WITH_AES_256_GCM_SHA384: "TLS_RSA_WITH_AES_256_GCM_SHA384",
}

// FIPSApprovedTLS13Suites lists TLS 1.3 cipher suites that are FIPS-approved.
// Note: TLS_CHACHA20_POLY1305_SHA256 is explicitly excluded. While Go's TLS 1.3
// implementation always makes it available for negotiation (it cannot be disabled
// via tls.Config.CipherSuites), ChaCha20-Poly1305 is NOT a FIPS-approved algorithm
// and its implementation in golang.org/x/crypto does not route through BoringCrypto.
// See docs/quic-go-crypto-audit.md for details.
var FIPSApprovedTLS13Suites = map[uint16]string{
	tls.TLS_AES_128_GCM_SHA256: "TLS_AES_128_GCM_SHA256",
	tls.TLS_AES_256_GCM_SHA384: "TLS_AES_256_GCM_SHA384",
	// TLS_CHACHA20_POLY1305_SHA256 intentionally omitted — not FIPS-approved
}

// BannedCipherPatterns contains substrings that identify non-FIPS cipher suites.
var BannedCipherPatterns = []string{
	"RC4",
	"DES",
	"3DES",
	"NULL",
	"EXPORT",
	"anon",
}

// IsFIPSApproved returns true if the given cipher suite ID is in the FIPS-approved list.
func IsFIPSApproved(id uint16) bool {
	if _, ok := FIPSApprovedCipherSuites[id]; ok {
		return true
	}
	if _, ok := FIPSApprovedTLS13Suites[id]; ok {
		return true
	}
	return false
}

// GetFIPSTLSConfig returns a tls.Config restricted to FIPS-approved cipher suites and protocols.
func GetFIPSTLSConfig() *tls.Config {
	var suiteIDs []uint16
	for id := range FIPSApprovedCipherSuites {
		suiteIDs = append(suiteIDs, id)
	}

	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		MaxVersion:   tls.VersionTLS13,
		CipherSuites: suiteIDs,
		CurvePreferences: []tls.CurveID{
			tls.CurveP256,
			tls.CurveP384,
		},
	}
}
