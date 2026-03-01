package cfapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cloudflared-fips/cloudflared-fips/internal/compliance"
)

// mockCFAPIRoutes creates a test server that routes by path and returns Cloudflare API-format responses.
func mockCFAPIRoutes(t *testing.T, handlers map[string]interface{}) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth header
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"errors":  []map[string]interface{}{{"code": 9109, "message": "Invalid access token"}},
			})
			return
		}

		handler, ok := handlers[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"errors":  []map[string]interface{}{{"code": 7003, "message": "Not found"}},
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"errors":  []interface{}{},
			"result":  handler,
		})
	}))
}

func newTestChecker(t *testing.T, handlers map[string]interface{}) (*ComplianceChecker, func()) {
	t.Helper()
	server := mockCFAPIRoutes(t, handlers)
	client := NewClient("test-token", WithBaseURL(server.URL), WithCacheTTL(0))
	cc := NewComplianceChecker(client, "zone-123", "acct-456", "tunnel-789")
	return cc, server.Close
}

// ---------------------------------------------------------------------------
// RunEdgeChecks — structure
// ---------------------------------------------------------------------------

func TestRunEdgeChecks_Structure(t *testing.T) {
	cc, cleanup := newTestChecker(t, map[string]interface{}{})
	defer cleanup()

	section := cc.RunEdgeChecks()
	if section.ID != "edge" {
		t.Errorf("section ID = %q, want edge", section.ID)
	}
	if section.Name != "Cloudflare Edge" {
		t.Errorf("section Name = %q, want Cloudflare Edge", section.Name)
	}
	if len(section.Items) != 11 {
		t.Errorf("expected 11 items, got %d", len(section.Items))
	}

	expectedIDs := []string{"ce-1", "ce-2", "ce-3", "ce-4", "ce-5", "ce-6", "ce-7", "ce-8", "ce-9", "ce-10", "ce-11"}
	for i, id := range expectedIDs {
		if i < len(section.Items) && section.Items[i].ID != id {
			t.Errorf("item %d: ID = %q, want %q", i, section.Items[i].ID, id)
		}
	}
}

// ---------------------------------------------------------------------------
// checkAccessPolicy
// ---------------------------------------------------------------------------

func TestCheckAccessPolicy_AppsExist(t *testing.T) {
	cc, cleanup := newTestChecker(t, map[string]interface{}{
		"/zones/zone-123/access/apps": []map[string]string{
			{"id": "app-1", "name": "Internal App"},
			{"id": "app-2", "name": "Dashboard"},
		},
	})
	defer cleanup()

	item := cc.checkAccessPolicy()
	if item.Status != compliance.StatusPass {
		t.Errorf("status = %q, want pass", item.Status)
	}
	if item.ID != "ce-1" {
		t.Errorf("ID = %q, want ce-1", item.ID)
	}
}

func TestCheckAccessPolicy_NoApps(t *testing.T) {
	cc, cleanup := newTestChecker(t, map[string]interface{}{
		"/zones/zone-123/access/apps": []interface{}{},
	})
	defer cleanup()

	item := cc.checkAccessPolicy()
	if item.Status != compliance.StatusFail {
		t.Errorf("status = %q, want fail (no apps)", item.Status)
	}
}

func TestCheckAccessPolicy_APIError(t *testing.T) {
	cc, cleanup := newTestChecker(t, map[string]interface{}{})
	defer cleanup()

	item := cc.checkAccessPolicy()
	if item.Status != compliance.StatusUnknown {
		t.Errorf("status = %q, want unknown (API error)", item.Status)
	}
}

// ---------------------------------------------------------------------------
// checkIdentityProvider
// ---------------------------------------------------------------------------

func TestCheckIdentityProvider_AppsExist(t *testing.T) {
	cc, cleanup := newTestChecker(t, map[string]interface{}{
		"/zones/zone-123/access/apps": []map[string]string{{"id": "app-1"}},
	})
	defer cleanup()

	item := cc.checkIdentityProvider()
	if item.Status != compliance.StatusPass {
		t.Errorf("status = %q, want pass", item.Status)
	}
}

func TestCheckIdentityProvider_NoApps(t *testing.T) {
	cc, cleanup := newTestChecker(t, map[string]interface{}{
		"/zones/zone-123/access/apps": []interface{}{},
	})
	defer cleanup()

	item := cc.checkIdentityProvider()
	if item.Status != compliance.StatusUnknown {
		t.Errorf("status = %q, want unknown (no apps)", item.Status)
	}
}

