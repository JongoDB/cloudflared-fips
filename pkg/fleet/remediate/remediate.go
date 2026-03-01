// Package remediate provides programmatic remediation for fleet compliance failures.
//
// Each remediation action is classified as either auto-remediable (safe to execute
// programmatically) or manual-only (the system can only provide instructions).
// Actions that require a reboot return a needs_reboot status rather than
// automatically rebooting.
package remediate

import "time"

// ActionID identifies a specific remediation action.
type ActionID string

const (
	ActionEnableOSFIPS    ActionID = "enable_os_fips"
	ActionInstallWARP     ActionID = "install_warp"
	ActionConnectWARP     ActionID = "connect_warp"
	ActionFixEdgeCiphers  ActionID = "fix_edge_ciphers"
	ActionFixMinTLS       ActionID = "fix_min_tls"
	ActionEnableHSTS      ActionID = "enable_hsts"
	ActionEnableDiskEnc   ActionID = "enable_disk_encryption"
	ActionEnrollMDM       ActionID = "enroll_mdm"
)

// ActionStatus represents the outcome of a remediation action.
type ActionStatus string

const (
	StatusPending      ActionStatus = "pending"
	StatusExecuting    ActionStatus = "executing"
	StatusSuccess      ActionStatus = "success"
	StatusFailed       ActionStatus = "failed"
	StatusNeedsReboot  ActionStatus = "needs_reboot"
	StatusManualOnly   ActionStatus = "manual_only"
	StatusSkipped      ActionStatus = "skipped"
)

// RemediationAction describes a single fix that can be applied.
type RemediationAction struct {
	ID           ActionID     `json:"id"`
	Description  string       `json:"description"`
	AutoExec     bool         `json:"auto_exec"`      // Safe to execute programmatically
	Instructions string       `json:"instructions"`    // Human-readable steps (always provided)
	Status       ActionStatus `json:"status"`
	Output       string       `json:"output,omitempty"` // Command output or error message
}

// RemediationRequest is sent from the controller to an agent.
type RemediationRequest struct {
	ID      string     `json:"id"`
	NodeID  string     `json:"node_id"`
	Actions []ActionID `json:"actions"`
	DryRun  bool       `json:"dry_run"`
}

// RemediationResult is sent from the agent back to the controller.
type RemediationResult struct {
	RequestID   string              `json:"request_id"`
	NodeID      string              `json:"node_id"`
	Actions     []RemediationAction `json:"actions"`
	CompletedAt time.Time           `json:"completed_at"`
	DryRun      bool                `json:"dry_run"`
}

// IsAutoRemediable returns true if the action can be safely auto-executed.
func IsAutoRemediable(id ActionID) bool {
	switch id {
	case ActionEnableOSFIPS, ActionInstallWARP, ActionConnectWARP,
		ActionFixEdgeCiphers, ActionFixMinTLS, ActionEnableHSTS:
		return true
	case ActionEnableDiskEnc, ActionEnrollMDM:
		return false
	default:
		return false
	}
}
