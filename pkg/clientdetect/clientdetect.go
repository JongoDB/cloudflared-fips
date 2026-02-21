// Package clientdetect provides TLS ClientHello inspection and JA4
// fingerprinting to detect whether connecting clients use FIPS-capable TLS.
//
// It integrates with Go's tls.Config.GetConfigForClient callback to passively
// inspect the cipher suites and extensions offered by clients.
package clientdetect

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// ClientInfo holds the FIPS detection result for a single client connection.
type ClientInfo struct {
	RemoteAddr    string    `json:"remote_addr"`
	ServerName    string    `json:"server_name"`
	TLSVersion    uint16    `json:"tls_version"`
	CipherSuites  []uint16  `json:"cipher_suites"`
	JA4Hash       string    `json:"ja4_hash"`
	FIPSCapable   bool      `json:"fips_capable"`
	FIPSReason    string    `json:"fips_reason"`
	DetectedAt    time.Time `json:"detected_at"`
}

// Inspector collects client TLS fingerprints and maintains a connection log.
type Inspector struct {
	mu      sync.RWMutex
	clients []ClientInfo
	maxLog  int
}

// NewInspector creates a client TLS inspector with a maximum log size.
func NewInspector(maxLog int) *Inspector {
	if maxLog <= 0 {
		maxLog = 1000
	}
	return &Inspector{
		maxLog: maxLog,
	}
}

// GetConfigForClient returns a tls.Config callback that inspects ClientHello
// messages. Install this on the dashboard TLS server to passively detect
// client FIPS capability.
//
//	tlsConfig := &tls.Config{
//	    GetConfigForClient: inspector.GetConfigForClient,
//	}
func (ins *Inspector) GetConfigForClient(hello *tls.ClientHelloInfo) (*tls.Config, error) {
	info := ClientInfo{
		RemoteAddr:   hello.Conn.RemoteAddr().String(),
		ServerName:   hello.ServerName,
		CipherSuites: hello.CipherSuites,
		DetectedAt:   time.Now(),
	}

	// Determine FIPS capability from offered cipher suites
	info.FIPSCapable, info.FIPSReason = analyzeFIPSCapability(hello.CipherSuites)

	// Compute JA4-style fingerprint
	info.JA4Hash = computeJA4(hello)

	// Detect TLS version from supported versions
	if len(hello.SupportedVersions) > 0 {
		info.TLSVersion = hello.SupportedVersions[0]
	}

	ins.mu.Lock()
	ins.clients = append(ins.clients, info)
	// Trim to max log size
	if len(ins.clients) > ins.maxLog {
		ins.clients = ins.clients[len(ins.clients)-ins.maxLog:]
	}
	ins.mu.Unlock()

	// Return nil to use the default TLS config (don't override)
	return nil, nil
}

// RecentClients returns the most recent n client connection records.
func (ins *Inspector) RecentClients(n int) []ClientInfo {
	ins.mu.RLock()
	defer ins.mu.RUnlock()

	if n <= 0 || n > len(ins.clients) {
		n = len(ins.clients)
	}
	start := len(ins.clients) - n
	result := make([]ClientInfo, n)
	copy(result, ins.clients[start:])
	return result
}

// FIPSStats returns aggregate FIPS capability statistics.
func (ins *Inspector) FIPSStats() FIPSSummary {
	ins.mu.RLock()
	defer ins.mu.RUnlock()

	summary := FIPSSummary{}
	for _, c := range ins.clients {
		summary.Total++
		if c.FIPSCapable {
			summary.FIPSCapable++
		} else {
			summary.NonFIPS++
		}
	}
	return summary
}

// FIPSSummary holds aggregate client FIPS statistics.
type FIPSSummary struct {
	Total       int `json:"total"`
	FIPSCapable int `json:"fips_capable"`
	NonFIPS     int `json:"non_fips"`
}

// analyzeFIPSCapability determines if a client's cipher suite offering is
// consistent with FIPS mode. Key heuristics:
//   - Absence of ChaCha20-Poly1305 strongly suggests FIPS (it's non-FIPS)
//   - Presence of only AES-GCM/AES-CBC suites with ECDHE/RSA is FIPS
//   - Presence of RC4, DES, 3DES, EXPORT ciphers is NOT FIPS
func analyzeFIPSCapability(suites []uint16) (bool, string) {
	hasChaCha := false
	hasRC4orDES := false
	hasAESGCM := false

	for _, id := range suites {
		name := tls.CipherSuiteName(id)
		upper := strings.ToUpper(name)

		if strings.Contains(upper, "CHACHA20") {
			hasChaCha = true
		}
		if strings.Contains(upper, "RC4") || strings.Contains(upper, "3DES") || strings.Contains(upper, "DES_CBC") {
			hasRC4orDES = true
		}
		if strings.Contains(upper, "AES") && strings.Contains(upper, "GCM") {
			hasAESGCM = true
		}
	}

	if hasRC4orDES {
		return false, "Client offers banned ciphers (RC4/DES/3DES)"
	}
	if !hasChaCha && hasAESGCM {
		return true, "No ChaCha20, AES-GCM present — consistent with FIPS mode"
	}
	if hasChaCha {
		return false, "Client offers ChaCha20-Poly1305 (non-FIPS cipher)"
	}
	if !hasAESGCM {
		return false, "Client does not offer AES-GCM ciphers"
	}
	return false, "Indeterminate"
}