// ---------------------------------------------------------------------------
// checkAuthMethod / checkMFAEnforced (static items)
// ---------------------------------------------------------------------------

func TestCheckAuthMethod_AlwaysUnknown(t *testing.T) {
	cc, cleanup := newTestChecker(t, map[string]interface{}{})
	defer cleanup()

	item := cc.checkAuthMethod()
	if item.Status != compliance.StatusUnknown {
		t.Errorf("status = %q, want unknown", item.Status)
	}
	if item.ID != "ce-3" {
		t.Errorf("ID = %q, want ce-3", item.ID)
	}
}

func TestCheckMFAEnforced_AlwaysUnknown(t *testing.T) {
	cc, cleanup := newTestChecker(t, map[string]interface{}{})
	defer cleanup()

	item := cc.checkMFAEnforced()
	if item.Status != compliance.StatusUnknown {
		t.Errorf("status = %q, want unknown", item.Status)
	}
	if item.ID != "ce-4" {
		t.Errorf("ID = %q, want ce-4", item.ID)
	}
}

// ---------------------------------------------------------------------------
// checkCipherRestriction
// ---------------------------------------------------------------------------

func TestCheckCipherRestriction_AllFIPS(t *testing.T) {
	cc, cleanup := newTestChecker(t, map[string]interface{}{
		"/zones/zone-123/settings/ciphers": map[string]interface{}{
			"id":    "ciphers",
			"value": []string{"ECDHE-RSA-AES128-GCM-SHA256", "ECDHE-RSA-AES256-GCM-SHA384"},
		},
	})
	defer cleanup()

	item := cc.checkCipherRestriction()
	if item.Status != compliance.StatusPass {
		t.Errorf("status = %q, want pass (all FIPS ciphers)", item.Status)
	}
}

func TestCheckCipherRestriction_SomeNonFIPS(t *testing.T) {
	cc, cleanup := newTestChecker(t, map[string]interface{}{
		"/zones/zone-123/settings/ciphers": map[string]interface{}{
			"id":    "ciphers",
			"value": []string{"ECDHE-RSA-AES128-GCM-SHA256", "ECDHE-RSA-CHACHA20-POLY1305"},
		},
	})
	defer cleanup()

	item := cc.checkCipherRestriction()
	if item.Status != compliance.StatusWarning {
		t.Errorf("status = %q, want warning (non-FIPS cipher)", item.Status)
	}
}

func TestCheckCipherRestriction_NoCiphersConfigured(t *testing.T) {
	cc, cleanup := newTestChecker(t, map[string]interface{}{
		"/zones/zone-123/settings/ciphers": map[string]interface{}{
			"id":    "ciphers",
			"value": []string{},
		},
	})
	defer cleanup()

	item := cc.checkCipherRestriction()
	if item.Status != compliance.StatusWarning {
		t.Errorf("status = %q, want warning (no custom ciphers)", item.Status)
	}
}

// ---------------------------------------------------------------------------
// checkMinTLSVersion
// ---------------------------------------------------------------------------

func TestCheckMinTLSVersion_12(t *testing.T) {
	cc, cleanup := newTestChecker(t, map[string]interface{}{
		"/zones/zone-123/settings/min_tls_version": map[string]interface{}{
			"id":    "min_tls_version",
			"value": "1.2",
		},
	})
	defer cleanup()

	item := cc.checkMinTLSVersion()
	if item.Status != compliance.StatusPass {
		t.Errorf("status = %q, want pass (TLS 1.2)", item.Status)
	}
}

func TestCheckMinTLSVersion_13(t *testing.T) {
	cc, cleanup := newTestChecker(t, map[string]interface{}{
		"/zones/zone-123/settings/min_tls_version": map[string]interface{}{
			"id":    "min_tls_version",
			"value": "1.3",
		},
	})
	defer cleanup()

	item := cc.checkMinTLSVersion()
	if item.Status != compliance.StatusPass {
		t.Errorf("status = %q, want pass (TLS 1.3)", item.Status)
	}
}

