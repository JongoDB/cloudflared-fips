package cfapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// mockCFAPI creates a test server that returns Cloudflare API-style responses.
func mockCFAPI(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(handler)
}

func cfResponse(result interface{}) []byte {
	resultJSON, _ := json.Marshal(result)
	resp := map[string]interface{}{
		"success":  true,
		"errors":   []interface{}{},
		"messages": []interface{}{},
		"result":   json.RawMessage(resultJSON),
	}
	data, _ := json.Marshal(resp)
	return data
}

func cfErrorResponse(code int, msg string) []byte {
	resp := map[string]interface{}{
		"success": false,
		"errors": []map[string]interface{}{
			{"code": code, "message": msg},
		},
		"messages": []interface{}{},
		"result":   nil,
	}
	data, _ := json.Marshal(resp)
	return data
}

func TestNewClient(t *testing.T) {
	c := NewClient("test-token")
	if c.token != "test-token" {
		t.Errorf("expected token test-token, got %q", c.token)
	}
	if c.baseURL != "https://api.cloudflare.com/client/v4" {
		t.Errorf("unexpected baseURL: %q", c.baseURL)
	}
	if c.ttl != 60*time.Second {
		t.Errorf("expected 60s TTL, got %v", c.ttl)
	}
}

func TestNewClientOptions(t *testing.T) {
	c := NewClient("tok",
		WithBaseURL("https://test.example.com"),
		WithCacheTTL(5*time.Second),
	)
	if c.baseURL != "https://test.example.com" {
		t.Errorf("expected custom baseURL, got %q", c.baseURL)
	}
	if c.ttl != 5*time.Second {
		t.Errorf("expected 5s TTL, got %v", c.ttl)
	}
}

func TestClearCache(t *testing.T) {
	c := NewClient("tok")
	c.cache["test"] = cacheEntry{data: []byte(`"cached"`), expiresAt: time.Now().Add(time.Hour)}
	if len(c.cache) != 1 {
		t.Fatal("expected 1 cache entry")
	}
	c.ClearCache()
	if len(c.cache) != 0 {
		t.Error("expected empty cache after ClearCache")
	}
}

func TestGetBearerToken(t *testing.T) {
	var gotAuth string
	srv := mockCFAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Write(cfResponse("ok"))
	})
	defer srv.Close()

	c := NewClient("my-secret-token", WithBaseURL(srv.URL))
	c.get("/test")

	if gotAuth != "Bearer my-secret-token" {
		t.Errorf("expected Bearer token header, got %q", gotAuth)
	}
}

func TestGetCaching(t *testing.T) {
	callCount := 0
	srv := mockCFAPI(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Write(cfResponse("data"))
	})
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL), WithCacheTTL(time.Hour))

	// First call hits server
	_, err := c.get("/zones/z1/settings/test")
	if err != nil {
		t.Fatal(err)
	}
	if callCount != 1 {
		t.Fatalf("expected 1 call, got %d", callCount)
	}

	// Second call hits cache
	_, err = c.get("/zones/z1/settings/test")
	if err != nil {
		t.Fatal(err)
	}
	if callCount != 1 {
		t.Errorf("expected still 1 call (cached), got %d", callCount)
	}
}

func TestGetCacheExpiry(t *testing.T) {
	callCount := 0
	srv := mockCFAPI(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Write(cfResponse("data"))
	})
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL), WithCacheTTL(1*time.Millisecond))

	c.get("/path")
	time.Sleep(5 * time.Millisecond) // let cache expire
	c.get("/path")

	if callCount != 2 {
		t.Errorf("expected 2 calls after cache expiry, got %d", callCount)
	}
}

func TestGetAPIError(t *testing.T) {
	srv := mockCFAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write(cfErrorResponse(9109, "Invalid access token"))
	})
	defer srv.Close()

	c := NewClient("bad-token", WithBaseURL(srv.URL))
	_, err := c.get("/test")
	if err == nil {
		t.Fatal("expected error for API error response")
	}
	if !strings.Contains(err.Error(), "Invalid access token") {
		t.Errorf("expected token error message, got %q", err.Error())
	}
}

func TestGetRateLimited(t *testing.T) {
	srv := mockCFAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		w.Write([]byte(`{"success":false,"errors":[{"code":429,"message":"rate limited"}]}`))
	})
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	_, err := c.get("/test")
	if err == nil {
		t.Fatal("expected error for 429")
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("expected rate limit error, got %q", err.Error())
	}
}

func TestGetMinTLSVersion(t *testing.T) {
	srv := mockCFAPI(t, func(w http.ResponseWriter, r *http.Request) {
		setting := ZoneSetting{
			ID:    "min_tls_version",
			Value: json.RawMessage(`"1.2"`),
		}
		w.Write(cfResponse(setting))
	})
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	version, err := c.GetMinTLSVersion("zone1")
	if err != nil {
		t.Fatal(err)
	}
	if version != "1.2" {
		t.Errorf("expected 1.2, got %q", version)
	}
}

