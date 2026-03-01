package remediate

import (
	"fmt"
	"os/exec"
	"runtime"
)

func installWARP() (string, error) {
	switch runtime.GOOS {
	case "linux":
		return installWARPLinux()
	case "darwin":
		return "", fmt.Errorf("install WARP on macOS:\n" +
			"  Download from https://1.1.1.1 or install via:\n" +
			"  brew install cloudflare-warp")
	case "windows":
		return "", fmt.Errorf("install WARP on Windows:\n" +
			"  Download from https://1.1.1.1 or use:\n" +
			"  winget install Cloudflare.Warp")
	default:
		return "", fmt.Errorf("WARP installation not supported on %s", runtime.GOOS)
	}
}

func installWARPLinux() (string, error) {
	// Try apt (Debian/Ubuntu)
	if _, err := exec.LookPath("apt-get"); err == nil {
		// Add Cloudflare GPG key and repo
		cmds := []struct {
			name string
			args []string
		}{
			{"curl", []string{"-fsSL", "https://pkg.cloudflareclient.com/pubkey.gpg", "-o", "/usr/share/keyrings/cloudflare-warp-archive-keyring.gpg"}},
			{"bash", []string{"-c", `echo "deb [signed-by=/usr/share/keyrings/cloudflare-warp-archive-keyring.gpg] https://pkg.cloudflareclient.com/ $(lsb_release -cs) main" > /etc/apt/sources.list.d/cloudflare-client.list`}},
			{"apt-get", []string{"update"}},
			{"apt-get", []string{"install", "-y", "cloudflare-warp"}},
		}
		var allOutput string
		for _, cmd := range cmds {
			out, err := exec.Command(cmd.name, cmd.args...).CombinedOutput()
			allOutput += string(out) + "\n"
			if err != nil {
				return allOutput, fmt.Errorf("%s failed: %w", cmd.name, err)
			}
		}
		return allOutput, nil
	}

	// Try yum/dnf (RHEL/CentOS)
	pkgMgr := "yum"
	if _, err := exec.LookPath("dnf"); err == nil {
		pkgMgr = "dnf"
	}
	if _, err := exec.LookPath(pkgMgr); err == nil {
		cmds := []struct {
			name string
			args []string
		}{
			{"rpm", []string{"-ivh", "https://pkg.cloudflareclient.com/cloudflare-release-el8.rpm"}},
			{pkgMgr, []string{"install", "-y", "cloudflare-warp"}},
		}
		var allOutput string
		for _, cmd := range cmds {
			out, err := exec.Command(cmd.name, cmd.args...).CombinedOutput()
			allOutput += string(out) + "\n"
			if err != nil {
				return allOutput, fmt.Errorf("%s failed: %w", cmd.name, err)
			}
		}
		return allOutput, nil
	}

	return "", fmt.Errorf("no supported package manager found (need apt-get or yum/dnf)")
}

func connectWARP() (string, error) {
	path, err := exec.LookPath("warp-cli")
	if err != nil {
		return "", fmt.Errorf("warp-cli not found on PATH")
	}

	out, err := exec.Command(path, "connect").CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("warp-cli connect failed: %w", err)
	}
	return string(out), nil
}

func warpInstallInstructions() string {
	switch runtime.GOOS {
	case "linux":
		return "Debian/Ubuntu: curl -fsSL https://pkg.cloudflareclient.com/pubkey.gpg | sudo gpg --dearmor -o /usr/share/keyrings/cloudflare-warp-archive-keyring.gpg && sudo apt install cloudflare-warp\n" +
			"RHEL/CentOS: sudo rpm -ivh https://pkg.cloudflareclient.com/cloudflare-release-el8.rpm && sudo yum install cloudflare-warp"
	case "darwin":
		return "Download from https://1.1.1.1 or: brew install cloudflare-warp"
	case "windows":
		return "Download from https://1.1.1.1 or: winget install Cloudflare.Warp"
	default:
		return "Visit https://1.1.1.1 for installation instructions."
	}
}
