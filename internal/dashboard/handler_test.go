package dashboard

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudflared-fips/cloudflared-fips/internal/compliance"
)

func testChecker() *compliance.Checker {
	checker := compliance.NewChecker()
	checker.AddSection(compliance.Section{
		ID:   "test-section",
		Name: "Test Section",
		Items: []compliance.ChecklistItem{
			{
				ID:                 "t-1",
				Name:               "Test item 1",
				Status:             compliance.StatusPass,
				Severity:           "critical",
				VerificationMethod: compliance.VerifyDirect,
			},
			{
				ID:                 "t-2",
				Name:               "Test item 2",
				Status:             compliance.StatusWarning,
				Severity:           "high",
				VerificationMethod: compliance.VerifyAPI,
			},
		},
	})
	return checker
}

func TestHandleHealth(t *testing.T) {
	handler := NewHandler("", testChecker())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()
	handler.HandleHealth(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("expected status ok, got %q", body["status"])
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %q", ct)
	}
}

func TestHandleCompliance(t *testing.T) {
	handler := NewHandler("", testChecker())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/compliance", nil)
	w := httptest.NewRecorder()
	handler.HandleCompliance(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var report compliance.ComplianceReport
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		t.Fatalf("decode report: %v", err)
	}

	if report.Summary.Total != 2 {
		t.Errorf("expected 2 total items, got %d", report.Summary.Total)
	}
	if report.Summary.Passed != 1 {
		t.Errorf("expected 1 passed, got %d", report.Summary.Passed)
	}
	if report.Summary.Warnings != 1 {
		t.Errorf("expected 1 warning, got %d", report.Summary.Warnings)
	}
	if len(report.Sections) != 1 {
		t.Errorf("expected 1 section, got %d", len(report.Sections))
	}
}

