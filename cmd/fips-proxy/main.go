// Command fips-proxy is a lightweight FIPS-compliant TLS reverse proxy.
// It provides Tier 3 deployment: customer-controlled TLS termination using
// BoringCrypto, with ClientHello inspection and JA4 fingerprinting.
//
// Deploy in a GovCloud environment (AWS GovCloud, Azure Government, etc.)
// for complete control over the TLS termination point.
//
// Usage:
//
//	fips-proxy \
//	  --listen :443 \
//	  --cert /etc/ssl/proxy.crt \
//	  --key /etc/ssl/proxy.key \
//	  --upstream localhost:8080 \
//	  --dashboard-addr localhost:8081
package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"github.com/cloudflared-fips/cloudflared-fips/internal/selftest"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/buildinfo"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/clientdetect"
)

func main() {
	listenAddr := flag.String("listen", ":443", "TLS listen address")
	certFile := flag.String("cert", "", "TLS certificate file (required)")
	keyFile := flag.String("key", "", "TLS private key file (required)")
	upstream := flag.String("upstream", "http://localhost:8080", "upstream backend URL")
	dashboardAddr := flag.String("dashboard-addr", "127.0.0.1:8081", "dashboard/metrics listen address")
	runSelfTest := flag.Bool("selftest", true, "run FIPS self-test on startup")
	flag.Parse()

	fmt.Fprintf(os.Stderr, "%s\n", buildinfo.String())
	fmt.Fprintf(os.Stderr, "FIPS Proxy — Tier 3 Self-Hosted Edge\n")

	if *certFile == "" || *keyFile == "" {
		fmt.Fprintf(os.Stderr, "Error: --cert and --key are required\n")
		os.Exit(1)
	}

	// Run FIPS self-test
	if *runSelfTest {
		fmt.Fprintf(os.Stderr, "Running FIPS self-test...\n")
		_, err := selftest.RunAllChecks()
		if err != nil {
			fmt.Fprintf(os.Stderr, "FIPS self-test FAILED: %v\n", err)
			fmt.Fprintf(os.Stderr, "Refusing to start proxy with failed crypto validation.\n")
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "FIPS self-test passed.\n")
	}

	// Client TLS inspector
	inspector := clientdetect.NewInspector(10000)

	// FIPS TLS config
	tlsCfg := selftest.GetFIPSTLSConfig()
	tlsCfg.GetConfigForClient = inspector.GetConfigForClient

	cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load certificate: %v\n", err)
		os.Exit(1)
	}
	tlsCfg.Certificates = []tls.Certificate{cert}

	// Parse upstream
	upstreamURL, err := url.Parse(*upstream)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid upstream URL: %v\n", err)
		os.Exit(1)
	}

	// Reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(upstreamURL)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, "upstream unreachable", http.StatusBadGateway)
	}

	// TLS listener
	listener, err := tls.Listen("tcp", *listenAddr, tlsCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to listen on %s: %v\n", *listenAddr, err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "FIPS proxy listening on %s -> %s\n", *listenAddr, *upstream)

	// Dashboard/metrics server (plaintext, localhost only)
	go startDashboard(*dashboardAddr, inspector)

	// Serve
	server := &http.Server{
		Handler:      proxy,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
		ConnState: func(conn net.Conn, state http.ConnState) {
			// Connection state tracking for metrics
			if state == http.StateNew {
				// logged by inspector via GetConfigForClient
			}
		},
	}

	if err := server.Serve(listener); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

func startDashboard(addr string, inspector *clientdetect.Inspector) {
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"version": buildinfo.Version,
			"mode":    "fips-proxy-tier3",
		})
	})

	// Client TLS inspection results
	mux.HandleFunc("GET /api/v1/clients", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"recent":  inspector.RecentClients(100),
			"summary": inspector.FIPSStats(),
		})
	})

	// Metrics (Prometheus format)
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		stats := inspector.FIPSStats()
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "# HELP fips_proxy_clients_total Total TLS connections inspected\n")
		fmt.Fprintf(w, "# TYPE fips_proxy_clients_total counter\n")
		fmt.Fprintf(w, "fips_proxy_clients_total %d\n", stats.Total)
		fmt.Fprintf(w, "# HELP fips_proxy_clients_fips FIPS-capable client connections\n")
		fmt.Fprintf(w, "# TYPE fips_proxy_clients_fips counter\n")
		fmt.Fprintf(w, "fips_proxy_clients_fips %d\n", stats.FIPSCapable)
		fmt.Fprintf(w, "# HELP fips_proxy_clients_nonfips Non-FIPS client connections\n")
		fmt.Fprintf(w, "# TYPE fips_proxy_clients_nonfips counter\n")
		fmt.Fprintf(w, "fips_proxy_clients_nonfips %d\n", stats.NonFIPS)
	})

	// FIPS self-test (on-demand)
	mux.HandleFunc("GET /api/v1/selftest", func(w http.ResponseWriter, r *http.Request) {
		report, _ := selftest.GenerateReport(buildinfo.Version)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(report)
	})

	fmt.Fprintf(os.Stderr, "Dashboard/metrics on %s\n", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Fprintf(os.Stderr, "Dashboard error: %v\n", err)
	}
}

// Ensure io import is used (for potential future streaming)
var _ = io.EOF
