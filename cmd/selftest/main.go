// Command selftest runs the FIPS 140-2 compliance self-test suite and outputs
// a structured JSON report to stdout.
package main

import (
	"fmt"
	"os"

	"github.com/cloudflared-fips/cloudflared-fips/internal/selftest"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/buildinfo"
)

func main() {
	fmt.Fprintf(os.Stderr, "%s\n", buildinfo.String())
	fmt.Fprintf(os.Stderr, "Running FIPS self-test suite...\n\n")

	report, err := selftest.GenerateReport(buildinfo.Version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "SELF-TEST FAILED: %v\n", err)
	}

	if printErr := selftest.PrintReport(report); printErr != nil {
		fmt.Fprintf(os.Stderr, "failed to print report: %v\n", printErr)
		os.Exit(2)
	}

	if err != nil {
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "\nAll self-tests passed.\n")
}
