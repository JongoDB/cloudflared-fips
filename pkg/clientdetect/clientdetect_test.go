package clientdetect

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewInspector(t *testing.T) {
	ins := NewInspector(100)
	if ins.maxLog != 100 {
		t.Errorf("expected maxLog 100, got %d", ins.maxLog)
	}

	// Default maxLog when <= 0
	ins2 := NewInspector(0)
	if ins2.maxLog != 1000 {
		t.Errorf("expected default maxLog 1000, got %d", ins2.maxLog)
	}

	ins3 := NewInspector(-5)
	if ins3.maxLog != 1000 {
		t.Errorf("expected default maxLog 1000, got %d", ins3.maxLog)
	}
}

func TestRecentClientsEmpty(t *testing.T) {
	ins := NewInspector(10)
	clients := ins.RecentClients(5)
	if len(clients) != 0 {
		t.Errorf("expected 0 clients, got %d", len(clients))
	}
}

func TestFIPSStatsEmpty(t *testing.T) {
	ins := NewInspector(10)
	stats := ins.FIPSStats()
	if stats.Total != 0 || stats.FIPSCapable != 0 || stats.NonFIPS != 0 {
		t.Errorf("expected all zero stats, got %+v", stats)
	}
}

func TestAnalyzeFIPSCapability(t *testing.T) {
	tests := []struct {
		name      string
		suites    []uint16
		wantFIPS  bool
		wantMatch string
	}{
		{
			name: "AES-GCM only (FIPS)",
			suites: []uint16{
				0xc02f, // TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
				0xc030, // TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384
			},
			wantFIPS:  true,
			wantMatch: "No ChaCha20",
		},
		{
			name: "ChaCha20 present (non-FIPS)",
			suites: []uint16{
				0xc02f, // TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
				0xcca8, // TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256
			},
			wantFIPS:  false,
			wantMatch: "ChaCha20",
		},
		{
			name:      "Empty suites",
			suites:    []uint16{},
			wantFIPS:  false,
			wantMatch: "does not offer AES-GCM",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFIPS, reason := analyzeFIPSCapability(tt.suites)
			if gotFIPS != tt.wantFIPS {
				t.Errorf("FIPS capability: got %v, want %v (reason: %s)", gotFIPS, tt.wantFIPS, reason)
			}
			if !strings.Contains(reason, tt.wantMatch) {
				t.Errorf("reason %q does not contain %q", reason, tt.wantMatch)
			}
		})
	}
}

func TestMatchKnownFIPSClient(t *testing.T) {
	tests := []struct {
		hash    string
		wantOK  bool
		partial string
	}{
		{"t13_04_h2", true, "Windows FIPS"},
		{"t13_08_h2", true, "RHEL Firefox"},
		{"t13_04_h2_abc123def456", true, "Windows FIPS"},
		{"t99_99_xx_deadbeef", false, ""},
		{"invalid", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.hash, func(t *testing.T) {
			desc, ok := MatchKnownFIPSClient(tt.hash)
			if ok != tt.wantOK {
				t.Errorf("MatchKnownFIPSClient(%q) = _, %v; want %v", tt.hash, ok, tt.wantOK)
			}
			if tt.partial != "" && !strings.Contains(desc, tt.partial) {
				t.Errorf("description %q does not contain %q", desc, tt.partial)
			}
		})
	}
}

func TestKnownFIPSFingerprintsNotEmpty(t *testing.T) {
	if len(KnownFIPSFingerprints) == 0 {
		t.Error("KnownFIPSFingerprints should not be empty")
	}
}

// PostureCollector tests

func TestNewPostureCollector(t *testing.T) {
	pc := NewPostureCollector()
	if pc == nil {
		t.Fatal("expected non-nil PostureCollector")
	}
	if len(pc.AllDevices()) != 0 {
		t.Error("expected empty device list")
	}
}

