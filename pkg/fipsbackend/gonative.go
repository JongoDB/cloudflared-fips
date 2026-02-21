package fipsbackend

import "os"

// GoNative implements Backend for Go 1.24+ native FIPS 140-3 module.
// Activated via GODEBUG=fips140=on (or =only for strict mode).
// Platform: all OS/arch combinations supported by Go.
// FIPS 140-3: CAVP A6650 — CMVP validation pending as of 2025.
type GoNative struct{}

func (g *GoNative) Name() string            { return "go-native" }
func (g *GoNative) DisplayName() string      { return "Go Cryptographic Module (native)" }
func (g *GoNative) CMVPCertificate() string  { return "CAVP A6650 (CMVP pending)" }
func (g *GoNative) FIPSStandard() string     { return "140-3 (pending)" }
func (g *GoNative) Validated() bool          { return false }

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
