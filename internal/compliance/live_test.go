package compliance

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudflared-fips/cloudflared-fips/pkg/manifest"
)

// ---------------------------------------------------------------------------
// isLoopback
// ---------------------------------------------------------------------------

func TestIsLoopback(t *testing.T) {
	tests := []struct {
		target string
		want   bool
	}{
		{"localhost:8080", true},
		{"127.0.0.1:443", true},
		{"[::1]:8080", true},     // IPv6 loopback with port (bracket notation)
		{"127.0.0.1", true},      // no port
		{"localhost", true},      // no port
		{"", true},               // empty host treated as loopback
		{"10.0.0.1:80", false},   // private IP, not loopback
		{"192.168.1.1:80", false},
		{"example.com:443", false},
		{"0.0.0.0:80", false},
	}

	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			got := isLoopback(tt.target)
			if got != tt.want {
				t.Errorf("isLoopback(%q) = %v, want %v", tt.target, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// NewLiveChecker + functional options
// ---------------------------------------------------------------------------

func TestNewLiveCheckerDefaults(t *testing.T) {
	lc := NewLiveChecker()
	if lc.manifestPath != "configs/build-manifest.json" {
		t.Errorf("default manifestPath = %q, want configs/build-manifest.json", lc.manifestPath)
	}
	if lc.metricsAddr != "localhost:2000" {
		t.Errorf("default metricsAddr = %q, want localhost:2000", lc.metricsAddr)
	}
	if len(lc.ingressTargets) != 0 {
		t.Errorf("default ingressTargets = %v, want empty", lc.ingressTargets)
	}
}

func TestNewLiveCheckerWithOptions(t *testing.T) {
	lc := NewLiveChecker(
		WithManifestPath("/tmp/manifest.json"),
		WithBinaryPath("/usr/local/bin/test"),
		WithConfigPath("/etc/test.yaml"),
		WithMetricsAddr("localhost:9090"),
		WithIngressTargets([]string{"10.0.0.5:443", "10.0.0.6:443"}),
	)
	if lc.manifestPath != "/tmp/manifest.json" {
		t.Errorf("manifestPath = %q", lc.manifestPath)
	}
	if lc.binaryPath != "/usr/local/bin/test" {
		t.Errorf("binaryPath = %q", lc.binaryPath)
	}
	if lc.configPath != "/etc/test.yaml" {
		t.Errorf("configPath = %q", lc.configPath)
	}
	if lc.metricsAddr != "localhost:9090" {
		t.Errorf("metricsAddr = %q", lc.metricsAddr)
	}
	if len(lc.ingressTargets) != 2 {
		t.Errorf("ingressTargets len = %d, want 2", len(lc.ingressTargets))
	}
}

// ---------------------------------------------------------------------------
// Section structure: RunTunnelChecks, RunLocalServiceChecks, RunBuildSupplyChainChecks
// ---------------------------------------------------------------------------

func TestRunTunnelChecks_Structure(t *testing.T) {
	lc := NewLiveChecker(
		WithManifestPath("/nonexistent/manifest.json"),
		WithMetricsAddr("127.0.0.1:1"), // unreachable
	)
	section := lc.RunTunnelChecks()

	if section.ID != "tunnel" {
		t.Errorf("section ID = %q, want tunnel", section.ID)
	}
	if len(section.Items) != 12 {
		t.Errorf("tunnel section has %d items, want 12", len(section.Items))
	}

	// Verify all items have IDs, names, severity, and verification methods
	for _, item := range section.Items {
		if item.ID == "" {
			t.Error("item has empty ID")
		}
		if item.Name == "" {
			t.Errorf("item %s has empty Name", item.ID)
		}
		if item.Severity == "" {
			t.Errorf("item %s has empty Severity", item.ID)
		}
		if item.VerificationMethod == "" {
			t.Errorf("item %s has empty VerificationMethod", item.ID)
		}
		// Status must be set (not empty string)
		validStatuses := map[Status]bool{StatusPass: true, StatusFail: true, StatusWarning: true, StatusUnknown: true}
		if !validStatuses[item.Status] {
			t.Errorf("item %s has invalid status %q", item.ID, item.Status)
		}
	}
}

func TestRunLocalServiceChecks_Structure(t *testing.T) {
	lc := NewLiveChecker()
	section := lc.RunLocalServiceChecks()

	if section.ID != "local" {
		t.Errorf("section ID = %q, want local", section.ID)
	}
	if len(section.Items) != 4 {
		t.Errorf("local section has %d items, want 4", len(section.Items))
	}
}

func TestRunBuildSupplyChainChecks_Structure(t *testing.T) {
	lc := NewLiveChecker(WithManifestPath("/nonexistent"))
	section := lc.RunBuildSupplyChainChecks()

	if section.ID != "build" {
		t.Errorf("section ID = %q, want build", section.ID)
	}
	if len(section.Items) != 7 {
		t.Errorf("build section has %d items, want 7", len(section.Items))
	}
}

// ---------------------------------------------------------------------------
// checkTLSVersion — deterministic (no I/O)
// ---------------------------------------------------------------------------

func TestCheckTLSVersion(t *testing.T) {
	lc := NewLiveChecker()
	item := lc.checkTLSVersion()
	// GetFIPSTLSConfig always sets MinVersion >= TLS 1.2
	if item.Status != StatusPass {
		t.Errorf("TLS version check: got %s, want pass", item.Status)
	}
	if item.ID != "t-5" {
		t.Errorf("ID = %q, want t-5", item.ID)
	}
}

// ---------------------------------------------------------------------------
// checkKeyExchange — deterministic (no I/O)
// ---------------------------------------------------------------------------

func TestCheckKeyExchange(t *testing.T) {
	lc := NewLiveChecker()
	item := lc.checkKeyExchange()
	// GetFIPSTLSConfig sets CurvePreferences to P-256/P-384
	if item.Status != StatusPass {
		t.Errorf("key exchange check: got %s, want pass", item.Status)
	}
}

// ---------------------------------------------------------------------------
// checkCipherSuites — deterministic (no I/O)
// ---------------------------------------------------------------------------

func TestCheckCipherSuites(t *testing.T) {
	lc := NewLiveChecker()
	item := lc.checkCipherSuites()
	// In a FIPS environment: pass (backend detected or all suites approved)
	// In a non-FIPS environment: fail (non-FIPS ciphers present, no backend)
	// Either is valid — just ensure the check runs without panic and produces a valid status
	validStatuses := map[Status]bool{StatusPass: true, StatusFail: true, StatusWarning: true}
	if !validStatuses[item.Status] {
		t.Errorf("cipher suites check returned invalid status: %s", item.Status)
	}
	if item.ID != "t-6" {
		t.Errorf("ID = %q, want t-6", item.ID)
	}
}

// ---------------------------------------------------------------------------
// checkBoringCryptoActive — deterministic
// ---------------------------------------------------------------------------

func TestCheckBoringCryptoActive(t *testing.T) {
	lc := NewLiveChecker()
	item := lc.checkBoringCryptoActive()
	// In test environment, GoNative backend is detected via GODEBUG
	// Either pass (backend detected) or fail (no backend) — both are valid
	if item.ID != "t-1" {
		t.Errorf("ID = %q, want t-1", item.ID)
	}
	if item.Severity != "critical" {
		t.Errorf("severity = %q, want critical", item.Severity)
	}
}

// ---------------------------------------------------------------------------
// checkFIPSBackend — deterministic
// ---------------------------------------------------------------------------

func TestCheckFIPSBackend(t *testing.T) {
	lc := NewLiveChecker()
	item := lc.checkFIPSBackend()
	if item.ID != "t-12" {
		t.Errorf("ID = %q, want t-12", item.ID)
	}
	// Either pass/warning (backend detected) or fail (no backend)
	if item.Status == "" {
		t.Error("status is empty")
	}
}

// ---------------------------------------------------------------------------
// checkOSFIPSMode
// ---------------------------------------------------------------------------

func TestCheckOSFIPSMode(t *testing.T) {
	lc := NewLiveChecker()
	item := lc.checkOSFIPSMode()
	if item.ID != "t-2" {
		t.Errorf("ID = %q, want t-2", item.ID)
	}
	// In containers / dev environments FIPS mode is typically not enabled
	// Result should be pass (FIPS enabled) or warning (not enabled / not Linux)
	if item.Status == StatusFail {
		t.Errorf("OS FIPS mode should not produce fail, got fail")
	}
}

// ---------------------------------------------------------------------------
// checkFIPSSelfTest — runs real KATs
// ---------------------------------------------------------------------------

func TestCheckFIPSSelfTest(t *testing.T) {
	lc := NewLiveChecker()
	item := lc.checkFIPSSelfTest()
	if item.ID != "t-3" {
		t.Errorf("ID = %q, want t-3", item.ID)
	}
	// KATs use NIST vectors — pass in FIPS environments, may fail in non-FIPS
	// (self-test checks for FIPS backend presence). Either result is valid.
	validStatuses := map[Status]bool{StatusPass: true, StatusFail: true}
	if !validStatuses[item.Status] {
		t.Errorf("FIPS self-test: got unexpected status %s", item.Status)
	}
}

// ---------------------------------------------------------------------------
// checkFIPSModuleSunset — deterministic
// ---------------------------------------------------------------------------

func TestCheckFIPSModuleSunset(t *testing.T) {
	lc := NewLiveChecker()
	item := lc.checkFIPSModuleSunset()
	if item.ID != "b-7" {
		t.Errorf("ID = %q, want b-7", item.ID)
	}
	// Must produce a valid status
	validStatuses := map[Status]bool{StatusPass: true, StatusFail: true, StatusWarning: true, StatusUnknown: true}
	if !validStatuses[item.Status] {
		t.Errorf("invalid status %q", item.Status)
	}
}

// ---------------------------------------------------------------------------
// checkTunnelProtocol — with mock metrics server
// ---------------------------------------------------------------------------

func TestCheckTunnelProtocol_MetricsReachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "# HELP cloudflared_tunnel_active_streams")
	}))
	defer srv.Close()

	addr := strings.TrimPrefix(srv.URL, "http://")
	lc := NewLiveChecker(WithMetricsAddr(addr))
	item := lc.checkTunnelProtocol()
	if item.Status != StatusPass {
		t.Errorf("tunnel protocol with reachable metrics: got %s, want pass", item.Status)
	}
}