func TestHandleManifest(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.json")
	content := `{"version":"1.0.0","commit":"abc123","crypto_engine":"boringcrypto"}`
	if err := os.WriteFile(manifestPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	handler := NewHandler(manifestPath, testChecker())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manifest", nil)
	w := httptest.NewRecorder()
	handler.HandleManifest(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["version"] != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %v", body["version"])
	}
}

func TestHandleManifestMissing(t *testing.T) {
	handler := NewHandler("/nonexistent/manifest.json", testChecker())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manifest", nil)
	w := httptest.NewRecorder()
	handler.HandleManifest(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleExportJSON(t *testing.T) {
	handler := NewHandler("", testChecker())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/compliance/export?format=json", nil)
	w := httptest.NewRecorder()
	handler.HandleExport(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	cd := w.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "compliance-report.json") {
		t.Errorf("expected Content-Disposition with filename, got %q", cd)
	}
}

func TestHandleExportDefaultFormat(t *testing.T) {
	handler := NewHandler("", testChecker())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/compliance/export", nil)
	w := httptest.NewRecorder()
	handler.HandleExport(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for default format, got %d", w.Code)
	}
}

func TestHandleExportPDF(t *testing.T) {
	handler := NewHandler("", testChecker())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/compliance/export?format=pdf", nil)
	w := httptest.NewRecorder()
	handler.HandleExport(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501 for PDF, got %d", w.Code)
	}
}

func TestHandleExportInvalidFormat(t *testing.T) {
	handler := NewHandler("", testChecker())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/compliance/export?format=xml", nil)
	w := httptest.NewRecorder()
	handler.HandleExport(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid format, got %d", w.Code)
	}
}

func TestHandleSSE(t *testing.T) {
	handler := NewHandler("", testChecker())

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.HandleSSE(w, req)

	body, _ := io.ReadAll(w.Result().Body)
	bodyStr := string(body)

	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected text/event-stream, got %q", ct)
	}
	if !strings.Contains(bodyStr, "event: compliance") {
		t.Errorf("expected SSE compliance event, body: %q", bodyStr)
	}
	if !strings.Contains(bodyStr, "data: ") {
		t.Errorf("expected SSE data field, body: %q", bodyStr)
	}
}

func TestRegisterRoutes(t *testing.T) {
	handler := NewHandler("", testChecker())
	mux := http.NewServeMux()
	RegisterRoutes(mux, handler)

	endpoints := []struct {
		path   string
		status int
	}{
		{"/api/v1/health", http.StatusOK},
		{"/api/v1/compliance", http.StatusOK},
		{"/api/v1/selftest", http.StatusOK},
	}

	for _, ep := range endpoints {
		req := httptest.NewRequest(http.MethodGet, ep.path, nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != ep.status {
			t.Errorf("%s: expected %d, got %d", ep.path, ep.status, w.Code)
		}
	}
}

func TestHandleSelfTest(t *testing.T) {
	handler := NewHandler("", testChecker())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/selftest", nil)
	w := httptest.NewRecorder()
	handler.HandleSelfTest(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := body["results"]; !ok {
		t.Error("expected 'results' key in selftest response")
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusCreated, map[string]string{"key": "value"})

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %q", ct)
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["key"] != "value" {
		t.Errorf("expected value, got %q", body["key"])
	}
}

func TestSecurityHeaders(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wrapped := SecurityHeaders(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	headers := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":       "DENY",
		"Referrer-Policy":       "strict-origin-when-cross-origin",
	}
	for key, expected := range headers {
		if got := w.Header().Get(key); got != expected {
			t.Errorf("header %s: expected %q, got %q", key, expected, got)
		}
	}
}

func TestWriteSSEEvent(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"status": "ok"}

	err := writeSSEEvent(w, w, "test-event", data)
	if err != nil {
		t.Fatalf("writeSSEEvent: %v", err)
	}

	body := w.Body.String()
	if !strings.Contains(body, "event: test-event") {
		t.Errorf("expected event name, got %q", body)
	}
	if !strings.Contains(body, `"status":"ok"`) {
		t.Errorf("expected data, got %q", body)
	}
}

// ---------------------------------------------------------------------------
// Integration: proxy → ProxyStatsChecker → Checker → Dashboard /api/v1/compliance
// ---------------------------------------------------------------------------

// TestProxyToDashboardIntegration simulates the full data flow:
// 1. Mock fips-proxy serves /api/v1/clients with TLS inspection stats
// 2. ProxyStatsChecker fetches from mock proxy
// 3. Gateway section is added to the compliance Checker
// 4. Dashboard handler serves /api/v1/compliance with gateway data included
func TestProxyToDashboardIntegration(t *testing.T) {
	// Step 1: Mock fips-proxy returning client stats
	mockProxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/clients" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"summary": map[string]interface{}{
				"total":        42,
				"fips_capable": 38,
				"non_fips":     4,
			},
		})
	}))
	defer mockProxy.Close()

	// Step 2: ProxyStatsChecker pointed at mock proxy
	proxyAddr := strings.TrimPrefix(mockProxy.URL, "http://")
	proxyChecker := compliance.NewProxyStatsChecker(proxyAddr)

	// Step 3: Build Checker with gateway section
	checker := compliance.NewChecker()
	checker.AddSection(proxyChecker.RunGatewayClientChecks())

	// Step 4: Dashboard handler serves the report
	handler := NewHandler("", checker)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/compliance", nil)
	w := httptest.NewRecorder()
	handler.HandleCompliance(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var report compliance.ComplianceReport
	if err := json.NewDecoder(w.Body).Decode(&report); err != nil {
		t.Fatalf("decode report: %v", err)
	}

	// Verify gateway section is present
	var gatewaySec *compliance.Section
	for i, s := range report.Sections {
		if s.ID == "gateway" {
			gatewaySec = &report.Sections[i]
			break
		}
	}
	if gatewaySec == nil {
		t.Fatal("expected 'gateway' section in compliance report, not found")
	}

	if gatewaySec.Name != "Gateway Clients" {
		t.Errorf("section name = %q, want 'Gateway Clients'", gatewaySec.Name)
	}

	if len(gatewaySec.Items) != 4 {
		t.Fatalf("expected 4 gateway items, got %d", len(gatewaySec.Items))
	}

	// gw-1: total connections
	gw1 := gatewaySec.Items[0]
	if gw1.ID != "gw-1" {
		t.Errorf("item 0 ID = %q, want gw-1", gw1.ID)
	}
	if gw1.Status != compliance.StatusPass {
		t.Errorf("gw-1 status = %q, want pass", gw1.Status)
	}
	if !strings.Contains(gw1.What, "42") {
		t.Errorf("gw-1 should mention 42 total connections, got: %s", gw1.What)
	}

	// gw-2: FIPS-capable (38 of 42 = ~90%, not 100% → warning)
	gw2 := gatewaySec.Items[1]
	if gw2.ID != "gw-2" {
		t.Errorf("item 1 ID = %q, want gw-2", gw2.ID)
	}
	if gw2.Status != compliance.StatusWarning {
		t.Errorf("gw-2 status = %q, want warning (not 100%% FIPS)", gw2.Status)
	}
	if !strings.Contains(gw2.What, "38") {
		t.Errorf("gw-2 should mention 38 FIPS-capable clients, got: %s", gw2.What)
	}

	// gw-3: non-FIPS clients (4 > 0 → warning)
	gw3 := gatewaySec.Items[2]
	if gw3.ID != "gw-3" {
		t.Errorf("item 2 ID = %q, want gw-3", gw3.ID)
	}
	if gw3.Status != compliance.StatusWarning {
		t.Errorf("gw-3 status = %q, want warning (4 non-FIPS)", gw3.Status)
	}
	if !strings.Contains(gw3.What, "4") {
		t.Errorf("gw-3 should mention 4 non-FIPS clients, got: %s", gw3.What)
	}

	// gw-4: proxy active
	gw4 := gatewaySec.Items[3]
	if gw4.ID != "gw-4" {
		t.Errorf("item 3 ID = %q, want gw-4", gw4.ID)
	}
	if gw4.Status != compliance.StatusPass {
		t.Errorf("gw-4 status = %q, want pass", gw4.Status)
	}

	// Verify summary counts
	if report.Summary.Total != 4 {
		t.Errorf("total items = %d, want 4", report.Summary.Total)
	}
	// 2 pass (gw-1, gw-4) + 2 warnings (gw-2, gw-3)
	if report.Summary.Passed != 2 {
		t.Errorf("passed = %d, want 2", report.Summary.Passed)
	}
	if report.Summary.Warnings != 2 {
		t.Errorf("warnings = %d, want 2", report.Summary.Warnings)
	}
}

