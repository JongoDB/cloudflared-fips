//go:build !linux

package remediate

import (
	"fmt"
	"runtime"
)

func enableOSFIPS() (output string, needsReboot bool, err error) {
	switch runtime.GOOS {
	case "darwin":
		return "macOS CommonCrypto is always FIPS-validated. No action needed.", false, nil
	case "windows":
		return "", false, fmt.Errorf("Windows FIPS mode must be enabled via Group Policy.\n" +
			"Run: gpedit.msc → Computer Configuration → Windows Settings →\n" +
			"Security Settings → Local Policies → Security Options →\n" +
			"\"System cryptography: Use FIPS compliant algorithms\" → Enabled")
	default:
		return "", false, fmt.Errorf("OS FIPS enablement not supported on %s", runtime.GOOS)
	}
}

func osFIPSInstructions() string {
	switch runtime.GOOS {
	case "darwin":
		return "macOS: CommonCrypto is always FIPS-validated. No action required."
	case "windows":
		return "Windows: Enable via Group Policy (gpedit.msc):\n" +
			"  Computer Configuration → Windows Settings → Security Settings →\n" +
			"  Local Policies → Security Options →\n" +
			"  \"System cryptography: Use FIPS compliant algorithms\" → Enabled\n" +
			"Then reboot."
	default:
		return "Consult your OS documentation for FIPS mode enablement."
	}
}

func diskEncInstructions() string {
	switch runtime.GOOS {
	case "darwin":
		return "macOS: System Settings → Privacy & Security → FileVault → Turn On"
	case "windows":
		return "Windows: Enable BitLocker via Control Panel → BitLocker Drive Encryption\n" +
			"Or run: manage-bde -on C: -RecoveryPassword"
	default:
		return "Consult your OS documentation for full-disk encryption."
	}
}