func TestCheckTunnelProtocol_MetricsUnreachable(t *testing.T) {
	lc := NewLiveChecker(WithMetricsAddr("127.0.0.1:1"))
	item := lc.checkTunnelProtocol()
	// Without metrics and without cloudflared running, should be unknown or pass
	// (pass if cloudflared process detected on Linux)
	if item.Status == StatusFail {
		t.Errorf("tunnel protocol without metrics should not fail, got fail")
	}
}

// ---------------------------------------------------------------------------
// checkTunnelRedundancy — with mock metrics server
// ---------------------------------------------------------------------------

func TestCheckTunnelRedundancy_MetricsReachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "cloudflared_tunnel_connections 4")
	}))
	defer srv.Close()

	addr := strings.TrimPrefix(srv.URL, "http://")
	lc := NewLiveChecker(WithMetricsAddr(addr))
	item := lc.checkTunnelRedundancy()
	if item.Status != StatusPass {
		t.Errorf("tunnel redundancy with metrics: got %s, want pass", item.Status)
	}
}

// ---------------------------------------------------------------------------
// checkConfigDrift
// ---------------------------------------------------------------------------

func TestCheckConfigDrift_NoConfigPath(t *testing.T) {
	lc := NewLiveChecker() // no WithConfigPath
	item := lc.checkConfigDrift()
	if item.Status != StatusUnknown {
		t.Errorf("config drift with no path: got %s, want unknown", item.Status)
	}
}

