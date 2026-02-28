// Command fips-proxy is a lightweight FIPS-compliant TLS reverse proxy.
// It provides per-site FIPS gateway functionality: client-facing TLS
// termination using BoringCrypto, with ClientHello inspection and JA4
// fingerprinting. Zero trust — upstream connections use FIPS TLS by default.
//
// Deploy at client sites or origin data centers as a symmetric FIPS gateway.
//
// Usage:
//
//	fips-proxy \
//	  --listen :443 \
//	  --tls-cert /etc/ssl/proxy.crt \
//	  --tls-key /etc/ssl/proxy.key \
//	  --upstream https://origin:8080 \
//	  --dashboard-addr localhost:8081
package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/cloudflared-fips/cloudflared-fips/internal/selftest"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/buildinfo"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/clientdetect"
)

const shutdownTimeout = 10 * time.Second

func main() {
	listenAddr := flag.String("listen", ":443", "TLS listen address")
	// Primary flag names match provision scripts; --cert/--key kept as aliases
	tlsCertFile := flag.String("tls-cert", "", "TLS certificate file (required)")
	tlsKeyFile := flag.String("tls-key", "", "TLS private key file (required)")
	certFileAlias := flag.String("cert", "", "TLS certificate file (alias for --tls-cert)")
	keyFileAlias := flag.String("key", "", "TLS private key file (alias for --tls-key)")
	upstream := flag.String("upstream", "http://localhost:8080", "upstream backend URL")
	upstreamTLS := flag.Bool("upstream-tls", true, "use FIPS TLS for upstream connections (zero trust)")
	upstreamInsecure := flag.Bool("upstream-insecure", false, "skip upstream TLS certificate verification (dev only)")
	dashboardAddr := flag.String("dashboard-addr", "127.0.0.1:8081", "dashboard/metrics listen address")
	runSelfTest := flag.Bool("selftest", true, "run FIPS self-test on startup")
	flag.Parse()

	// Resolve cert/key flag aliases: --tls-cert/--tls-key take priority
	certFile := *tlsCertFile
	if certFile == "" {
		certFile = *certFileAlias
	}
	keyFile := *tlsKeyFile
	if keyFile == "" {
		keyFile = *keyFileAlias
	}

	logger := log.New(os.Stderr, "[fips-proxy] ", log.LstdFlags)

	logger.Printf("%s", buildinfo.String())
	logger.Printf("FIPS Proxy — Per-Site FIPS Gateway")

	if certFile == "" || keyFile == "" {
		logger.Printf("Error: --tls-cert and --tls-key are required (or --cert/--key)")
		os.Exit(1)
	}

	// Run FIPS self-test
	if *runSelfTest {
		logger.Printf("Running FIPS self-test...")
		_, err := selftest.RunAllChecks()
		if err != nil {
			logger.Printf("FIPS self-test FAILED: %v", err)
			logger.Printf("Refusing to start proxy with failed crypto validation.")
			os.Exit(1)
		}
		logger.Printf("FIPS self-test passed.")
	}

	// Top-level context cancelled on SIGINT/SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Client TLS inspector
	inspector := clientdetect.NewInspector(10000)

	// FIPS TLS config
	tlsCfg := selftest.GetFIPSTLSConfig()
	tlsCfg.GetConfigForClient = inspector.GetConfigForClient

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		logger.Printf("Failed to load certificate: %v", err)
		os.Exit(1)
	}
	tlsCfg.Certificates = []tls.Certificate{cert}

	// Parse upstream — auto-upgrade to HTTPS when --upstream-tls is enabled
	upstreamStr := *upstream
	if *upstreamTLS && strings.HasPrefix(upstreamStr, "http://") {
		upstreamStr = "https://" + strings.TrimPrefix(upstreamStr, "http://")
		logger.Printf("Upstream TLS enabled: auto-upgraded %s → %s", *upstream, upstreamStr)
	}

	upstreamURL, err := url.Parse(upstreamStr)
	if err != nil {
		logger.Printf("Invalid upstream URL: %v", err)
		os.Exit(1)
	}

	// Reverse proxy with optional FIPS TLS transport for upstream
	proxy := httputil.NewSingleHostReverseProxy(upstreamURL)
	if upstreamURL.Scheme == "https" {
		upstreamTLSCfg := selftest.GetFIPSTLSConfig()
		upstreamTLSCfg.InsecureSkipVerify = *upstreamInsecure
		proxy.Transport = &http.Transport{
			TLSClientConfig:     upstreamTLSCfg,
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		}
		if *upstreamInsecure {
			logger.Printf("WARNING: upstream TLS certificate verification disabled (--upstream-insecure)")
		}
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, "upstream unreachable", http.StatusBadGateway)
	}

	// TLS listener
	listener, err := tls.Listen("tcp", *listenAddr, tlsCfg)
	if err != nil {
		logger.Printf("Failed to listen on %s: %v", *listenAddr, err)
		os.Exit(1)
	}

	logger.Printf("FIPS proxy listening on %s -> %s", *listenAddr, *upstream)

	// Dashboard/metrics server (plaintext, localhost only)
	dashServer := startDashboard(*dashboardAddr, inspector, logger)

	// Main FIPS proxy server
	server := &http.Server{
		Handler:           proxy,
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		ConnState: func(conn net.Conn, state http.ConnState) {
			if state == http.StateNew {
				// logged by inspector via GetConfigForClient
			}
		},
	}

	// Start server in background
	errCh := make(chan error, 1)
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	logger.Printf("Server ready")

	// Wait for shutdown signal or server error
	select {
	case <-ctx.Done():
		logger.Printf("Shutdown signal received, draining connections...")
	case err := <-errCh:
		logger.Printf("Server error: %v", err)
		os.Exit(1)
	}

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	// Shutdown both servers
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Printf("Proxy shutdown error: %v", err)
	}
	if err := dashServer.Shutdown(shutdownCtx); err != nil {
		logger.Printf("Dashboard shutdown error: %v", err)
	}

	logger.Printf("Server stopped gracefully")
}

func startDashboard(addr string, inspector *clientdetect.Inspector, logger *log.Logger) *http.Server {
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

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Printf("Dashboard/metrics on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Printf("Dashboard error: %v", err)
		}
	}()

	return server
}
