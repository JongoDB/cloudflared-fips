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