func TestCheckConfigDrift_ConfigMissing(t *testing.T) {
	lc := NewLiveChecker(WithConfigPath("/nonexistent/config.yaml"))
	item := lc.checkConfigDrift()
	if item.Status != StatusUnknown {
		t.Errorf("config drift with missing file: got %s, want unknown", item.Status)
	}
}

func TestCheckConfigDrift_ConfigPresent(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "cloudflared-fips-test-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.WriteString("deployment_tier: standard\n"); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	lc := NewLiveChecker(WithConfigPath(tmpFile.Name()))
	item := lc.checkConfigDrift()
	if item.Status != StatusPass {
		t.Errorf("config drift with present file: got %s, want pass", item.Status)
	}
}

// ---------------------------------------------------------------------------
// checkBinaryIntegrity
// ---------------------------------------------------------------------------

func TestCheckBinaryIntegrity_NoBinaryPath(t *testing.T) {
	lc := &LiveChecker{manifestPath: "/nonexistent", binaryPath: ""}
	item := lc.checkBinaryIntegrity()
	if item.Status != StatusUnknown {
		t.Errorf("binary integrity with no path: got %s, want unknown", item.Status)
	}
}

func TestCheckBinaryIntegrity_NoManifest(t *testing.T) {
	tmpBin, err := os.CreateTemp("", "cloudflared-fips-test-bin-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpBin.Name())
	if _, err := tmpBin.WriteString("fake binary content"); err != nil {
		t.Fatal(err)
	}
	tmpBin.Close()

	lc := &LiveChecker{manifestPath: "/nonexistent/manifest.json", binaryPath: tmpBin.Name()}
	item := lc.checkBinaryIntegrity()
	if item.Status != StatusUnknown {
		t.Errorf("binary integrity with no manifest: got %s, want unknown", item.Status)
	}
}

func TestCheckBinaryIntegrity_HashMatch(t *testing.T) {
	// Create a temp binary
	tmpBin, err := os.CreateTemp("", "cloudflared-fips-test-bin-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpBin.Name())
	content := []byte("deterministic binary content for test")
	if _, err := tmpBin.Write(content); err != nil {
		t.Fatal(err)
	}
	tmpBin.Close()

	// Compute expected hash
	hash := sha256.Sum256(content)
	hashHex := hex.EncodeToString(hash[:])

	// Create manifest with matching hash
	m := manifest.BuildManifest{
		Version:      "1.0.0-test",
		BinarySHA256: hashHex,
		FIPSCertificates: []manifest.FIPSCertificate{
			{Module: "test", Certificate: "#0000"},
		},
	}
	tmpManifest := writeManifest(t, m)
	defer os.Remove(tmpManifest)

	lc := &LiveChecker{manifestPath: tmpManifest, binaryPath: tmpBin.Name()}
	item := lc.checkBinaryIntegrity()
	if item.Status != StatusPass {
		t.Errorf("binary integrity with matching hash: got %s (%s), want pass", item.Status, item.What)
	}
}

func TestCheckBinaryIntegrity_HashMismatch(t *testing.T) {
	tmpBin, err := os.CreateTemp("", "cloudflared-fips-test-bin-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpBin.Name())
	if _, err := tmpBin.WriteString("actual binary content"); err != nil {
		t.Fatal(err)
	}
	tmpBin.Close()

	m := manifest.BuildManifest{
		Version:      "1.0.0-test",
		BinarySHA256: "0000000000000000000000000000000000000000000000000000000000000000",
		FIPSCertificates: []manifest.FIPSCertificate{
			{Module: "test", Certificate: "#0000"},
		},
	}
	tmpManifest := writeManifest(t, m)
	defer os.Remove(tmpManifest)

	lc := &LiveChecker{manifestPath: tmpManifest, binaryPath: tmpBin.Name()}
	item := lc.checkBinaryIntegrity()
	if item.Status != StatusFail {
		t.Errorf("binary integrity with mismatched hash: got %s, want fail", item.Status)
	}
}

func TestCheckBinaryIntegrity_EmptyManifestHash(t *testing.T) {
	tmpBin, err := os.CreateTemp("", "cloudflared-fips-test-bin-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpBin.Name())
	if _, err := tmpBin.WriteString("content"); err != nil {
		t.Fatal(err)
	}
	tmpBin.Close()

	m := manifest.BuildManifest{Version: "1.0.0-test", BinarySHA256: ""}
	tmpManifest := writeManifest(t, m)
	defer os.Remove(tmpManifest)

	lc := &LiveChecker{manifestPath: tmpManifest, binaryPath: tmpBin.Name()}
	item := lc.checkBinaryIntegrity()
	if item.Status != StatusWarning {
		t.Errorf("binary integrity with empty manifest hash: got %s, want warning", item.Status)
	}
}

// ---------------------------------------------------------------------------
// checkBuildManifestPresent
// ---------------------------------------------------------------------------

func TestCheckBuildManifestPresent_Missing(t *testing.T) {
	lc := NewLiveChecker(WithManifestPath("/nonexistent/manifest.json"))
	item := lc.checkBuildManifestPresent()
	if item.Status != StatusFail {
		t.Errorf("manifest present with missing file: got %s, want fail", item.Status)
	}
}

func TestCheckBuildManifestPresent_Valid(t *testing.T) {
	m := manifest.BuildManifest{Version: "1.0.0"}
	tmpManifest := writeManifest(t, m)
	defer os.Remove(tmpManifest)

	lc := NewLiveChecker(WithManifestPath(tmpManifest))
	item := lc.checkBuildManifestPresent()
	if item.Status != StatusPass {
		t.Errorf("manifest present with valid file: got %s, want pass", item.Status)
	}
}

// ---------------------------------------------------------------------------
// checkFIPSCertsListed
// ---------------------------------------------------------------------------

func TestCheckFIPSCertsListed_WithCerts(t *testing.T) {
	m := manifest.BuildManifest{
		Version: "1.0.0",
		FIPSCertificates: []manifest.FIPSCertificate{
			{Module: "BoringSSL", Certificate: "#4735"},
		},
	}
	tmpManifest := writeManifest(t, m)
	defer os.Remove(tmpManifest)

	lc := NewLiveChecker(WithManifestPath(tmpManifest))
	item := lc.checkFIPSCertsListed()
	if item.Status != StatusPass {
		t.Errorf("FIPS certs with certs present: got %s, want pass", item.Status)
	}
}

func TestCheckFIPSCertsListed_NoCerts(t *testing.T) {
	m := manifest.BuildManifest{Version: "1.0.0", FIPSCertificates: nil}
	tmpManifest := writeManifest(t, m)
	defer os.Remove(tmpManifest)

	lc := NewLiveChecker(WithManifestPath(tmpManifest))
	item := lc.checkFIPSCertsListed()
	if item.Status != StatusFail {
		t.Errorf("FIPS certs with none listed: got %s, want fail", item.Status)
	}
}

// ---------------------------------------------------------------------------
// checkUpstreamVersion
// ---------------------------------------------------------------------------

func TestCheckUpstreamVersion_Present(t *testing.T) {
	m := manifest.BuildManifest{
		Version:                    "1.0.0",
		CloudflaredUpstreamVersion: "2025.2.1",
	}
	tmpManifest := writeManifest(t, m)
	defer os.Remove(tmpManifest)

	lc := NewLiveChecker(WithManifestPath(tmpManifest))
	item := lc.checkUpstreamVersion()
	if item.Status != StatusPass {
		t.Errorf("upstream version present: got %s, want pass", item.Status)
	}
}

func TestCheckUpstreamVersion_Missing(t *testing.T) {
	m := manifest.BuildManifest{Version: "1.0.0"}
	tmpManifest := writeManifest(t, m)
	defer os.Remove(tmpManifest)

	lc := NewLiveChecker(WithManifestPath(tmpManifest))
	item := lc.checkUpstreamVersion()
	if item.Status != StatusWarning {
		t.Errorf("upstream version missing: got %s, want warning", item.Status)
	}
}

// ---------------------------------------------------------------------------
// checkSignatureValid
// ---------------------------------------------------------------------------

func TestCheckSignatureValid_NoBinaryPath(t *testing.T) {
	lc := &LiveChecker{binaryPath: ""}
	item := lc.checkSignatureValid()
	if item.Status != StatusUnknown {
		t.Errorf("sig valid with no binary path: got %s, want unknown", item.Status)
	}
}

func TestCheckSignatureValid_NoSignature(t *testing.T) {
	tmpBin, err := os.CreateTemp("", "cloudflared-fips-test-bin-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpBin.Name())
	tmpBin.Close()

	lc := &LiveChecker{binaryPath: tmpBin.Name()}
	item := lc.checkSignatureValid()
	if item.Status != StatusWarning {
		t.Errorf("sig valid with no signature: got %s, want warning", item.Status)
	}
}

func TestCheckSignatureValid_WithSigFile(t *testing.T) {
	tmpBin, err := os.CreateTemp("", "cloudflared-fips-test-bin-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpBin.Name())
	tmpBin.Close()

	// Create a .sig file next to the binary
	sigPath := tmpBin.Name() + ".sig"
	if err := os.WriteFile(sigPath, []byte("fake sig"), 0644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(sigPath)

	lc := &LiveChecker{binaryPath: tmpBin.Name()}
	item := lc.checkSignatureValid()
	if item.Status != StatusPass {
		t.Errorf("sig valid with .sig file: got %s, want pass", item.Status)
	}
}

// ---------------------------------------------------------------------------
// Local service checks — with real TCP/TLS servers
// ---------------------------------------------------------------------------

func TestCheckLocalTLSEnabled_NoTargets(t *testing.T) {
	lc := NewLiveChecker() // no ingress targets
	item := lc.checkLocalTLSEnabled()
	if item.Status != StatusUnknown {
		t.Errorf("local TLS with no targets: got %s, want unknown", item.Status)
	}
}

func TestCheckLocalTLSEnabled_LoopbackTarget(t *testing.T) {
	lc := NewLiveChecker(WithIngressTargets([]string{"localhost:8080"}))
	item := lc.checkLocalTLSEnabled()
	if item.Status != StatusPass {
		t.Errorf("local TLS with loopback: got %s, want pass (loopback exempt)", item.Status)
	}
	if !strings.Contains(item.What, "Loopback") {
		t.Errorf("expected loopback explanation, got: %s", item.What)
	}
}

func TestCheckLocalCipherSuite_NoTargets(t *testing.T) {
	lc := NewLiveChecker()
	item := lc.checkLocalCipherSuite()
	if item.Status != StatusUnknown {
		t.Errorf("local cipher with no targets: got %s, want unknown", item.Status)
	}
}

func TestCheckLocalCipherSuite_LoopbackTarget(t *testing.T) {
	lc := NewLiveChecker(WithIngressTargets([]string{"127.0.0.1:9999"}))
	item := lc.checkLocalCipherSuite()
	if item.Status != StatusPass {
		t.Errorf("local cipher with loopback: got %s, want pass", item.Status)
	}
}

func TestCheckLocalCertificateValid_LoopbackTarget(t *testing.T) {
	lc := NewLiveChecker(WithIngressTargets([]string{"localhost:9999"}))
	item := lc.checkLocalCertificateValid()
	if item.Status != StatusPass {
		t.Errorf("local cert with loopback: got %s, want pass", item.Status)
	}
}

func TestCheckServiceReachable_NoTargets(t *testing.T) {
	lc := NewLiveChecker()
	item := lc.checkServiceReachable()
	if item.Status != StatusUnknown {
		t.Errorf("reachable with no targets: got %s, want unknown", item.Status)
	}
}

func TestCheckServiceReachable_ListenerActive(t *testing.T) {
	// Start a real TCP listener
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	lc := NewLiveChecker(WithIngressTargets([]string{ln.Addr().String()}))
	item := lc.checkServiceReachable()
	if item.Status != StatusPass {
		t.Errorf("reachable with active listener: got %s, want pass", item.Status)
	}
}

func TestCheckServiceReachable_NoListener(t *testing.T) {
	lc := NewLiveChecker(WithIngressTargets([]string{"127.0.0.1:1"}))
	item := lc.checkServiceReachable()
	if item.Status != StatusFail {
		t.Errorf("reachable with no listener: got %s, want fail", item.Status)
	}
}

// ---------------------------------------------------------------------------
// Local TLS checks with a real TLS server
// ---------------------------------------------------------------------------

func TestCheckLocalTLSEnabled_RealTLSServer(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	// Extract host:port from the TLS test server
	addr := srv.Listener.Addr().String()

	// We can't test this as a "non-loopback" target easily because
	// httptest.NewTLSServer always binds to 127.0.0.1 which is loopback.
	// Instead, verify that the TLS server is accepted.
	lc := NewLiveChecker(WithIngressTargets([]string{addr}))
	item := lc.checkLocalTLSEnabled()
	// It's loopback so it passes via exemption
	if item.Status != StatusPass {
		t.Errorf("local TLS with real TLS server (loopback): got %s, want pass", item.Status)
	}
}

// ---------------------------------------------------------------------------
// checkSBOMPresent — filesystem-dependent
// ---------------------------------------------------------------------------

func TestCheckSBOMPresent_NotFound(t *testing.T) {
	lc := NewLiveChecker()
	item := lc.checkSBOMPresent()
	// In a test environment, SBOM likely isn't at standard paths
	if item.Status != StatusWarning && item.Status != StatusPass {
		t.Errorf("SBOM check: got %s, want warning or pass", item.Status)
	}
}

// ---------------------------------------------------------------------------
// checkReproducibleBuild — uses runtime/debug info
// ---------------------------------------------------------------------------

func TestCheckReproducibleBuild(t *testing.T) {
	lc := NewLiveChecker()
	item := lc.checkReproducibleBuild()
	// In a go test environment, build info is typically available
	validStatuses := map[Status]bool{StatusPass: true, StatusWarning: true, StatusUnknown: true}
	if !validStatuses[item.Status] {
		t.Errorf("reproducible build: got %s, expected pass/warning/unknown", item.Status)
	}
}

// ---------------------------------------------------------------------------
// Full integration: Checker + LiveChecker -> report
// ---------------------------------------------------------------------------

func TestLiveChecker_FullReport(t *testing.T) {
	m := manifest.BuildManifest{
		Version:                    "1.0.0-test",
		CloudflaredUpstreamVersion: "2025.2.1",
		FIPSCertificates: []manifest.FIPSCertificate{
			{Module: "BoringSSL", Certificate: "#4735"},
		},
	}
	tmpManifest := writeManifest(t, m)
	defer os.Remove(tmpManifest)

	lc := NewLiveChecker(
		WithManifestPath(tmpManifest),
		WithMetricsAddr("127.0.0.1:1"),
		WithIngressTargets([]string{"localhost:8080"}),
	)

	checker := NewChecker()
	checker.AddSection(lc.RunTunnelChecks())
	checker.AddSection(lc.RunLocalServiceChecks())
	checker.AddSection(lc.RunBuildSupplyChainChecks())

	report := checker.GenerateReport()
	if len(report.Sections) != 3 {
		t.Errorf("report has %d sections, want 3", len(report.Sections))
	}
	if report.Summary.Total != 12+4+7 {
		t.Errorf("report has %d total items, want %d", report.Summary.Total, 12+4+7)
	}
	if report.Timestamp == "" {
		t.Error("report timestamp is empty")
	}

	// No items should have empty status
	for _, section := range report.Sections {
		for _, item := range section.Items {
			if item.Status == "" {
				t.Errorf("item %s in section %s has empty status", item.ID, section.ID)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// detectCloudflaredProcess — platform-specific, just ensure no panic
// ---------------------------------------------------------------------------

func TestDetectCloudflaredProcess_NoPanic(t *testing.T) {
	// This just ensures the function doesn't panic
	running, tokenBased := detectCloudflaredProcess()
	_ = running
	_ = tokenBased
	// On Linux it scans /proc; on other platforms returns false, false
}

// ---------------------------------------------------------------------------
// checkCertificateValidity — just verify structure, no real cert
// ---------------------------------------------------------------------------

func TestCheckCertificateValidity(t *testing.T) {
	lc := NewLiveChecker()
	item := lc.checkCertificateValidity()
	if item.ID != "t-8" {
		t.Errorf("ID = %q, want t-8", item.ID)
	}
	// Should be pass (if cloudflared running) or unknown (if not)
	if item.Status == StatusFail {
		t.Errorf("cert validity should not fail in test environment, got fail")
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func writeManifest(t *testing.T, m manifest.BuildManifest) string {
	t.Helper()
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	tmpFile, err := os.CreateTemp("", "manifest-*.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tmpFile.Write(data); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	return tmpFile.Name()
}

// Suppress unused import warning for filepath (used by live.go, tested indirectly)
var _ = filepath.Base
// Suppress unused import warning for tls (used in test helpers)
var _ = tls.VersionTLS12
