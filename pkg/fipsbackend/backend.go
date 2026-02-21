// Package fipsbackend provides a runtime-selectable FIPS cryptographic module
// abstraction. The active backend is determined at build time via build tags
// (boringcrypto) or runtime detection (GODEBUG=fips140, system crypto).
//
// Each backend reports its CMVP certificate, FIPS standard (140-2 vs 140-3),
// validation status, and can run module-specific self-tests.
package fipsbackend

// Backend describes a FIPS cryptographic module that may be active at runtime.
type Backend interface {
	// Name returns the backend identifier: "boringcrypto", "go-native", "systemcrypto", "none".
	Name() string

	// DisplayName returns a human-friendly label for the dashboard.
	DisplayName() string

	// CMVPCertificate returns the CMVP certificate number, e.g. "#3678", or
	// "pending (CAVP A6650)" if CAVP-tested but not yet CMVP-validated.
	CMVPCertificate() string

	// FIPSStandard returns "140-2", "140-3", or "140-3 (pending)".
	FIPSStandard() string

	// Validated returns true if the module holds an active CMVP certificate.
	Validated() bool

	// Active returns true if this backend is the one currently in use.
	Active() bool

	// SelfTest runs the module-specific Known Answer Tests.
	// Returns true if all tests pass, along with any error details.
	SelfTest() (bool, error)
}

// Info is a JSON-serializable snapshot of a backend's identity and validation state.
type Info struct {
	Name            string `json:"name"`
	DisplayName     string `json:"display_name"`
	CMVPCertificate string `json:"cmvp_certificate"`
	FIPSStandard    string `json:"fips_standard"`
	Validated       bool   `json:"validated"`
	Active          bool   `json:"active"`
}

// ToInfo converts a Backend to a serializable Info struct.
func ToInfo(b Backend) Info {
	return Info{
		Name:            b.Name(),
		DisplayName:     b.DisplayName(),
		CMVPCertificate: b.CMVPCertificate(),
		FIPSStandard:    b.FIPSStandard(),
		Validated:       b.Validated(),
		Active:          b.Active(),
	}
}

// AllBackends returns all known FIPS backend implementations.
// The caller can filter by Active() to find the one in use.
func AllBackends() []Backend {
	return []Backend{
		&BoringCrypto{},
		&GoNative{},
		&SystemCrypto{},
	}
}

// Detect returns the currently active FIPS backend, or nil if none is active.
func Detect() Backend {
	for _, b := range AllBackends() {
		if b.Active() {
			return b
		}
	}
	return nil
}

// DetectInfo returns Info for the active backend, or a "none" placeholder.
func DetectInfo() Info {
	b := Detect()
	if b == nil {
		return Info{
			Name:            "none",
			DisplayName:     "No FIPS Module",
			CMVPCertificate: "n/a",
			FIPSStandard:    "n/a",
			Validated:       false,
			Active:          false,
		}
	}
	return ToInfo(b)
}
