package fleet

import (
	"testing"

	"github.com/cloudflared-fips/cloudflared-fips/internal/compliance"
)

func TestNewAgentChecks(t *testing.T) {
	a := NewAgentChecks()
	if a == nil {
		t.Fatal("NewAgentChecks() returned nil")
	}
}

func TestAgentChecks_RunChecks_Structure(t *testing.T) {
	a := NewAgentChecks()
	section := a.RunChecks()

	if section.ID != "agent-posture" {
		t.Errorf("section ID = %q, want agent-posture", section.ID)
	}
	if section.Name != "Endpoint FIPS Posture" {
		t.Errorf("section Name = %q, want Endpoint FIPS Posture", section.Name)
	}
	if len(section.Items) != 6 {
		t.Errorf("expected 6 items, got %d", len(section.Items))
	}

	expectedIDs := []string{"ag-fips", "ag-os", "ag-disk", "ag-mdm", "ag-warp", "ag-tls"}
	for i, id := range expectedIDs {
		if i < len(section.Items) && section.Items[i].ID != id {
			t.Errorf("item %d: ID = %q, want %q", i, section.Items[i].ID, id)
		}
	}
}

func TestAgentChecks_AllItemsHaveValidStatus(t *testing.T) {
	a := NewAgentChecks()
	section := a.RunChecks()

	validStatuses := map[compliance.Status]bool{
		compliance.StatusPass:    true,
		compliance.StatusFail:    true,
		compliance.StatusWarning: true,
		compliance.StatusUnknown: true,
	}

	for _, item := range section.Items {
		if !validStatuses[item.Status] {
			t.Errorf("item %s (%s): invalid status %q", item.ID, item.Name, item.Status)
		}
	}
}

func TestAgentChecks_OSType_AlwaysPass(t *testing.T) {
	a := NewAgentChecks()
	item := a.checkOSType()
	if item.ID != "ag-os" {
		t.Errorf("ID = %q, want ag-os", item.ID)
	}
	if item.Status != compliance.StatusPass {
		t.Errorf("OS type status = %q, want pass", item.Status)
	}
}

func TestAgentChecks_TLSCapabilities_AlwaysPass(t *testing.T) {
	a := NewAgentChecks()
	item := a.checkTLSCapabilities()
	if item.ID != "ag-tls" {
		t.Errorf("ID = %q, want ag-tls", item.ID)
	}
	if item.Status != compliance.StatusPass {
		t.Errorf("TLS capabilities status = %q, want pass", item.Status)
	}
}

func TestAgentChecks_OSSFIPSMode_ProducesValidStatus(t *testing.T) {
	a := NewAgentChecks()
	item := a.checkOSFIPSMode()
	if item.ID != "ag-fips" {
		t.Errorf("ID = %q, want ag-fips", item.ID)
	}
	if item.Severity != "critical" {
		t.Errorf("severity = %q, want critical", item.Severity)
	}
	// Status varies by OS — just ensure it's valid
	validStatuses := map[compliance.Status]bool{
		compliance.StatusPass:    true,
		compliance.StatusFail:    true,
		compliance.StatusUnknown: true,
	}
	if !validStatuses[item.Status] {
		t.Errorf("OS FIPS mode status = %q, not a valid status", item.Status)
	}
}

func TestAgentChecks_DiskEncryption_ProducesValidStatus(t *testing.T) {
	a := NewAgentChecks()
	item := a.checkDiskEncryption()
	if item.ID != "ag-disk" {
		t.Errorf("ID = %q, want ag-disk", item.ID)
	}
	if item.Severity != "high" {
		t.Errorf("severity = %q, want high", item.Severity)
	}
}

func TestAgentChecks_WARPInstalled_ProducesValidStatus(t *testing.T) {
	a := NewAgentChecks()
	item := a.checkWARPInstalled()
	if item.ID != "ag-warp" {
		t.Errorf("ID = %q, want ag-warp", item.ID)
	}
	// In dev environment, WARP is typically not installed
	validStatuses := map[compliance.Status]bool{
		compliance.StatusPass:    true,
		compliance.StatusWarning: true,
		compliance.StatusUnknown: true,
	}
	if !validStatuses[item.Status] {
		t.Errorf("WARP status = %q, not a valid status", item.Status)
	}
}

func TestAgentChecks_MDMEnrollment_ProducesValidStatus(t *testing.T) {
	a := NewAgentChecks()
	item := a.checkMDMEnrollment()
	if item.ID != "ag-mdm" {
		t.Errorf("ID = %q, want ag-mdm", item.ID)
	}
	if item.Severity != "medium" {
		t.Errorf("severity = %q, want medium", item.Severity)
	}
}
