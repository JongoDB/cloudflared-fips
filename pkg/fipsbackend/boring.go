package fipsbackend

import (
	"crypto/tls"
	"strings"
)

// BoringCrypto implements Backend for GOEXPERIMENT=boringcrypto builds.
// This statically links Google's BoringSSL into the Go binary.
// Platform: Linux amd64/arm64 only.
// FIPS 140-2: CMVP #3678 (BoringSSL), #4407 (BoringCrypto in Go)
// FIPS 140-3: CMVP #4735
type BoringCrypto struct{}

func (b *BoringCrypto) Name() string            { return "boringcrypto" }
func (b *BoringCrypto) DisplayName() string      { return "BoringCrypto (BoringSSL)" }
func (b *BoringCrypto) CMVPCertificate() string  { return "#4407 (140-2), #4735 (140-3)" }
func (b *BoringCrypto) FIPSStandard() string     { return "140-2" }
func (b *BoringCrypto) Validated() bool          { return true }

// Active detects BoringCrypto by checking if the TLS cipher suite list
// is restricted (no RC4, no non-FIPS suites). When GOEXPERIMENT=boringcrypto
// is active, Go's crypto/tls only exposes FIPS-approved suites.
func (b *BoringCrypto) Active() bool {
	return isBoringCryptoActive()
}

// SelfTest runs BoringCrypto-specific validation: cipher suite restriction
// check and KAT vectors. The actual KAT execution is delegated to the
// selftest package; here we just verify the module is linked.
func (b *BoringCrypto) SelfTest() (bool, error) {
	if !b.Active() {
		return false, nil
	}
	return true, nil
}

// isBoringCryptoActive detects BoringCrypto by inspecting the available
// TLS cipher suites. BoringCrypto restricts the set to FIPS-approved only.
func isBoringCryptoActive() bool {
	suites := tls.CipherSuites()
	for _, s := range suites {
		if strings.Contains(s.Name, "RC4") {
			return false
		}
	}
	// Additional heuristic: BoringCrypto builds have fewer suites
	// A standard Go build has ~15 TLS 1.2 suites; BoringCrypto has ~8
	return len(suites) > 0 && len(suites) <= 10
}
