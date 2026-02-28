package clientdetect

import (
	"testing"

	"github.com/cloudflared-fips/cloudflared-fips/internal/compliance"
)

func TestNewComplianceChecker(t *testing.T) {
	inspector := NewInspector(100)
	posture := NewPostureCollector()
	cc := NewComplianceChecker(inspector, posture)
	if cc == nil {
		t.Fatal("NewComplianceChecker returned nil")
	}
}

func TestRunClientPostureChecks_Structure(t *testing.T) {
	inspector := NewInspector(100)
	posture := NewPostureCollector()
	cc := NewComplianceChecker(inspector, posture)

	section := cc.RunClientPostureChecks()

	if section.ID != "client" {
		t.Errorf("section ID = %q, want client", section.ID)
	}
	if section.Name != "Client Posture" {
		t.Errorf("section Name = %q, want Client Posture", section.Name)
	}
	if len(section.Items) != 8 {
		t.Errorf("expected 8 items, got %d", len(section.Items))
	}

	expectedIDs := []string{"cp-1", "cp-2", "cp-3", "cp-4", "cp-5", "cp-6", "cp-7", "cp-8"}
	for i, id := range expectedIDs {
		if i < len(section.Items) && section.Items[i].ID != id {
			t.Errorf("item %d: ID = %q, want %q", i, section.Items[i].ID, id)
		}
	}
}

func TestRunClientPostureChecks_NoDevicesNoClients(t *testing.T) {
	inspector := NewInspector(100)
	posture := NewPostureCollector()
	cc := NewComplianceChecker(inspector, posture)

	section := cc.RunClientPostureChecks()

	// With no devices or clients, most checks should return warning
	for _, item := range section.Items {
		if item.Status == compliance.StatusFail {
			t.Errorf("item %s should not fail with no data, got fail", item.ID)
		}
	}
}

// ---------------------------------------------------------------------------
// checkClientOSFIPS with posture data
// ---------------------------------------------------------------------------

func TestCheckClientOSFIPS_AllFIPS(t *testing.T) {
	inspector := NewInspector(100)
	posture := NewPostureCollector()
	posture.devices["dev-1"] = DevicePosture{DeviceID: "dev-1", OSFIPSEnabled: true}
	posture.devices["dev-2"] = DevicePosture{DeviceID: "dev-2", OSFIPSEnabled: true}

	cc := NewComplianceChecker(inspector, posture)
	item := cc.checkClientOSFIPS()

	if item.Status != compliance.StatusPass {
		t.Errorf("status = %q, want pass (all FIPS)", item.Status)
	}
}

func TestCheckClientOSFIPS_SomeNonFIPS(t *testing.T) {
	inspector := NewInspector(100)
	posture := NewPostureCollector()
	posture.devices["dev-1"] = DevicePosture{DeviceID: "dev-1", OSFIPSEnabled: true}
	posture.devices["dev-2"] = DevicePosture{DeviceID: "dev-2", OSFIPSEnabled: false}

	cc := NewComplianceChecker(inspector, posture)
	item := cc.checkClientOSFIPS()

	if item.Status != compliance.StatusWarning {
		t.Errorf("status = %q, want warning (mixed FIPS)", item.Status)
	}
}

func TestCheckClientOSFIPS_NoDevices(t *testing.T) {
	inspector := NewInspector(100)
	posture := NewPostureCollector()
	cc := NewComplianceChecker(inspector, posture)

	item := cc.checkClientOSFIPS()
	if item.Status != compliance.StatusWarning {
		t.Errorf("status = %q, want warning (no devices)", item.Status)
	}
}

// ---------------------------------------------------------------------------
// checkClientOSType
// ---------------------------------------------------------------------------

func TestCheckClientOSType_WithDevices(t *testing.T) {
	inspector := NewInspector(100)
	posture := NewPostureCollector()
	posture.devices["dev-1"] = DevicePosture{DeviceID: "dev-1", OSType: "linux"}

	cc := NewComplianceChecker(inspector, posture)
	item := cc.checkClientOSType()

	if item.Status != compliance.StatusPass {
		t.Errorf("status = %q, want pass", item.Status)
	}
}

func TestCheckClientOSType_NoDevices(t *testing.T) {
	inspector := NewInspector(100)
	posture := NewPostureCollector()
	cc := NewComplianceChecker(inspector, posture)

	item := cc.checkClientOSType()
	if item.Status != compliance.StatusWarning {
		t.Errorf("status = %q, want warning", item.Status)
	}
}

// ---------------------------------------------------------------------------
// checkBrowserTLS with inspector data
// ---------------------------------------------------------------------------

