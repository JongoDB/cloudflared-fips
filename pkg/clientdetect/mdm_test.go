package clientdetect

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewMDMClient(t *testing.T) {
	c := NewMDMClient(MDMConfig{Provider: MDMProviderIntune})
	if c == nil {
		t.Fatal("NewMDMClient returned nil")
	}
}

func TestMDMClient_IsConfigured_Intune(t *testing.T) {
	tests := []struct {
		name string
		cfg  MDMConfig
		want bool
	}{
		{
			"fully configured",
			MDMConfig{Provider: MDMProviderIntune, TenantID: "t", ClientID: "c", ClientSecret: "s"},
			true,
		},
		{
			"missing tenant",
			MDMConfig{Provider: MDMProviderIntune, ClientID: "c", ClientSecret: "s"},
			false,
		},
		{
			"missing client ID",
			MDMConfig{Provider: MDMProviderIntune, TenantID: "t", ClientSecret: "s"},
			false,
		},
		{
			"missing secret",
			MDMConfig{Provider: MDMProviderIntune, TenantID: "t", ClientID: "c"},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewMDMClient(tt.cfg)
			if got := c.IsConfigured(); got != tt.want {
				t.Errorf("IsConfigured() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMDMClient_IsConfigured_Jamf(t *testing.T) {
	tests := []struct {
		name string
		cfg  MDMConfig
		want bool
	}{
		{
			"fully configured",
			MDMConfig{Provider: MDMProviderJamf, BaseURL: "https://jamf.example.com", APIToken: "tok"},
			true,
		},
		{
			"missing URL",
			MDMConfig{Provider: MDMProviderJamf, APIToken: "tok"},
			false,
		},
		{
			"missing token",
			MDMConfig{Provider: MDMProviderJamf, BaseURL: "https://jamf.example.com"},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewMDMClient(tt.cfg)
			if got := c.IsConfigured(); got != tt.want {
				t.Errorf("IsConfigured() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMDMClient_IsConfigured_None(t *testing.T) {
	c := NewMDMClient(MDMConfig{Provider: MDMProviderNone})
	if c.IsConfigured() {
		t.Error("MDMProviderNone should not be configured")
	}
}

func TestMDMClient_Provider(t *testing.T) {
	c := NewMDMClient(MDMConfig{Provider: MDMProviderIntune})
	if c.Provider() != MDMProviderIntune {
		t.Errorf("Provider() = %q, want intune", c.Provider())
	}
}

func TestMDMClient_FetchDevices_UnconfiguredProvider(t *testing.T) {
	c := NewMDMClient(MDMConfig{Provider: MDMProviderNone})
	_, err := c.FetchDevices()
	if err == nil {
		t.Error("expected error for unconfigured provider")
	}
}

func TestMDMClient_FetchJamfDevices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []map[string]interface{}{
				{
					"id": "jamf-1",
					"general": map[string]interface{}{
						"name":            "MacBook-1",
						"lastContactTime": "2026-02-28T10:00:00Z",
						"mdmCapable":      true,
					},
					"operatingSystem": map[string]interface{}{
						"name":    "macOS",
						"version": "14.3",
					},
					"security": map[string]interface{}{
						"fileVault2Status": "ALL_ENCRYPTED",
					},
				},
				{
					"id": "jamf-2",
					"general": map[string]interface{}{
						"name":            "MacBook-2",
						"lastContactTime": "2026-02-28T09:00:00Z",
						"mdmCapable":      false,
					},
					"operatingSystem": map[string]interface{}{
						"name":    "macOS",
						"version": "13.6",
					},
					"security": map[string]interface{}{
						"fileVault2Status": "NOT_ENCRYPTED",
					},
				},
			},
		})
	}))
	defer server.Close()

	c := NewMDMClient(MDMConfig{
		Provider: MDMProviderJamf,
		BaseURL:  server.URL,
		APIToken: "test-token",
	})

	devices, err := c.FetchDevices()
	if err != nil {
		t.Fatalf("FetchDevices: %v", err)
	}

	if len(devices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(devices))
	}

	// First device: encrypted, MDM capable
	if devices[0].DeviceName != "MacBook-1" {
		t.Errorf("device 0 name = %q, want MacBook-1", devices[0].DeviceName)
	}
	if !devices[0].Encrypted {
		t.Error("device 0 should be encrypted (ALL_ENCRYPTED)")
	}
	if !devices[0].MDMEnrolled {
		t.Error("device 0 should be MDM enrolled")
	}
	if !devices[0].Compliant {
		t.Error("device 0 should be compliant (mdmCapable && encrypted)")
	}
	if devices[0].Provider != "jamf" {
		t.Errorf("device 0 provider = %q, want jamf", devices[0].Provider)
	}

	// Second device: not encrypted, not MDM capable
	if devices[1].Encrypted {
		t.Error("device 1 should not be encrypted")
	}
	if devices[1].Compliant {
		t.Error("device 1 should not be compliant")
	}
}

func TestMDMClient_FetchDevices_Caching(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		// Return at least one device so the cache stores a non-nil slice
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []map[string]interface{}{
				{
					"id":              "cache-test-1",
					"general":         map[string]interface{}{"name": "CachedDevice", "mdmCapable": true},
					"operatingSystem": map[string]interface{}{"name": "macOS"},
					"security":        map[string]interface{}{"fileVault2Status": "ALL_ENCRYPTED"},
				},
			},
		})
	}))
	defer server.Close()

	c := NewMDMClient(MDMConfig{
		Provider: MDMProviderJamf,
		BaseURL:  server.URL,
		APIToken: "tok",
	})

	// First fetch hits the server
	_, _ = c.FetchDevices()
	if callCount != 1 {
		t.Errorf("first fetch: call count = %d, want 1", callCount)
	}

	// Second fetch should use cache
	_, _ = c.FetchDevices()
	if callCount != 1 {
		t.Errorf("second fetch: call count = %d, want 1 (cached)", callCount)
	}
}

