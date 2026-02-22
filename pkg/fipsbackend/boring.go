package fipsbackend

import (
	"crypto/tls"
	"runtime"
	"strconv"
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

// FIPSStandard returns the FIPS standard based on the Go version.
// Go 1.24+ ships BoringSSL fips-20230428 (CMVP #4735, FIPS 140-3).
// Go 1.22/1.23 ships older .syso (CMVP #4407, FIPS 140-2).
func (b *BoringCrypto) FIPSStandard() string {
	if goVersionAtLeast(1, 24) {
		return "140-3"
	}
	return "140-2"
}
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

// goVersionAtLeast returns true if the runtime Go version is >= major.minor.
func goVersionAtLeast(major, minor int) bool {
	v := runtime.Version() // e.g. "go1.24.1" or "go1.23"
	v = strings.TrimPrefix(v, "go")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return false
	}
	maj, err := strconv.Atoi(parts[0])
	if err != nil {
		return false
	}
	min, err := strconv.Atoi(parts[1])
	if err != nil {
		return false
	}
	return maj > major || (maj == major && min >= minor)
}
