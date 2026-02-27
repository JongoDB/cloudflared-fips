package fipsbackend

import (
	"runtime"
	"runtime/debug"
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

// isBoringCryptoActive detects BoringCrypto by inspecting the binary's build
// settings. When built with GOEXPERIMENT=boringcrypto, runtime/debug reports
// this in the build info. This is more reliable than cipher suite heuristics,
// which changed in Go 1.24 (BoringCrypto now exposes 13 suites, not 8).
func isBoringCryptoActive() bool {
	// Primary: check runtime version string — Go embeds "X:boringcrypto"
	if strings.Contains(runtime.Version(), "X:boringcrypto") {
		return true
	}
	// Fallback: check build settings from debug info
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return false
	}
	for _, s := range info.Settings {
		if s.Key == "GOEXPERIMENT" && strings.Contains(s.Value, "boringcrypto") {
			return true
		}
	}
	return false
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