func TestMDMClient_ComplianceSummary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []map[string]interface{}{
				{
					"id":              "1",
					"general":         map[string]interface{}{"name": "D1", "mdmCapable": true},
					"operatingSystem": map[string]interface{}{"name": "macOS"},
					"security":        map[string]interface{}{"fileVault2Status": "ALL_ENCRYPTED"},
				},
				{
					"id":              "2",
					"general":         map[string]interface{}{"name": "D2", "mdmCapable": true},
					"operatingSystem": map[string]interface{}{"name": "macOS"},
					"security":        map[string]interface{}{"fileVault2Status": "NOT_ENCRYPTED"},
				},
			},
		})
	}))
	defer server.Close()

	c := NewMDMClient(MDMConfig{
		Provider: MDMProviderJamf,
		BaseURL:  server.URL,
		APIToken: "tok",
	})

	summary := c.ComplianceSummary()
	if summary.TotalDevices != 2 {
		t.Errorf("TotalDevices = %d, want 2", summary.TotalDevices)
	}
	if summary.Enrolled != 2 {
		t.Errorf("Enrolled = %d, want 2", summary.Enrolled)
	}
	if summary.Encrypted != 1 {
		t.Errorf("Encrypted = %d, want 1", summary.Encrypted)
	}
	if summary.Compliant != 1 {
		t.Errorf("Compliant = %d, want 1", summary.Compliant)
	}
	if summary.Provider != "jamf" {
		t.Errorf("Provider = %q, want jamf", summary.Provider)
	}
}

func TestMDMClient_ComplianceSummary_Unconfigured(t *testing.T) {
	c := NewMDMClient(MDMConfig{Provider: MDMProviderNone})
	summary := c.ComplianceSummary()
	if summary.Error == "" {
		t.Error("expected error in summary for unconfigured provider")
	}
}
