// Package buildinfo provides build metadata injected via ldflags at compile time.
package buildinfo

import "fmt"

// These variables are set at build time via -ldflags.
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
	FIPSBuild = "false"
)

// String returns a formatted build info string.
func String() string {
	return fmt.Sprintf("cloudflared-fips %s (commit: %s, built: %s, fips: %s)",
		Version, GitCommit, BuildDate, FIPSBuild)
}

// IsFIPS returns true if the binary was built with FIPS flags.
func IsFIPS() bool {
	return FIPSBuild == "true"
}
