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

// ListTunnelsCLI runs cloudflared tunnel list and parses the JSON output.
func ListTunnelsCLI() ([]CLITunnel, error) {
	path, err := DetectCloudflared()
	if err != nil {
		return nil, fmt.Errorf("cloudflared not found: %w", err)
	}

	out, err := exec.Command(path, "tunnel", "list", "--output", "json").Output()
	if err != nil {
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
		return "", "", fmt.Errorf("cloudflared tunnel create failed: %s", strings.TrimSpace(string(out)))
	}

	// Parse UUID from output (e.g. "Created tunnel <name> with id <uuid>")
	match := uuidFromOutput.FindString(string(out))
	if match == "" {
		return "", "", fmt.Errorf("could not parse tunnel UUID from output: %s", string(out))
	}

	credsPath = FindCredentialsFile(match)
	return match, credsPath, nil
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
