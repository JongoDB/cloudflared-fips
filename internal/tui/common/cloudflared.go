package common

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// CLITunnel represents a tunnel returned by cloudflared tunnel list.
type CLITunnel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

var uuidFromOutput = regexp.MustCompile(`([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})`)

// DetectCloudflared returns the path to cloudflared if found on PATH.
func DetectCloudflared() (string, error) {
	return exec.LookPath("cloudflared")
}

// ErrNotLoggedIn is returned when cloudflared has no origin certificate.
// The user must run "cloudflared login" first.
var ErrNotLoggedIn = fmt.Errorf("not logged in — run \"cloudflared login\" first")

// isNotLoggedIn checks whether cloudflared output indicates a missing cert.pem.
func isNotLoggedIn(output string) bool {
	return strings.Contains(output, "origincert") ||
		strings.Contains(output, "origin cert") ||
		strings.Contains(output, "cert.pem")
}

// ListTunnelsCLI runs cloudflared tunnel list and parses the JSON output.
func ListTunnelsCLI() ([]CLITunnel, error) {
	path, err := DetectCloudflared()
	if err != nil {
		return nil, fmt.Errorf("cloudflared not found: %w", err)
	}

	out, err := exec.Command(path, "tunnel", "list", "--output", "json").CombinedOutput()
	if err != nil {
		if isNotLoggedIn(string(out)) {
			return nil, ErrNotLoggedIn
		}
		return nil, fmt.Errorf("cloudflared tunnel list failed: %w", err)
	}

	var tunnels []CLITunnel
	if err := json.Unmarshal(out, &tunnels); err != nil {
		return nil, fmt.Errorf("parse tunnel list: %w", err)
	}
	return tunnels, nil
}

// CreateTunnel runs cloudflared tunnel create and returns the UUID and
// credentials file path. The credentials file is expected at
// ~/.cloudflared/<uuid>.json.
func CreateTunnel(name string) (uuid, credsPath string, err error) {
	path, err := DetectCloudflared()
	if err != nil {
		return "", "", fmt.Errorf("cloudflared not found: %w", err)
	}

	out, err := exec.Command(path, "tunnel", "create", name).CombinedOutput()
	if err != nil {
		if isNotLoggedIn(string(out)) {
			return "", "", ErrNotLoggedIn
		}
		return "", "", fmt.Errorf("tunnel create failed: %s", firstLine(string(out)))
	}

	// Parse UUID from output (e.g. "Created tunnel <name> with id <uuid>")
	match := uuidFromOutput.FindString(string(out))
	if match == "" {
		return "", "", fmt.Errorf("could not parse tunnel UUID from output: %s", firstLine(string(out)))
	}

	credsPath = FindCredentialsFile(match)
	return match, credsPath, nil
}

// firstLine returns the first non-empty line of s, trimmed.
func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return strings.TrimSpace(s)
}

// FindCredentialsFile returns the default credentials file path for a tunnel
// UUID. Returns empty string if the file does not exist.
func FindCredentialsFile(uuid string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	p := filepath.Join(home, ".cloudflared", uuid+".json")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return ""
}
