package common

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// FindBinary locates a companion binary by checking:
//  1. Same directory as current executable
//  2. System PATH
//
// Returns empty string if not found.
func FindBinary(name string) string {
	exe, err := os.Executable()
	if err == nil {
		candidate := filepath.Join(filepath.Dir(exe), name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	return ""
}

// FindProvisionScript returns the path to the provision script, checking
// installed locations first (RPM/DEB install to /usr/local/bin), then
// PATH, then falling back to relative paths for development.
func FindProvisionScript() string {
	if _, err := os.Stat("/usr/local/bin/cloudflared-fips-provision"); err == nil {
		return "/usr/local/bin/cloudflared-fips-provision"
	}
	if p, err := exec.LookPath("cloudflared-fips-provision"); err == nil {
		return p
	}
	switch runtime.GOOS {
	case "darwin":
		return "./scripts/provision-macos.sh"
	default:
		return "./scripts/provision-linux.sh"
	}
}

// FindUnprovisionScript returns the path to the unprovision script.
func FindUnprovisionScript() string {
	if _, err := os.Stat("/usr/local/bin/cloudflared-fips-unprovision"); err == nil {
		return "/usr/local/bin/cloudflared-fips-unprovision"
	}
	if p, err := exec.LookPath("cloudflared-fips-unprovision"); err == nil {
		return p
	}
	switch runtime.GOOS {
	case "darwin":
		return "./scripts/unprovision-macos.sh"
	default:
		return "./scripts/unprovision-linux.sh"
	}
}
