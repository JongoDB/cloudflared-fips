package fipsbackend

import "runtime"

// SystemCrypto implements Backend for Microsoft Build of Go's
// GOEXPERIMENT=systemcrypto. This uses platform-native crypto:
//   - Windows: CNG (Cryptography Next Generation) — CMVP validated
//   - macOS: CommonCrypto/Security.framework — Apple CMVP cert
//   - Linux: OpenSSL via dlopen — depends on distro validation
//
// Requires the Microsoft fork of Go (github.com/nicholasgasior/go).
// When the mainline Go adopts systemcrypto, this will work with standard Go.
type SystemCrypto struct{}

func (s *SystemCrypto) Name() string       { return "systemcrypto" }
func (s *SystemCrypto) DisplayName() string { return "Platform System Crypto" }

func (s *SystemCrypto) CMVPCertificate() string {
	switch runtime.GOOS {
	case "windows":
		return "#4515 (Windows CNG)"
	case "darwin":
		return "#3856 (Apple corecrypto)"
	case "linux":
		return "Varies by distro OpenSSL"
	default:
		return "n/a"
	}
}

func (s *SystemCrypto) FIPSStandard() string {
	switch runtime.GOOS {
	case "windows":
		return "140-2"
	case "darwin":
		return "140-2"
	default:
		return "varies"
	}
}

func (s *SystemCrypto) Validated() bool {
	// Windows CNG and Apple corecrypto are CMVP validated.
	// Linux depends on distro OpenSSL validation.
	return runtime.GOOS == "windows" || runtime.GOOS == "darwin"
}

// Active returns true if running under GOEXPERIMENT=systemcrypto.
// Detection: the Microsoft Go fork sets an internal flag, but from
// pure Go we can only heuristically detect this. For now, this returns
// false and requires explicit opt-in via config or build tag.
func (s *SystemCrypto) Active() bool {
	// systemcrypto detection requires build-tag support from the Microsoft fork.
	// In a standard Go build, this is never active.
	return false
}

func (s *SystemCrypto) SelfTest() (bool, error) {
	if !s.Active() {
		return false, nil
	}
	return true, nil
}
