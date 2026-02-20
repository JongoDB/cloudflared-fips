package manifest

import (
	"encoding/json"
	"fmt"
	"os"
)

// ReadManifest reads and parses a build manifest from the given file path.
func ReadManifest(path string) (*BuildManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var m BuildManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	return &m, nil
}

// WriteManifest serializes and writes a build manifest to the given file path.
func WriteManifest(path string, m *BuildManifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	return nil
}