// TestProxyToDashboardIntegration_ProxyDown verifies fail-closed behavior:
// when the proxy is unreachable, the gateway section shows all-unknown items.
func TestProxyToDashboardIntegration_ProxyDown(t *testing.T) {
	// Point at an address that will definitely refuse connections
	proxyChecker := compliance.NewProxyStatsChecker("127.0.0.1:1")

	checker := compliance.NewChecker()
	checker.AddSection(proxyChecker.RunGatewayClientChecks())

	handler := NewHandler("", checker)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/compliance", nil)
	w := httptest.NewRecorder()
	handler.HandleCompliance(w, req)

	var report compliance.ComplianceReport
	if err := json.NewDecoder(w.Body).Decode(&report); err != nil {
		t.Fatalf("decode: %v", err)
	}

	var gatewaySec *compliance.Section
	for i, s := range report.Sections {
		if s.ID == "gateway" {
			gatewaySec = &report.Sections[i]
			break
		}
	}
	if gatewaySec == nil {
		t.Fatal("expected gateway section even when proxy is down")
	}

	// All 4 items should be unknown (fail-closed)
	for _, item := range gatewaySec.Items {
		if item.Status != compliance.StatusUnknown {
			t.Errorf("item %s: status = %q, want unknown (fail-closed)", item.ID, item.Status)
		}
	}

	if report.Summary.Unknown != 4 {
		t.Errorf("unknown count = %d, want 4", report.Summary.Unknown)
	}
}

// TestProxyToDashboardIntegration_AllFIPS verifies the happy path:
// 100% FIPS clients → all items pass.
func TestProxyToDashboardIntegration_AllFIPS(t *testing.T) {
	mockProxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"summary": map[string]interface{}{
				"total":        100,
				"fips_capable": 100,
				"non_fips":     0,
			},
		})
	}))
	defer mockProxy.Close()

	proxyAddr := strings.TrimPrefix(mockProxy.URL, "http://")
	proxyChecker := compliance.NewProxyStatsChecker(proxyAddr)

	checker := compliance.NewChecker()
	checker.AddSection(proxyChecker.RunGatewayClientChecks())

	handler := NewHandler("", checker)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/compliance", nil)
	w := httptest.NewRecorder()
	handler.HandleCompliance(w, req)

	var report compliance.ComplianceReport
	if err := json.NewDecoder(w.Body).Decode(&report); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// All 4 items should pass when 100% FIPS
	if report.Summary.Passed != 4 {
		t.Errorf("passed = %d, want 4 (all FIPS)", report.Summary.Passed)
	}
	if report.Summary.Warnings != 0 {
		t.Errorf("warnings = %d, want 0", report.Summary.Warnings)
	}
	if report.Summary.Failed != 0 {
		t.Errorf("failed = %d, want 0", report.Summary.Failed)
	}
}

// TestProxyToDashboardIntegration_SSE verifies gateway data flows through SSE.
func TestProxyToDashboardIntegration_SSE(t *testing.T) {
	mockProxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"summary": map[string]interface{}{
				"total":        10,
				"fips_capable": 8,
				"non_fips":     2,
			},
		})
	}))
	defer mockProxy.Close()

	proxyAddr := strings.TrimPrefix(mockProxy.URL, "http://")
	proxyChecker := compliance.NewProxyStatsChecker(proxyAddr)

	checker := compliance.NewChecker()
	checker.AddSection(proxyChecker.RunGatewayClientChecks())

	handler := NewHandler("", checker)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.HandleSSE(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "event: compliance") {
		t.Error("expected SSE compliance event")
	}
	if !strings.Contains(body, "gateway") {
		t.Error("expected gateway section in SSE data")
	}
	if !strings.Contains(body, "gw-1") {
		t.Error("expected gw-1 item in SSE data")
	}
}
