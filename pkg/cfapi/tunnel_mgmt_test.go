package cfapi

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestCreateTunnel(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody createTunnelRequest

	srv := mockCFAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		resp := createTunnelResponse{
			ID:   "tun-uuid-1234",
			Name: "my-fips-tunnel",
		}
		_, _ = w.Write(cfResponse(resp))
	})
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	result, err := c.CreateTunnel("acct-123", "my-fips-tunnel")
	if err != nil {
		t.Fatal(err)
	}

	if gotMethod != "POST" {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if !strings.Contains(gotPath, "/accounts/acct-123/cfd_tunnel") {
		t.Errorf("unexpected path: %s", gotPath)
	}
	if gotBody.Name != "my-fips-tunnel" {
		t.Errorf("expected tunnel name 'my-fips-tunnel', got %q", gotBody.Name)
	}
	if gotBody.TunnelSecret == "" {
		t.Error("expected tunnel_secret to be set")
	}

	if result.ID != "tun-uuid-1234" {
		t.Errorf("expected tunnel ID tun-uuid-1234, got %q", result.ID)
	}
	if result.Name != "my-fips-tunnel" {
		t.Errorf("expected tunnel name my-fips-tunnel, got %q", result.Name)
	}
	if result.Token == "" {
		t.Error("expected non-empty token")
	}

	// Verify token is valid base64-encoded JSON
	tokenJSON, err := base64.StdEncoding.DecodeString(result.Token)
	if err != nil {
		t.Fatalf("token is not valid base64: %v", err)
	}
	var tokenData map[string]string
	if err := json.Unmarshal(tokenJSON, &tokenData); err != nil {
		t.Fatalf("token is not valid JSON: %v", err)
	}
	if tokenData["a"] != "acct-123" {
		t.Errorf("expected account ID acct-123 in token, got %q", tokenData["a"])
	}
	if tokenData["t"] != "tun-uuid-1234" {
		t.Errorf("expected tunnel ID tun-uuid-1234 in token, got %q", tokenData["t"])
	}
	if tokenData["s"] == "" {
		t.Error("expected secret in token")
	}
}

func TestCreateTunnelAPIError(t *testing.T) {
	srv := mockCFAPI(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(cfErrorResponse(1003, "tunnel name already exists"))
	})
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	_, err := c.CreateTunnel("acct-123", "existing-tunnel")
	if err == nil {
		t.Fatal("expected error for duplicate tunnel name")
	}
	if !strings.Contains(err.Error(), "tunnel name already exists") {
		t.Errorf("expected 'tunnel name already exists' error, got: %v", err)
	}
}

func TestConfigureTunnelIngress(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody TunnelIngressConfig

	srv := mockCFAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		_, _ = w.Write(cfResponse(map[string]interface{}{}))
	})
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	ingress := []TunnelIngressRule{
		{Hostname: "dashboard.example.com", Service: "http://localhost:8080"},
		{Service: "http_status:404"},
	}
	err := c.ConfigureTunnelIngress("acct-123", "tun-uuid-1234", ingress)
	if err != nil {
		t.Fatal(err)
	}

	if gotMethod != "PUT" {
		t.Errorf("expected PUT, got %s", gotMethod)
	}
	if !strings.Contains(gotPath, "/accounts/acct-123/cfd_tunnel/tun-uuid-1234/configurations") {
		t.Errorf("unexpected path: %s", gotPath)
	}
	if len(gotBody.Config.Ingress) != 2 {
		t.Errorf("expected 2 ingress rules, got %d", len(gotBody.Config.Ingress))
	}
	if gotBody.Config.Ingress[0].Hostname != "dashboard.example.com" {
		t.Errorf("expected hostname dashboard.example.com, got %q", gotBody.Config.Ingress[0].Hostname)
	}
}

func TestConfigureTunnelIngressAPIError(t *testing.T) {
	srv := mockCFAPI(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(cfErrorResponse(1004, "invalid tunnel configuration"))
	})
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	err := c.ConfigureTunnelIngress("acct-123", "tun-uuid-1234", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid tunnel configuration") {
		t.Errorf("expected config error, got: %v", err)
	}
}

