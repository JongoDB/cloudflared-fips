package fipsbackend

import "os"

// GoNative implements Backend for Go 1.24+ native FIPS 140-3 module.
// Activated via GODEBUG=fips140=on (or =only for strict mode).
// Platform: all OS/arch combinations supported by Go.
// FIPS 140-3: CAVP A6650 — CMVP validation pending as of Feb 2026.
//
// CMVP tracking: https://csrc.nist.gov/projects/cryptographic-module-validation-program/modules-in-process/modules-in-process-list
// When the Go Cryptographic Module appears on the validated list, update
// CMVPCertificate and Validated accordingly.
type GoNative struct{}

// GoNativeCMVPValidated controls whether the Go native FIPS module has received
// CMVP validation. Set to true and update GoNativeCMVPCert when NIST issues the
// certificate. This is the single place to flip when validation completes.
var GoNativeCMVPValidated = false

// GoNativeCMVPCert is the CMVP certificate number once validated.
// Update this when NIST issues the certificate for Go Cryptographic Module.
var GoNativeCMVPCert = "CAVP A6650 (CMVP pending)"

func (g *GoNative) Name() string           { return "go-native" }
func (g *GoNative) DisplayName() string     { return "Go Cryptographic Module (native)" }
func (g *GoNative) CMVPCertificate() string { return GoNativeCMVPCert }
func (g *GoNative) Validated() bool         { return GoNativeCMVPValidated }

func (g *GoNative) FIPSStandard() string {
	if GoNativeCMVPValidated {
		return "140-3"
	}
	return "140-3 (pending)"
}

// Active checks if GODEBUG=fips140 is set to "on" or "only".
func (g *GoNative) Active() bool {
	godebug := os.Getenv("GODEBUG")
	if godebug == "" {
		return false
	}
	// Parse GODEBUG comma-separated key=value pairs
	for _, entry := range splitComma(godebug) {
		if entry == "fips140=on" || entry == "fips140=only" {
			return true
		}
	}
	return false
}

// SelfTest for Go native FIPS is handled by the runtime's own self-test
// which runs automatically when GODEBUG=fips140=on is set.
func (g *GoNative) SelfTest() (bool, error) {
	if !g.Active() {
		return false, nil
	}
	// Go native FIPS runs its own power-up self-test at init time.
	// If we got this far, the self-test passed.
	return true, nil
}

func splitComma(s string) []string {
	var result []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			if start < i {
				result = append(result, s[start:i])
			}
			start = i + 1
		}
	}
	return result
}
