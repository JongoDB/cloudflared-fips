package remediate

import (
	"fmt"
	"log"
	"time"

	"github.com/cloudflared-fips/cloudflared-fips/internal/compliance"
)

// Executor plans and executes remediation actions based on compliance failures.
type Executor struct {
	logger *log.Logger
	dryRun bool
}

// NewExecutor creates a new remediation executor.
func NewExecutor(logger *log.Logger) *Executor {
	if logger == nil {
		logger = log.Default()
	}
	return &Executor{logger: logger}
}

// Plan analyzes a compliance section and returns available remediation actions.
// It does not execute anything — call Execute to apply them.
func (e *Executor) Plan(section compliance.Section) []RemediationAction {
	var actions []RemediationAction

	for _, item := range section.Items {
		if item.Status == compliance.StatusPass {
			continue
		}

		switch item.ID {
		case "ag-fips":
			actions = append(actions, RemediationAction{
				ID:           ActionEnableOSFIPS,
				Description:  "Enable OS FIPS mode",
				AutoExec:     true,
				Instructions: osFIPSInstructions(),
				Status:       StatusPending,
			})
		case "ag-warp":
			if item.Remediation == "WARP installed but not connected" ||
				item.Remediation == "WARP found but not on PATH or not running" {
				actions = append(actions, RemediationAction{
					ID:           ActionConnectWARP,
					Description:  "Connect Cloudflare WARP",
					AutoExec:     true,
					Instructions: "Run: warp-cli connect",
					Status:       StatusPending,
				})
			} else {
				actions = append(actions, RemediationAction{
					ID:           ActionInstallWARP,
					Description:  "Install Cloudflare WARP client",
					AutoExec:     true,
					Instructions: warpInstallInstructions(),
					Status:       StatusPending,
				})
			}
		case "ag-disk":
			actions = append(actions, RemediationAction{
				ID:           ActionEnableDiskEnc,
				Description:  "Enable full-disk encryption",
				AutoExec:     false,
				Instructions: diskEncInstructions(),
				Status:       StatusManualOnly,
			})
		case "ag-mdm":
			actions = append(actions, RemediationAction{
				ID:           ActionEnrollMDM,
				Description:  "Enroll device in MDM",
				AutoExec:     false,
				Instructions: "Contact your IT department to enroll this device in\nMicrosoft Intune or Jamf Pro.",
				Status:       StatusManualOnly,
			})
		}
	}

	return actions
}

// Execute runs the given remediation actions. Manual-only actions are skipped
// with instructions populated. Returns the result with per-action status.
func (e *Executor) Execute(req RemediationRequest, available []RemediationAction) RemediationResult {
	result := RemediationResult{
		RequestID: req.ID,
		NodeID:    req.NodeID,
		DryRun:    req.DryRun,
	}

	// Build action lookup from available
	actionMap := make(map[ActionID]*RemediationAction)
	for i := range available {
		actionMap[available[i].ID] = &available[i]
	}

	for _, actionID := range req.Actions {
		action, ok := actionMap[actionID]
		if !ok {
			result.Actions = append(result.Actions, RemediationAction{
				ID:     actionID,
				Status: StatusSkipped,
				Output: "action not applicable to current compliance state",
			})
			continue
		}

		if !action.AutoExec {
			action.Status = StatusManualOnly
			result.Actions = append(result.Actions, *action)
			continue
		}

		if req.DryRun {
			action.Status = StatusPending
			action.Output = "dry run — would execute"
			result.Actions = append(result.Actions, *action)
			continue
		}

		// Execute the action
		action.Status = StatusExecuting
		e.logger.Printf("remediate: executing %s", actionID)

		var err error
		var output string
		var needsReboot bool

		switch actionID {
		case ActionEnableOSFIPS:
			output, needsReboot, err = enableOSFIPS()
		case ActionInstallWARP:
			output, err = installWARP()
		case ActionConnectWARP:
			output, err = connectWARP()
		default:
			err = fmt.Errorf("no executor for action %s", actionID)
		}

		action.Output = output
		if err != nil {
			action.Status = StatusFailed
			action.Output = fmt.Sprintf("error: %v\n%s", err, output)
			e.logger.Printf("remediate: %s failed: %v", actionID, err)
		} else if needsReboot {
			action.Status = StatusNeedsReboot
			e.logger.Printf("remediate: %s succeeded (reboot required)", actionID)
		} else {
			action.Status = StatusSuccess
			e.logger.Printf("remediate: %s succeeded", actionID)
		}

		result.Actions = append(result.Actions, *action)
	}

	result.CompletedAt = time.Now().UTC()
	return result
}