func TestPostureCollectorReportAndGet(t *testing.T) {
	pc := NewPostureCollector()

	// POST a device posture
	body := `{"device_id":"dev1","os_fips_enabled":true,"os_type":"linux","os_version":"RHEL 9","mdm_enrolled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posture", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	pc.HandlePostureReport(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify device stored
	dev, ok := pc.GetDevice("dev1")
	if !ok {
		t.Fatal("device not found")
	}
	if !dev.OSFIPSEnabled {
		t.Error("expected FIPS enabled")
	}
	if dev.OSType != "linux" {
		t.Errorf("expected linux, got %q", dev.OSType)
	}
}

func TestPostureCollectorMissingDeviceID(t *testing.T) {
	pc := NewPostureCollector()
	body := `{"os_fips_enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posture", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	pc.HandlePostureReport(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestPostureCollectorInvalidJSON(t *testing.T) {
	pc := NewPostureCollector()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posture", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	pc.HandlePostureReport(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestPostureCollectorHandleList(t *testing.T) {
	pc := NewPostureCollector()

	// Add two devices
	for _, id := range []string{"dev1", "dev2"} {
		body, _ := json.Marshal(DevicePosture{DeviceID: id, OSFIPSEnabled: true})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/posture", bytes.NewReader(body))
		w := httptest.NewRecorder()
		pc.HandlePostureReport(w, req)
	}

	// List all
	req := httptest.NewRequest(http.MethodGet, "/api/v1/posture", nil)
	w := httptest.NewRecorder()
	pc.HandlePostureList(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var devices []DevicePosture
	if err := json.NewDecoder(w.Body).Decode(&devices); err != nil {
		t.Fatal(err)
	}
	if len(devices) != 2 {
		t.Errorf("expected 2 devices, got %d", len(devices))
	}
}

func TestPostureCollectorFIPSDeviceStats(t *testing.T) {
	pc := NewPostureCollector()

	// Add 2 FIPS and 1 non-FIPS device
	devices := []DevicePosture{
		{DeviceID: "d1", OSFIPSEnabled: true},
		{DeviceID: "d2", OSFIPSEnabled: true},
		{DeviceID: "d3", OSFIPSEnabled: false},
	}
	for _, d := range devices {
		body, _ := json.Marshal(d)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/posture", bytes.NewReader(body))
		w := httptest.NewRecorder()
		pc.HandlePostureReport(w, req)
	}

	fips, nonFIPS := pc.FIPSDeviceStats()
	if fips != 2 {
		t.Errorf("expected 2 FIPS, got %d", fips)
	}
	if nonFIPS != 1 {
		t.Errorf("expected 1 non-FIPS, got %d", nonFIPS)
	}
}

func TestPostureCollectorGetDeviceNotFound(t *testing.T) {
	pc := NewPostureCollector()
	_, ok := pc.GetDevice("nonexistent")
	if ok {
		t.Error("expected device not found")
	}
}

func TestPostureCollectorUpdatesExistingDevice(t *testing.T) {
	pc := NewPostureCollector()

	// Report device as non-FIPS
	body, _ := json.Marshal(DevicePosture{DeviceID: "d1", OSFIPSEnabled: false})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posture", bytes.NewReader(body))
	w := httptest.NewRecorder()
	pc.HandlePostureReport(w, req)

	// Update same device to FIPS
	body, _ = json.Marshal(DevicePosture{DeviceID: "d1", OSFIPSEnabled: true})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/posture", bytes.NewReader(body))
	w = httptest.NewRecorder()
	pc.HandlePostureReport(w, req)

	dev, ok := pc.GetDevice("d1")
	if !ok {
		t.Fatal("device not found")
	}
	if !dev.OSFIPSEnabled {
		t.Error("device should be FIPS-enabled after update")
	}

	// Should still be 1 device, not 2
	if len(pc.AllDevices()) != 1 {
		t.Errorf("expected 1 device after update, got %d", len(pc.AllDevices()))
	}
}

func TestFIPSSummaryJSON(t *testing.T) {
	s := FIPSSummary{Total: 10, FIPSCapable: 7, NonFIPS: 3}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}

	var decoded FIPSSummary
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded != s {
		t.Errorf("round-trip mismatch: got %+v", decoded)
	}
}
