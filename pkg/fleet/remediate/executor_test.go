package remediate

import (
	"log"
	"testing"

	"github.com/cloudflared-fips/cloudflared-fips/internal/compliance"
)

func TestIsAutoRemediable(t *testing.T) {
	tests := []struct {
		id   ActionID
		want bool
	}{
		{ActionEnableOSFIPS, true},
		{ActionInstallWARP, true},
		{ActionConnectWARP, true},
		{ActionFixEdgeCiphers, true},
		{ActionFixMinTLS, true},
		{ActionEnableHSTS, true},
		{ActionEnableDiskEnc, false},
		{ActionEnrollMDM, false},
		{ActionID("unknown"), false},
	}
	for _, tt := range tests {
		if got := IsAutoRemediable(tt.id); got != tt.want {
			t.Errorf("IsAutoRemediable(%q) = %v, want %v", tt.id, got, tt.want)
		}
	}
}

func TestPlanNoFailures(t *testing.T) {
	section := compliance.Section{
		ID:   "test",
		Name: "Test",
		Items: []compliance.ChecklistItem{
			{ID: "ag-fips", Status: compliance.StatusPass},
			{ID: "ag-warp", Status: compliance.StatusPass},
		},
	}

	exec := NewExecutor(log.Default())
	actions := exec.Plan(section)
	if len(actions) != 0 {
		t.Errorf("Plan with all passing items should return 0 actions, got %d", len(actions))
	}
}

func TestPlanWithFailures(t *testing.T) {
	section := compliance.Section{
		ID:   "test",
		Name: "Test",
		Items: []compliance.ChecklistItem{
			{ID: "ag-fips", Status: compliance.StatusFail},
			{ID: "ag-warp", Status: compliance.StatusWarning, Remediation: "WARP installed but not connected"},
			{ID: "ag-disk", Status: compliance.StatusWarning},
			{ID: "ag-mdm", Status: compliance.StatusWarning},
		},
	}

	exec := NewExecutor(log.Default())
	actions := exec.Plan(section)

	if len(actions) != 4 {
		t.Fatalf("expected 4 actions, got %d", len(actions))
	}

	// Check FIPS action
	if actions[0].ID != ActionEnableOSFIPS {
		t.Errorf("expected first action to be enable_os_fips, got %s", actions[0].ID)
	}
	if !actions[0].AutoExec {
		t.Error("enable_os_fips should be auto-executable")
	}

	// Check WARP action (should be connect, not install)
	if actions[1].ID != ActionConnectWARP {
		t.Errorf("expected second action to be connect_warp, got %s", actions[1].ID)
	}

	// Check disk encryption (manual only)
	if actions[2].ID != ActionEnableDiskEnc {
		t.Errorf("expected third action to be enable_disk_encryption, got %s", actions[2].ID)
	}
	if actions[2].AutoExec {
		t.Error("enable_disk_encryption should not be auto-executable")
	}
	if actions[2].Status != StatusManualOnly {
		t.Errorf("expected manual_only status, got %s", actions[2].Status)
	}

	// Check MDM (manual only)
	if actions[3].ID != ActionEnrollMDM {
		t.Errorf("expected fourth action to be enroll_mdm, got %s", actions[3].ID)
	}
	if actions[3].Status != StatusManualOnly {
		t.Errorf("expected manual_only status, got %s", actions[3].Status)
	}
}

func TestExecuteDryRun(t *testing.T) {
	available := []RemediationAction{
		{ID: ActionEnableOSFIPS, AutoExec: true, Status: StatusPending},
		{ID: ActionEnableDiskEnc, AutoExec: false, Status: StatusManualOnly,
			Instructions: "Enable encryption"},
	}

	exec := NewExecutor(log.Default())
	req := RemediationRequest{
		ID:      "test-req-1",
		NodeID:  "node-1",
		Actions: []ActionID{ActionEnableOSFIPS, ActionEnableDiskEnc},
		DryRun:  true,
	}

	result := exec.Execute(req, available)

	if result.RequestID != "test-req-1" {
		t.Errorf("expected request ID test-req-1, got %s", result.RequestID)
	}
	if !result.DryRun {
		t.Error("expected dry run to be true")
	}
	if len(result.Actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(result.Actions))
	}

	// Auto-exec action in dry run should be pending
	if result.Actions[0].Status != StatusPending {
		t.Errorf("expected pending status for dry run auto action, got %s", result.Actions[0].Status)
	}
	if result.Actions[0].Output != "dry run — would execute" {
		t.Errorf("unexpected output: %s", result.Actions[0].Output)
	}

	// Manual action should still be manual_only
	if result.Actions[1].Status != StatusManualOnly {
		t.Errorf("expected manual_only status, got %s", result.Actions[1].Status)
	}
}

func TestExecuteUnknownAction(t *testing.T) {
	exec := NewExecutor(log.Default())
	req := RemediationRequest{
		ID:      "test-req-2",
		NodeID:  "node-1",
		Actions: []ActionID{ActionID("nonexistent")},
	}

	result := exec.Execute(req, nil)
	if len(result.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(result.Actions))
	}
	if result.Actions[0].Status != StatusSkipped {
		t.Errorf("expected skipped status, got %s", result.Actions[0].Status)
	}
}

func TestPlanWARPNotInstalled(t *testing.T) {
	section := compliance.Section{
		Items: []compliance.ChecklistItem{
			{ID: "ag-warp", Status: compliance.StatusWarning, Remediation: "Install Cloudflare WARP: https://1.1.1.1"},
		},
	}

	exec := NewExecutor(log.Default())
	actions := exec.Plan(section)

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].ID != ActionInstallWARP {
		t.Errorf("expected install_warp, got %s", actions[0].ID)
	}
}
