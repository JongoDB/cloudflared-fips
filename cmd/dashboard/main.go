// Command dashboard starts the FIPS compliance dashboard HTTP server.
// It serves the React frontend (embedded) and the compliance API.
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/cloudflared-fips/cloudflared-fips/internal/compliance"
	"github.com/cloudflared-fips/cloudflared-fips/internal/dashboard"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/buildinfo"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	manifestPath := flag.String("manifest", "configs/build-manifest.json", "path to build manifest")
	staticDir := flag.String("static", "dashboard/dist", "path to static frontend files")
	flag.Parse()

	fmt.Fprintf(os.Stderr, "%s\n", buildinfo.String())
	fmt.Fprintf(os.Stderr, "Starting dashboard server on %s\n", *addr)

	checker := compliance.NewChecker()
	handler := dashboard.NewHandler(*manifestPath, checker)

	mux := http.NewServeMux()
	dashboard.RegisterRoutes(mux, handler)

	// Serve the React frontend
	fs := http.FileServer(http.Dir(*staticDir))
	mux.Handle("/", fs)

	if err := http.ListenAndServe(*addr, mux); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