func TestCheckMinTLSVersion_10(t *testing.T) {
	cc, cleanup := newTestChecker(t, map[string]interface{}{
		"/zones/zone-123/settings/min_tls_version": map[string]interface{}{
			"id":    "min_tls_version",
			"value": "1.0",
		},
	})
	defer cleanup()

	item := cc.checkMinTLSVersion()
	if item.Status != compliance.StatusFail {
		t.Errorf("status = %q, want fail (TLS 1.0)", item.Status)
	}
}

func TestCheckMinTLSVersion_11(t *testing.T) {
	cc, cleanup := newTestChecker(t, map[string]interface{}{
		"/zones/zone-123/settings/min_tls_version": map[string]interface{}{
			"id":    "min_tls_version",
			"value": "1.1",
		},
	})
	defer cleanup()

	item := cc.checkMinTLSVersion()
	if item.Status != compliance.StatusFail {
		t.Errorf("status = %q, want fail (TLS 1.1)", item.Status)
	}
}

// ---------------------------------------------------------------------------
// checkEdgeCertificate
// ---------------------------------------------------------------------------

func TestCheckEdgeCertificate_ActiveValid(t *testing.T) {
	future := time.Now().Add(365 * 24 * time.Hour).Format(time.RFC3339)
	cc, cleanup := newTestChecker(t, map[string]interface{}{
		"/zones/zone-123/ssl/certificate_packs": []map[string]interface{}{
			{"id": "cert-1", "type": "universal", "status": "active", "expires_on": future},
		},
	})
	defer cleanup()

	item := cc.checkEdgeCertificate()
	if item.Status != compliance.StatusPass {
		t.Errorf("status = %q, want pass", item.Status)
	}
}

func TestCheckEdgeCertificate_ExpiringSoon(t *testing.T) {
	soon := time.Now().Add(15 * 24 * time.Hour).Format(time.RFC3339)
	cc, cleanup := newTestChecker(t, map[string]interface{}{
		"/zones/zone-123/ssl/certificate_packs": []map[string]interface{}{
			{"id": "cert-1", "type": "universal", "status": "active", "expires_on": soon},
		},
	})
	defer cleanup()

	item := cc.checkEdgeCertificate()
	if item.Status != compliance.StatusWarning {
		t.Errorf("status = %q, want warning (expires < 30 days)", item.Status)
	}
}

func TestCheckEdgeCertificate_Expired(t *testing.T) {
	past := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	cc, cleanup := newTestChecker(t, map[string]interface{}{
		"/zones/zone-123/ssl/certificate_packs": []map[string]interface{}{
			{"id": "cert-1", "type": "universal", "status": "active", "expires_on": past},
		},
	})
	defer cleanup()

	item := cc.checkEdgeCertificate()
	if item.Status != compliance.StatusFail {
		t.Errorf("status = %q, want fail (expired)", item.Status)
	}
}

func TestCheckEdgeCertificate_NoPacks(t *testing.T) {
	cc, cleanup := newTestChecker(t, map[string]interface{}{
		"/zones/zone-123/ssl/certificate_packs": []interface{}{},
	})
	defer cleanup()

	item := cc.checkEdgeCertificate()
	if item.Status != compliance.StatusFail {
		t.Errorf("status = %q, want fail (no packs)", item.Status)
	}
}

// ---------------------------------------------------------------------------
// checkHSTS
// ---------------------------------------------------------------------------

func TestCheckHSTS_Enabled(t *testing.T) {
	cc, cleanup := newTestChecker(t, map[string]interface{}{
		"/zones/zone-123/settings/security_header": map[string]interface{}{
			"id": "security_header",
			"value": map[string]interface{}{
				"strict_transport_security": map[string]interface{}{
					"enabled":            true,
					"max_age":            31536000,
					"include_subdomains": true,
					"preload":            true,
				},
			},
		},
	})
	defer cleanup()

	item := cc.checkHSTS()
	if item.Status != compliance.StatusPass {
		t.Errorf("status = %q, want pass", item.Status)
	}
}

func TestCheckHSTS_Disabled(t *testing.T) {
	cc, cleanup := newTestChecker(t, map[string]interface{}{
		"/zones/zone-123/settings/security_header": map[string]interface{}{
			"id": "security_header",
			"value": map[string]interface{}{
				"strict_transport_security": map[string]interface{}{
					"enabled": false,
				},
			},
		},
	})
	defer cleanup()

	item := cc.checkHSTS()
	if item.Status != compliance.StatusFail {
		t.Errorf("status = %q, want fail (HSTS disabled)", item.Status)
	}
}