func TestCreateDNSCNAME(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody DNSRecord

	srv := mockCFAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		_, _ = w.Write(cfResponse(DNSRecordResult{
			ID:      "dns-record-id",
			Type:    "CNAME",
			Name:    "dashboard.example.com",
			Content: "tun-uuid-1234.cfargotunnel.com",
		}))
	})
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	result, err := c.CreateDNSCNAME("zone-123", "dashboard.example.com", "tun-uuid-1234")
	if err != nil {
		t.Fatal(err)
	}

	if gotMethod != "POST" {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if !strings.Contains(gotPath, "/zones/zone-123/dns_records") {
		t.Errorf("unexpected path: %s", gotPath)
	}
	if gotBody.Type != "CNAME" {
		t.Errorf("expected CNAME type, got %q", gotBody.Type)
	}
	if gotBody.Name != "dashboard.example.com" {
		t.Errorf("expected name dashboard.example.com, got %q", gotBody.Name)
	}
	if gotBody.Content != "tun-uuid-1234.cfargotunnel.com" {
		t.Errorf("expected content tun-uuid-1234.cfargotunnel.com, got %q", gotBody.Content)
	}
	if !gotBody.Proxied {
		t.Error("expected proxied=true")
	}

	if result.ID != "dns-record-id" {
		t.Errorf("expected result ID dns-record-id, got %q", result.ID)
	}
}

func TestCreateDNSCNAMEAPIError(t *testing.T) {
	srv := mockCFAPI(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(cfErrorResponse(81053, "record already exists"))
	})
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	_, err := c.CreateDNSCNAME("zone-123", "dashboard.example.com", "tun-uuid-1234")
	if err == nil {
		t.Fatal("expected error for duplicate record")
	}
	if !strings.Contains(err.Error(), "record already exists") {
		t.Errorf("expected duplicate record error, got: %v", err)
	}
}

func TestGenerateTunnelToken(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef") // 32 bytes
	token := GenerateTunnelToken("account-id", "tunnel-id", secret)

	// Decode
	tokenJSON, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("failed to decode token: %v", err)
	}

	var data map[string]string
	if err := json.Unmarshal(tokenJSON, &data); err != nil {
		t.Fatalf("failed to parse token JSON: %v", err)
	}

	if data["a"] != "account-id" {
		t.Errorf("expected account 'account-id', got %q", data["a"])
	}
	if data["t"] != "tunnel-id" {
		t.Errorf("expected tunnel 'tunnel-id', got %q", data["t"])
	}

	// Verify the secret round-trips
	decodedSecret, err := base64.StdEncoding.DecodeString(data["s"])
	if err != nil {
		t.Fatalf("failed to decode secret from token: %v", err)
	}
	if string(decodedSecret) != string(secret) {
		t.Errorf("secret mismatch: got %q, want %q", decodedSecret, secret)
	}
}

func TestPostMethod(t *testing.T) {
	var gotMethod string
	srv := mockCFAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		_, _ = w.Write(cfResponse("created"))
	})
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	_, err := c.post("/test", map[string]string{"key": "value"})
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != "POST" {
		t.Errorf("expected POST method, got %s", gotMethod)
	}
}

func TestPutMethod(t *testing.T) {
	var gotMethod string
	srv := mockCFAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		_, _ = w.Write(cfResponse("replaced"))
	})
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	_, err := c.put("/test", map[string]string{"key": "value"})
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != "PUT" {
		t.Errorf("expected PUT method, got %s", gotMethod)
	}
}

func TestPostRateLimited(t *testing.T) {
	srv := mockCFAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`{"success":false,"errors":[{"code":429,"message":"rate limited"}]}`))
	})
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	_, err := c.post("/test", nil)
	if err == nil {
		t.Fatal("expected error for 429")
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("expected rate limit error, got %q", err.Error())
	}
}

func TestPutRateLimited(t *testing.T) {
	srv := mockCFAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`{"success":false,"errors":[{"code":429,"message":"rate limited"}]}`))
	})
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	_, err := c.put("/test", nil)
	if err == nil {
		t.Fatal("expected error for 429")
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("expected rate limit error, got %q", err.Error())
	}
}