func TestCheckBrowserTLS_NoClients(t *testing.T) {
	inspector := NewInspector(100)
	posture := NewPostureCollector()
	cc := NewComplianceChecker(inspector, posture)

	item := cc.checkBrowserTLS()
	if item.Status != compliance.StatusWarning {
		t.Errorf("status = %q, want warning (no clients)", item.Status)
	}
}

// ---------------------------------------------------------------------------
// checkNegotiatedCipher
// ---------------------------------------------------------------------------

func TestCheckNegotiatedCipher_NoClients(t *testing.T) {
	inspector := NewInspector(100)
	posture := NewPostureCollector()
	cc := NewComplianceChecker(inspector, posture)

	item := cc.checkNegotiatedCipher()
	if item.Status != compliance.StatusWarning {
		t.Errorf("status = %q, want warning (no clients)", item.Status)
	}
}

// ---------------------------------------------------------------------------
// checkTLSVersion (static pass)
// ---------------------------------------------------------------------------

func TestCheckTLSVersion_AlwaysPass(t *testing.T) {
	inspector := NewInspector(100)
	posture := NewPostureCollector()
	cc := NewComplianceChecker(inspector, posture)

	item := cc.checkTLSVersion()
	if item.Status != compliance.StatusPass {
		t.Errorf("status = %q, want pass", item.Status)
	}
	if item.ID != "cp-5" {
		t.Errorf("ID = %q, want cp-5", item.ID)
	}
}

// ---------------------------------------------------------------------------
// checkDevicePosture
// ---------------------------------------------------------------------------

func TestCheckDevicePosture_WithDevices(t *testing.T) {
	inspector := NewInspector(100)
	posture := NewPostureCollector()
	posture.devices["dev-1"] = DevicePosture{DeviceID: "dev-1"}

	cc := NewComplianceChecker(inspector, posture)
	item := cc.checkDevicePosture()

	if item.Status != compliance.StatusPass {
		t.Errorf("status = %q, want pass", item.Status)
	}
}

func TestCheckDevicePosture_NoDevices(t *testing.T) {
	inspector := NewInspector(100)
	posture := NewPostureCollector()
	cc := NewComplianceChecker(inspector, posture)

	item := cc.checkDevicePosture()
	if item.Status != compliance.StatusWarning {
		t.Errorf("status = %q, want warning", item.Status)
	}
}

// ---------------------------------------------------------------------------
// checkMDMEnrolled
// ---------------------------------------------------------------------------

func TestCheckMDMEnrolled_AllEnrolled(t *testing.T) {
	inspector := NewInspector(100)
	posture := NewPostureCollector()
	posture.devices["dev-1"] = DevicePosture{DeviceID: "dev-1", MDMEnrolled: true}
	posture.devices["dev-2"] = DevicePosture{DeviceID: "dev-2", MDMEnrolled: true}

	cc := NewComplianceChecker(inspector, posture)
	item := cc.checkMDMEnrolled()

	if item.Status != compliance.StatusPass {
		t.Errorf("status = %q, want pass", item.Status)
	}
}

func TestCheckMDMEnrolled_SomeNotEnrolled(t *testing.T) {
	inspector := NewInspector(100)
	posture := NewPostureCollector()
	posture.devices["dev-1"] = DevicePosture{DeviceID: "dev-1", MDMEnrolled: true}
	posture.devices["dev-2"] = DevicePosture{DeviceID: "dev-2", MDMEnrolled: false}

	cc := NewComplianceChecker(inspector, posture)
	item := cc.checkMDMEnrolled()

	if item.Status != compliance.StatusWarning {
		t.Errorf("status = %q, want warning (partial MDM)", item.Status)
	}
}

func TestCheckMDMEnrolled_NoDevices(t *testing.T) {
	inspector := NewInspector(100)
	posture := NewPostureCollector()
	cc := NewComplianceChecker(inspector, posture)

	item := cc.checkMDMEnrolled()
	if item.Status != compliance.StatusWarning {
		t.Errorf("status = %q, want warning (no devices)", item.Status)
	}
}

// ---------------------------------------------------------------------------
// checkClientCertificate (static warning)
// ---------------------------------------------------------------------------

func TestCheckClientCertificate_AlwaysWarning(t *testing.T) {
	inspector := NewInspector(100)
	posture := NewPostureCollector()
	cc := NewComplianceChecker(inspector, posture)

	item := cc.checkClientCertificate()
	if item.Status != compliance.StatusWarning {
		t.Errorf("status = %q, want warning (mTLS not configured)", item.Status)
	}
	if item.ID != "cp-8" {
		t.Errorf("ID = %q, want cp-8", item.ID)
	}
}
