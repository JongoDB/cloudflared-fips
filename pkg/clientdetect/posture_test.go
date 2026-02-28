package clientdetect

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPostureCollector_InitialState(t *testing.T) {
	pc := NewPostureCollector()
	if pc == nil {
		t.Fatal("NewPostureCollector returned nil")
	}
	if len(pc.AllDevices()) != 0 {
		t.Errorf("new collector should have 0 devices, got %d", len(pc.AllDevices()))
	}
}

func TestHandlePostureReport_ValidPost(t *testing.T) {
	pc := NewPostureCollector()

	body := `{"device_id":"dev-1","os_fips_enabled":true,"os_type":"linux","mdm_enrolled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posture", strings.NewReader(body))
	w := httptest.NewRecorder()

	pc.HandlePostureReport(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	devices := pc.AllDevices()
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
	if devices[0].DeviceID != "dev-1" {
		t.Errorf("device_id = %q, want dev-1", devices[0].DeviceID)
	}
	if !devices[0].OSFIPSEnabled {
		t.Error("os_fips_enabled = false, want true")
	}
	if !devices[0].MDMEnrolled {
		t.Error("mdm_enrolled = false, want true")
	}
}

func TestHandlePostureReport_MissingDeviceID(t *testing.T) {
	pc := NewPostureCollector()

	body := `{"os_fips_enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posture", strings.NewReader(body))
	w := httptest.NewRecorder()

	pc.HandlePostureReport(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandlePostureReport_InvalidJSON(t *testing.T) {
	pc := NewPostureCollector()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/posture", strings.NewReader("{invalid"))
	w := httptest.NewRecorder()

	pc.HandlePostureReport(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandlePostureReport_WrongMethod(t *testing.T) {
	pc := NewPostureCollector()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/posture", nil)
	w := httptest.NewRecorder()

	pc.HandlePostureReport(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandlePostureReport_UpdatesExistingDevice(t *testing.T) {
	pc := NewPostureCollector()

	body1 := `{"device_id":"dev-1","os_fips_enabled":false}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posture", strings.NewReader(body1))
	w := httptest.NewRecorder()
	pc.HandlePostureReport(w, req)

	body2 := `{"device_id":"dev-1","os_fips_enabled":true}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/posture", strings.NewReader(body2))
	w = httptest.NewRecorder()
	pc.HandlePostureReport(w, req)

	devices := pc.AllDevices()
	if len(devices) != 1 {
		t.Fatalf("expected 1 device (updated), got %d", len(devices))
	}
	if !devices[0].OSFIPSEnabled {
		t.Error("device should be updated to FIPS enabled")
	}
}

func TestHandlePostureList(t *testing.T) {
	pc := NewPostureCollector()
	pc.devices["dev-1"] = DevicePosture{DeviceID: "dev-1", OSType: "linux"}
	pc.devices["dev-2"] = DevicePosture{DeviceID: "dev-2", OSType: "windows"}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/posture", nil)
	w := httptest.NewRecorder()

	pc.HandlePostureList(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var devices []DevicePosture
	if err := json.NewDecoder(w.Body).Decode(&devices); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(devices) != 2 {
		t.Errorf("expected 2 devices, got %d", len(devices))
	}
}

func TestGetDevice(t *testing.T) {
	pc := NewPostureCollector()
	pc.devices["dev-1"] = DevicePosture{DeviceID: "dev-1", OSType: "linux"}

	d, ok := pc.GetDevice("dev-1")
	if !ok {
		t.Fatal("GetDevice(dev-1) not found")
	}
	if d.OSType != "linux" {
		t.Errorf("OSType = %q, want linux", d.OSType)
	}

	_, ok = pc.GetDevice("nonexistent")
	if ok {
		t.Error("GetDevice(nonexistent) should return false")
	}
}

func TestFIPSDeviceStats(t *testing.T) {
	pc := NewPostureCollector()
	pc.devices["dev-1"] = DevicePosture{DeviceID: "dev-1", OSFIPSEnabled: true}
	pc.devices["dev-2"] = DevicePosture{DeviceID: "dev-2", OSFIPSEnabled: true}
	pc.devices["dev-3"] = DevicePosture{DeviceID: "dev-3", OSFIPSEnabled: false}

	fips, nonFIPS := pc.FIPSDeviceStats()
	if fips != 2 {
		t.Errorf("fips = %d, want 2", fips)
	}
	if nonFIPS != 1 {
		t.Errorf("nonFIPS = %d, want 1", nonFIPS)
	}
}

func TestFIPSDeviceStats_Empty(t *testing.T) {
	pc := NewPostureCollector()

	fips, nonFIPS := pc.FIPSDeviceStats()
	if fips != 0 || nonFIPS != 0 {
		t.Errorf("empty collector: fips=%d, nonFIPS=%d, want 0,0", fips, nonFIPS)
	}
}
