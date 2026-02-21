// Command selftest runs the FIPS compliance self-test suite and outputs
// a structured JSON report to stdout.
//
// Flags:
//
//	--verify-signature  Also verify the GPG signature of the running binary
//	--key-path          Path to GPG public key for signature verification
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/cloudflared-fips/cloudflared-fips/internal/selftest"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/buildinfo"
)

func main() {
	verifySignature := flag.Bool("verify-signature", false, "verify GPG signature of the running binary")
	keyPath := flag.String("key-path", "", "path to GPG public key for signature verification")
	flag.Parse()

	fmt.Fprintf(os.Stderr, "%s\n", buildinfo.String())
	fmt.Fprintf(os.Stderr, "Running FIPS self-test suite...\n\n")

	opts := selftest.Options{
		VerifySignature:  *verifySignature,
		SignatureKeyPath: *keyPath,
	}

	report, err := selftest.GenerateReportWithOptions(buildinfo.Version, opts)
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