// ---------------------------------------------------------------------------
// checkTunnelHealth
// ---------------------------------------------------------------------------

func TestCheckTunnelHealth_Healthy(t *testing.T) {
	cc, cleanup := newTestChecker(t, map[string]interface{}{
		"/accounts/acct-456/cfd_tunnel/tunnel-789": map[string]interface{}{
			"id":     "tunnel-789",
			"status": "healthy",
			"connections": []map[string]interface{}{
				{"colo_name": "DFW", "is_active": true},
				{"colo_name": "ORD", "is_active": true},
			},
		},
	})
	defer cleanup()

	item := cc.checkTunnelHealth()
	if item.Status != compliance.StatusPass {
		t.Errorf("status = %q, want pass", item.Status)
	}
}

func TestCheckTunnelHealth_Degraded(t *testing.T) {
	cc, cleanup := newTestChecker(t, map[string]interface{}{
		"/accounts/acct-456/cfd_tunnel/tunnel-789": map[string]interface{}{
			"id":     "tunnel-789",
			"status": "degraded",
		},
	})
	defer cleanup()

	item := cc.checkTunnelHealth()
	if item.Status != compliance.StatusWarning {
		t.Errorf("status = %q, want warning (degraded)", item.Status)
	}
}

func TestCheckTunnelHealth_Down(t *testing.T) {
	cc, cleanup := newTestChecker(t, map[string]interface{}{
		"/accounts/acct-456/cfd_tunnel/tunnel-789": map[string]interface{}{
			"id":     "tunnel-789",
			"status": "down",
		},
	})
	defer cleanup()

	item := cc.checkTunnelHealth()
	if item.Status != compliance.StatusFail {
		t.Errorf("status = %q, want fail (down)", item.Status)
	}
}

func TestCheckTunnelHealth_NoIDs(t *testing.T) {
	client := NewClient("test-token")
	cc := NewComplianceChecker(client, "zone-123", "", "")

	item := cc.checkTunnelHealth()
	if item.Status != compliance.StatusUnknown {
		t.Errorf("status = %q, want unknown (no tunnel ID)", item.Status)
	}
}

// ---------------------------------------------------------------------------
// checkKeylessSSL
// ---------------------------------------------------------------------------

func TestCheckKeylessSSL_Active(t *testing.T) {
	cc, cleanup := newTestChecker(t, map[string]interface{}{
		"/zones/zone-123/keyless_certificates": []map[string]interface{}{
			{"id": "ks-1", "name": "HSM-1", "status": "active", "enabled": true},
		},
	})
	defer cleanup()

	item := cc.checkKeylessSSL()
	if item.Status != compliance.StatusPass {
		t.Errorf("status = %q, want pass", item.Status)
	}
}

func TestCheckKeylessSSL_ExistsButInactive(t *testing.T) {
	cc, cleanup := newTestChecker(t, map[string]interface{}{
		"/zones/zone-123/keyless_certificates": []map[string]interface{}{
			{"id": "ks-1", "name": "HSM-1", "status": "active", "enabled": false},
		},
	})
	defer cleanup()

	item := cc.checkKeylessSSL()
	if item.Status != compliance.StatusWarning {
		t.Errorf("status = %q, want warning (exists but not enabled)", item.Status)
	}
}

func TestCheckKeylessSSL_NotConfigured(t *testing.T) {
	cc, cleanup := newTestChecker(t, map[string]interface{}{
		"/zones/zone-123/keyless_certificates": []interface{}{},
	})
	defer cleanup()

	item := cc.checkKeylessSSL()
	if item.Status != compliance.StatusWarning {
		t.Errorf("status = %q, want warning (not configured)", item.Status)
	}
}

// ---------------------------------------------------------------------------
// checkRegionalServices
// ---------------------------------------------------------------------------

func TestCheckRegionalServices_Enabled(t *testing.T) {
	cc, cleanup := newTestChecker(t, map[string]interface{}{
		"/zones/zone-123/cache/regional_tiered_cache": map[string]interface{}{
			"id":    "regional_tiered_cache",
			"value": "on",
		},
	})
	defer cleanup()

	item := cc.checkRegionalServices()
	if item.Status != compliance.StatusPass {
		t.Errorf("status = %q, want pass", item.Status)
	}
}

