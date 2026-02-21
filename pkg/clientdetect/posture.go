package clientdetect

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// DevicePosture holds device FIPS status reported by a WARP agent or
// custom endpoint management solution.
type DevicePosture struct {
	DeviceID       string    `json:"device_id"`
	UserEmail      string    `json:"user_email"`
	OSFIPSEnabled  bool      `json:"os_fips_enabled"`
	OSType         string    `json:"os_type"`
	OSVersion      string    `json:"os_version"`
	WARPVersion    string    `json:"warp_version"`
	MDMEnrolled    bool      `json:"mdm_enrolled"`
	DiskEncryption bool      `json:"disk_encryption"`
	ReportedAt     time.Time `json:"reported_at"`
}

// PostureCollector receives and stores device posture reports from
// WARP agents or custom endpoint agents. It exposes an HTTP handler
// that agents POST posture data to.
type PostureCollector struct {
	mu      sync.RWMutex
	devices map[string]DevicePosture // keyed by device_id
}

// NewPostureCollector creates a device posture collector.
func NewPostureCollector() *PostureCollector {
	return &PostureCollector{
		devices: make(map[string]DevicePosture),
	}
}

// HandlePostureReport is an HTTP handler for receiving device posture reports.
// Agents POST JSON to this endpoint.
//
//	POST /api/v1/posture
//	{
//	  "device_id": "...",
//	  "os_fips_enabled": true,
//	  ...
//	}
func (pc *PostureCollector) HandlePostureReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	var posture DevicePosture
	if err := json.NewDecoder(r.Body).Decode(&posture); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %s", err), http.StatusBadRequest)
		return
	}

	if posture.DeviceID == "" {
		http.Error(w, "device_id required", http.StatusBadRequest)
		return
	}

	posture.ReportedAt = time.Now()

	pc.mu.Lock()
	pc.devices[posture.DeviceID] = posture
	pc.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
}

// HandlePostureList is an HTTP handler that returns all known device postures.
func (pc *PostureCollector) HandlePostureList(w http.ResponseWriter, r *http.Request) {
	pc.mu.RLock()
	devices := make([]DevicePosture, 0, len(pc.devices))
	for _, d := range pc.devices {
		devices = append(devices, d)
	}
	pc.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(devices)
}

// GetDevice returns the posture for a specific device, if known.
func (pc *PostureCollector) GetDevice(deviceID string) (DevicePosture, bool) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	d, ok := pc.devices[deviceID]
	return d, ok
}

// AllDevices returns all known device postures.
func (pc *PostureCollector) AllDevices() []DevicePosture {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	devices := make([]DevicePosture, 0, len(pc.devices))
	for _, d := range pc.devices {
		devices = append(devices, d)
	}
	return devices
}

// FIPSDeviceStats returns counts of FIPS-enabled vs non-FIPS devices.
func (pc *PostureCollector) FIPSDeviceStats() (fipsCount, nonFIPSCount int) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	for _, d := range pc.devices {
		if d.OSFIPSEnabled {
			fipsCount++
		} else {
			nonFIPSCount++
		}
	}
	return
}
