//go:build linux

package remediate

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func enableOSFIPS() (output string, needsReboot bool, err error) {
	// Already enabled?
	data, readErr := os.ReadFile("/proc/sys/crypto/fips_enabled")
	if readErr == nil && strings.TrimSpace(string(data)) == "1" {
		return "OS FIPS mode is already enabled", false, nil
	}

	// Try fips-mode-setup (RHEL, CentOS, Alma, Oracle, Amazon Linux)
	if path, lookErr := exec.LookPath("fips-mode-setup"); lookErr == nil {
		out, cmdErr := exec.Command(path, "--enable").CombinedOutput()
		if cmdErr != nil {
			return string(out), false, fmt.Errorf("fips-mode-setup --enable failed: %w", cmdErr)
		}
		return string(out) + "\nFIPS mode enabled. A reboot is required to activate.", true, nil
	}

	// Try ua enable fips (Ubuntu Pro)
	if path, lookErr := exec.LookPath("ua"); lookErr == nil {
		out, cmdErr := exec.Command(path, "enable", "fips").CombinedOutput()
		if cmdErr != nil {
			return string(out), false, fmt.Errorf("ua enable fips failed: %w", cmdErr)
		}
		return string(out) + "\nFIPS mode enabled via Ubuntu Pro. A reboot is required.", true, nil
	}

	// Try pro enable fips (newer Ubuntu Pro CLI)
	if path, lookErr := exec.LookPath("pro"); lookErr == nil {
		out, cmdErr := exec.Command(path, "enable", "fips").CombinedOutput()
		if cmdErr != nil {
			return string(out), false, fmt.Errorf("pro enable fips failed: %w", cmdErr)
		}
		return string(out) + "\nFIPS mode enabled via Ubuntu Pro. A reboot is required.", true, nil
	}

	return "", false, fmt.Errorf("no FIPS enablement tool found (need fips-mode-setup or ua/pro)")
}

func osFIPSInstructions() string {
	return "RHEL/CentOS/Alma: sudo fips-mode-setup --enable && sudo reboot\n" +
		"Ubuntu Pro: sudo pro enable fips && sudo reboot\n" +
		"Amazon Linux: sudo fips-mode-setup --enable && sudo reboot"
}

func diskEncInstructions() string {
	return "Linux LUKS encryption cannot be enabled on a running system.\n" +
		"Reinstall with encryption enabled, or use cryptsetup on\n" +
		"unmounted partitions. See: man cryptsetup-luksFormat"
}