func TestCheckRegionalServices_Disabled(t *testing.T) {
	cc, cleanup := newTestChecker(t, map[string]interface{}{
		"/zones/zone-123/cache/regional_tiered_cache": map[string]interface{}{
			"id":    "regional_tiered_cache",
			"value": "off",
		},
	})
	defer cleanup()

	item := cc.checkRegionalServices()
	if item.Status != compliance.StatusWarning {
		t.Errorf("status = %q, want warning (disabled)", item.Status)
	}
}

// ---------------------------------------------------------------------------
// isFIPSCipherName
// ---------------------------------------------------------------------------

func TestIsFIPSCipherName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"ECDHE-RSA-AES128-GCM-SHA256", true},
		{"ECDHE-RSA-AES256-GCM-SHA384", true},
		{"ECDHE-ECDSA-AES128-GCM-SHA256", true},
		{"ECDHE-ECDSA-AES256-GCM-SHA384", true},
		{"AES128-GCM-SHA256", true},
		{"AES256-GCM-SHA384", true},
		{"TLS_AES_128_GCM_SHA256", true},
		{"TLS_AES_256_GCM_SHA384", true},
		// Case insensitive
		{"ecdhe-rsa-aes128-gcm-sha256", true},
		// Non-FIPS
		{"ECDHE-RSA-CHACHA20-POLY1305", false},
		{"RC4-SHA", false},
		{"DES-CBC3-SHA", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isFIPSCipherName(tt.name)
			if got != tt.want {
				t.Errorf("isFIPSCipherName(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Full integration: RunEdgeChecks with fully mocked API
// ---------------------------------------------------------------------------

func TestRunEdgeChecks_FullyCompliant(t *testing.T) {
	future := time.Now().Add(365 * 24 * time.Hour).Format(time.RFC3339)

	cc, cleanup := newTestChecker(t, map[string]interface{}{
		"/zones/zone-123/access/apps": []map[string]string{
			{"id": "app-1", "name": "App"},
		},
		"/zones/zone-123/settings/ciphers": map[string]interface{}{
			"id":    "ciphers",
			"value": []string{"ECDHE-RSA-AES128-GCM-SHA256"},
		},
		"/zones/zone-123/settings/min_tls_version": map[string]interface{}{
			"id":    "min_tls_version",
			"value": "1.2",
		},
		"/zones/zone-123/ssl/certificate_packs": []map[string]interface{}{
			{"id": "cert-1", "type": "universal", "status": "active", "expires_on": future},
		},
		"/zones/zone-123/settings/security_header": map[string]interface{}{
			"id": "security_header",
			"value": map[string]interface{}{
				"strict_transport_security": map[string]interface{}{
					"enabled": true, "max_age": 31536000,
				},
			},
		},
		"/accounts/acct-456/cfd_tunnel/tunnel-789": map[string]interface{}{
			"id": "tunnel-789", "status": "healthy",
			"connections": []map[string]interface{}{{"is_active": true}},
		},
		"/zones/zone-123/keyless_certificates": []map[string]interface{}{
			{"id": "ks-1", "status": "active", "enabled": true},
		},
		"/zones/zone-123/cache/regional_tiered_cache": map[string]interface{}{
			"id": "regional_tiered_cache", "value": "on",
		},
	})
	defer cleanup()

	section := cc.RunEdgeChecks()

	// Count statuses
	var pass, fail, warn, unknown int
	for _, item := range section.Items {
		switch item.Status {
		case compliance.StatusPass:
			pass++
		case compliance.StatusFail:
			fail++
		case compliance.StatusWarning:
			warn++
		case compliance.StatusUnknown:
			unknown++
		}
	}

	if fail > 0 {
		for _, item := range section.Items {
			if item.Status == compliance.StatusFail {
				t.Errorf("unexpected fail: %s (%s): %s", item.ID, item.Name, item.What)
			}
		}
	}

	// ce-3 (auth method) and ce-4 (MFA) are always unknown
	if unknown != 2 {
		t.Errorf("unknown count = %d, want 2 (ce-3, ce-4)", unknown)
		for _, item := range section.Items {
			if item.Status == compliance.StatusUnknown {
				fmt.Printf("  unknown: %s %s\n", item.ID, item.Name)
			}
		}
	}
}
