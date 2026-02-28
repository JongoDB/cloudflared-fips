package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cloudflared-fips/cloudflared-fips/pkg/clientdetect"
)

// ---------------------------------------------------------------------------
// startDashboard — test the HTTP endpoints it registers
// ---------------------------------------------------------------------------

func newTestDashboard() http.Handler {
	inspector := clientdetect.NewInspector(100)
	logger := log.New(io.Discard, "", 0)
	srv := startDashboard("127.0.0.1:0", inspector, logger)
	return srv.Handler
}

func TestDashboard_Health(t *testing.T) {
	handler := newTestDashboard()

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("health status = %d, want 200", w.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status = %q, want ok", body["status"])
	}
	if body["mode"] != "fips-proxy-tier3" {
		t.Errorf("mode = %q, want fips-proxy-tier3", body["mode"])
	}
}

func TestDashboard_Clients_Empty(t *testing.T) {
	handler := newTestDashboard()

	req := httptest.NewRequest("GET", "/api/v1/clients", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("clients status = %d, want 200", w.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := body["summary"]; !ok {
		t.Error("expected 'summary' key in response")
	}
	if _, ok := body["recent"]; !ok {
		t.Error("expected 'recent' key in response")
	}

	// With no clients, summary should show zeros
	summary, ok := body["summary"].(map[string]interface{})
	if !ok {
		t.Fatal("summary is not a map")
	}
	if total, _ := summary["total"].(float64); total != 0 {
		t.Errorf("total = %v, want 0", total)
	}
}

func TestDashboard_Metrics(t *testing.T) {
	handler := newTestDashboard()

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("metrics status = %d, want 200", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "text/plain" {
		t.Errorf("Content-Type = %q, want text/plain", ct)
	}

	body := w.Body.String()
	if !strings.Contains(body, "fips_proxy_clients_total") {
		t.Error("expected fips_proxy_clients_total metric")
	}
	if !strings.Contains(body, "fips_proxy_clients_fips") {
		t.Error("expected fips_proxy_clients_fips metric")
	}
	if !strings.Contains(body, "fips_proxy_clients_nonfips") {
		t.Error("expected fips_proxy_clients_nonfips metric")
	}
	if !strings.Contains(body, "# HELP") {
		t.Error("expected Prometheus HELP lines")
	}
	if !strings.Contains(body, "# TYPE") {
		t.Error("expected Prometheus TYPE lines")
	}
}

func TestDashboard_SelfTest(t *testing.T) {
	handler := newTestDashboard()

	req := httptest.NewRequest("GET", "/api/v1/selftest", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("selftest status = %d, want 200", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := body["results"]; !ok {
		t.Error("expected 'results' key in selftest response")
	}
}