// computeJA4 produces a JA4-style fingerprint from a ClientHello.
// JA4 = TLSVersion_CipherCount_ExtCount_ALPNFirst_SortedCipherHash
// This is a simplified approximation (full JA4 requires raw ClientHello bytes).
func computeJA4(hello *tls.ClientHelloInfo) string {
	// TLS version (highest offered)
	version := "00"
	if len(hello.SupportedVersions) > 0 {
		switch hello.SupportedVersions[0] {
		case tls.VersionTLS13:
			version = "13"
		case tls.VersionTLS12:
			version = "12"
		case tls.VersionTLS11:
			version = "11"
		case tls.VersionTLS10:
			version = "10"
		}
	}

	// Sort cipher suites for deterministic hash
	sorted := make([]uint16, len(hello.CipherSuites))
	copy(sorted, hello.CipherSuites)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	// Build cipher string
	var cipherStrs []string
	for _, id := range sorted {
		cipherStrs = append(cipherStrs, fmt.Sprintf("%04x", id))
	}
	cipherConcat := strings.Join(cipherStrs, ",")

	// ALPN
	alpn := "00"
	if len(hello.SupportedProtos) > 0 {
		alpn = hello.SupportedProtos[0]
		if len(alpn) > 2 {
			alpn = alpn[:2]
		}
	}

	// Hash
	h := sha256.Sum256([]byte(cipherConcat))
	hashStr := hex.EncodeToString(h[:6]) // 12 hex chars

	return fmt.Sprintf("t%s_%02d_%s_%s", version, len(hello.CipherSuites), alpn, hashStr)
}

// KnownFIPSFingerprints maps JA4 hash prefixes to known FIPS-capable client
// descriptions. The key format is "tVV_CC_" where VV is TLS version and CC
// is cipher count. Full hash matching is used when available; prefix matching
// for pattern detection.
//
// These fingerprints are derived from observed ClientHello behavior of
// FIPS-mode operating systems and browsers. Actual hashes vary by OS patch
// level and browser version — this map provides baseline patterns.
var KnownFIPSFingerprints = map[string]string{
	// Windows 10/11 with FIPS policy enabled (Schannel/CNG)
	// GPO: "System cryptography: Use FIPS compliant algorithms"
	// Offers: TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	//         TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	//         TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	//         TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
	// No ChaCha20, no CBC (modern Windows FIPS), 4 suites typical
	"t13_04_h2": "Windows FIPS mode (Edge/Chrome via Schannel CNG)",
	"t12_04_h2": "Windows FIPS mode (TLS 1.2 only, Edge/Chrome via Schannel CNG)",

	// Windows with IE/Edge Legacy — slightly more suites including CBC
	"t12_06_h2": "Windows FIPS mode (legacy Edge, AES-GCM + AES-CBC suites)",
	"t13_06_h2": "Windows FIPS mode (legacy Edge with TLS 1.3, mixed suites)",

	// RHEL 8/9 Firefox with NSS FIPS mode
	// Firefox in FIPS mode uses NSS (Network Security Services) with
	// FIPS-approved suites only. Typically 6-8 suites, no ChaCha20.
	"t13_08_h2": "RHEL Firefox (NSS FIPS mode, TLS 1.3)",
	"t12_08_h2": "RHEL Firefox (NSS FIPS mode, TLS 1.2)",

	// RHEL curl/OpenSSL FIPS
	// OpenSSL in FIPS mode offers ~6 ECDHE+AES suites
	"t13_06_00": "RHEL curl/OpenSSL FIPS (no ALPN)",
	"t12_06_00": "RHEL curl/OpenSSL FIPS (TLS 1.2, no ALPN)",

	// macOS Safari (CommonCrypto/Secure Transport)
	// macOS always uses validated CommonCrypto but does include ChaCha20
	// in the offering — so macOS Safari is NOT flagged as strict FIPS.
	// However, macOS with MDM profiles restricting to FIPS can reduce suites.
	"t13_04_h2": "macOS Safari (restricted profile, FIPS-approved suites only)",

	// Go HTTP client with FIPS build
	// GOEXPERIMENT=boringcrypto restricts to ~8 suites, all AES-GCM/CBC
	"t13_08_h2": "Go client (BoringCrypto FIPS build)",

	// Cloudflare WARP with FIPS enforcement
	"t13_05_h2": "Cloudflare WARP (FIPS enforcement enabled)",
}

// MatchKnownFIPSClient checks if a JA4 hash matches a known FIPS fingerprint.
// Returns the description and true if matched, empty string and false otherwise.
func MatchKnownFIPSClient(ja4Hash string) (string, bool) {
	// Exact match first
	if desc, ok := KnownFIPSFingerprints[ja4Hash]; ok {
		return desc, true
	}
	// Prefix match (version + cipher count + ALPN)
	// JA4 format: tVV_CC_ALPN_HASH — try matching without the hash suffix
	parts := strings.SplitN(ja4Hash, "_", 4)
	if len(parts) >= 3 {
		prefix := parts[0] + "_" + parts[1] + "_" + parts[2]
		if desc, ok := KnownFIPSFingerprints[prefix]; ok {
			return desc, true
		}
	}
	return "", false
}
