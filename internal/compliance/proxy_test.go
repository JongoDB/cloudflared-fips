package compliance

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProxyStatsChecker_AllFIPSClients(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/clients" {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"summary": map[string]int{
				"total":        25,
				"fips_capable": 25,
				"non_fips":     0,
			},
		})
	}))
	defer srv.Close()

	addr := strings.TrimPrefix(srv.URL, "http://")
	checker := NewProxyStatsChecker(addr)
	section := checker.RunGatewayClientChecks()

	if section.ID != "gateway" {
		t.Errorf("expected section ID 'gateway', got %q", section.ID)
	}
	if len(section.Items) != 4 {
		t.Fatalf("expected 4 items, got %d", len(section.Items))
	}

	// All items should pass when all clients are FIPS-capable
	for _, item := range section.Items {
		if item.Status != StatusPass {
			t.Errorf("item %q: expected pass, got %s (what: %s)", item.Name, item.Status, item.What)
		}
	}

	// Verify counts in items
	if !strings.Contains(section.Items[0].What, "25") {
		t.Errorf("expected total count 25 in item, got: %s", section.Items[0].What)
	}
}

func TestProxyStatsChecker_NonFIPSClients(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"summary": map[string]int{
				"total":        10,
				"fips_capable": 7,
				"non_fips":     3,
			},
		})
	}))
	defer srv.Close()

	addr := strings.TrimPrefix(srv.URL, "http://")
	checker := NewProxyStatsChecker(addr)
	section := checker.RunGatewayClientChecks()

	if len(section.Items) != 4 {
		t.Fatalf("expected 4 items, got %d", len(section.Items))
	}

	// FIPS-capable item should warn (not 100%)
	if section.Items[1].Status != StatusWarning {
		t.Errorf("FIPS-capable item: expected warning, got %s", section.Items[1].Status)
	}

	// Non-FIPS item should warn
	if section.Items[2].Status != StatusWarning {
		t.Errorf("Non-FIPS item: expected warning, got %s", section.Items[2].Status)
	}
	if !strings.Contains(section.Items[2].What, "3 non-FIPS") {
		t.Errorf("expected '3 non-FIPS' in what, got: %s", section.Items[2].What)
	}
}

func TestProxyStatsChecker_NoClients(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"summary": map[string]int{
				"total":        0,
				"fips_capable": 0,
				"non_fips":     0,
			},
		})
	}))
	defer srv.Close()

	addr := strings.TrimPrefix(srv.URL, "http://")
	checker := NewProxyStatsChecker(addr)
	section := checker.RunGatewayClientChecks()

	// With 0 clients, FIPS-capable and non-FIPS items should be unknown
	if section.Items[1].Status != StatusUnknown {
		t.Errorf("FIPS-capable item with 0 clients: expected unknown, got %s", section.Items[1].Status)
	}
	if section.Items[2].Status != StatusUnknown {
		t.Errorf("Non-FIPS item with 0 clients: expected unknown, got %s", section.Items[2].Status)
	}
}

func TestProxyStatsChecker_ProxyUnreachable(t *testing.T) {
	// Use an address that won't have anything listening
	checker := NewProxyStatsChecker("127.0.0.1:1")
	section := checker.RunGatewayClientChecks()

	if len(section.Items) != 4 {
		t.Fatalf("expected 4 items, got %d", len(section.Items))
	}

	// All items should be unknown when proxy is unreachable (fail closed)
	for _, item := range section.Items {
		if item.Status != StatusUnknown {
			t.Errorf("item %q: expected unknown when proxy unreachable, got %s", item.Name, item.Status)
		}
	}
}

func TestProxyStatsChecker_ProxyBadResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	addr := strings.TrimPrefix(srv.URL, "http://")
	checker := NewProxyStatsChecker(addr)
	section := checker.RunGatewayClientChecks()

	// Bad response = all unknown
	for _, item := range section.Items {
		if item.Status != StatusUnknown {
			t.Errorf("item %q: expected unknown on bad response, got %s", item.Name, item.Status)
		}
	}
}

func TestProxyStatsChecker_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	addr := strings.TrimPrefix(srv.URL, "http://")
	checker := NewProxyStatsChecker(addr)
	section := checker.RunGatewayClientChecks()

	// Invalid JSON = all items present but first should be unknown (bad parse treated as zero values)
	if len(section.Items) != 4 {
		t.Fatalf("expected 4 items, got %d", len(section.Items))
	}
}