func TestGetCiphers(t *testing.T) {
	srv := mockCFAPI(t, func(w http.ResponseWriter, r *http.Request) {
		setting := ZoneSetting{
			ID:    "ciphers",
			Value: json.RawMessage(`["ECDHE-RSA-AES128-GCM-SHA256","ECDHE-RSA-AES256-GCM-SHA384"]`),
		}
		w.Write(cfResponse(setting))
	})
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	ciphers, err := c.GetCiphers("zone1")
	if err != nil {
		t.Fatal(err)
	}
	if len(ciphers) != 2 {
		t.Errorf("expected 2 ciphers, got %d", len(ciphers))
	}
}

func TestGetSecurityHeader(t *testing.T) {
	srv := mockCFAPI(t, func(w http.ResponseWriter, r *http.Request) {
		setting := ZoneSetting{
			ID:    "security_header",
			Value: json.RawMessage(`{"strict_transport_security":{"enabled":true,"max_age":31536000}}`),
		}
		w.Write(cfResponse(setting))
	})
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	header, err := c.GetSecurityHeader("zone1")
	if err != nil {
		t.Fatal(err)
	}
	if !header.StrictTransportSecurity.Enabled {
		t.Error("expected HSTS enabled")
	}
	if header.StrictTransportSecurity.MaxAge != 31536000 {
		t.Errorf("expected max_age 31536000, got %d", header.StrictTransportSecurity.MaxAge)
	}
}

func TestGetCertificatePacks(t *testing.T) {
	srv := mockCFAPI(t, func(w http.ResponseWriter, r *http.Request) {
		packs := []CertificatePack{{
			ID:     "cert1",
			Type:   "universal",
			Status: "active",
			Hosts:  []string{"example.com"},
		}}
		w.Write(cfResponse(packs))
	})
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	packs, err := c.GetCertificatePacks("zone1")
	if err != nil {
		t.Fatal(err)
	}
	if len(packs) != 1 {
		t.Fatalf("expected 1 cert pack, got %d", len(packs))
	}
	if packs[0].Status != "active" {
		t.Errorf("expected active status, got %q", packs[0].Status)
	}
}

func TestGetAccessApps(t *testing.T) {
	srv := mockCFAPI(t, func(w http.ResponseWriter, r *http.Request) {
		apps := []AccessApp{{
			ID:   "app1",
			Name: "Test App",
			Type: "self_hosted",
		}}
		w.Write(cfResponse(apps))
	})
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	apps, err := c.GetAccessApps("zone1")
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(apps))
	}
	if apps[0].Name != "Test App" {
		t.Errorf("expected Test App, got %q", apps[0].Name)
	}
}

func TestGetTunnel(t *testing.T) {
	srv := mockCFAPI(t, func(w http.ResponseWriter, r *http.Request) {
		tunnel := TunnelInfo{
			ID:     "tun1",
			Name:   "my-tunnel",
			Status: "active",
		}
		w.Write(cfResponse(tunnel))
	})
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	tunnel, err := c.GetTunnel("acct1", "tun1")
	if err != nil {
		t.Fatal(err)
	}
	if tunnel.Status != "active" {
		t.Errorf("expected active, got %q", tunnel.Status)
	}
}

func TestGetKeylessSSLConfigs(t *testing.T) {
	srv := mockCFAPI(t, func(w http.ResponseWriter, r *http.Request) {
		configs := []KeylessSSLConfig{{
			ID:      "kl1",
			Name:    "HSM Keyless",
			Status:  "active",
			Enabled: true,
			Port:    2407,
		}}
		w.Write(cfResponse(configs))
	})
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	configs, err := c.GetKeylessSSLConfigs("zone1")
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}
	if configs[0].Port != 2407 {
		t.Errorf("expected port 2407, got %d", configs[0].Port)
	}
}

func TestGetRegionalTieredCache(t *testing.T) {
	tests := []struct {
		value    string
		expected bool
	}{
		{"on", true},
		{"off", false},
	}

	for _, tt := range tests {
		srv := mockCFAPI(t, func(w http.ResponseWriter, r *http.Request) {
			w.Write(cfResponse(RegionalTieredCache{ID: "regional", Value: tt.value}))
		})

		c := NewClient("tok", WithBaseURL(srv.URL))
		result, err := c.GetRegionalTieredCache("zone1")
		if err != nil {
			t.Fatal(err)
		}
		if result != tt.expected {
			t.Errorf("value=%q: expected %v, got %v", tt.value, tt.expected, result)
		}
		srv.Close()
	}
}

func TestGetNetworkError(t *testing.T) {
	c := NewClient("tok", WithBaseURL("http://127.0.0.1:1"))
	_, err := c.get("/test")
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestGetInvalidJSON(t *testing.T) {
	srv := mockCFAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	})
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	_, err := c.get("/test")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
