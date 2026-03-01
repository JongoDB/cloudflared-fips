package config

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
)

var (
	uuidRe  = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	hexIDRe = regexp.MustCompile(`^[0-9a-fA-F]{32}$`)
)

// ValidateUUID checks that s is a valid UUID (8-4-4-4-12 hex).
func ValidateUUID(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("UUID is required")
	}
	if !uuidRe.MatchString(s) {
		return fmt.Errorf("invalid UUID format (expected xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx)")
	}
	return nil
}

// ValidateHexID checks that s is a 32-character hex string (Cloudflare zone/account ID).
func ValidateHexID(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("ID is required")
	}
	if !hexIDRe.MatchString(s) {
		return fmt.Errorf("invalid ID format (expected 32-character hex string)")
	}
	return nil
}

// ValidateFileExists checks that the path points to an existing file.
func ValidateFileExists(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("file path is required")
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s", path)
		}
		return fmt.Errorf("cannot access file: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("path is a directory, not a file: %s", path)
	}
	return nil
}

// ValidateHostPort checks that s is a valid host:port address.
func ValidateHostPort(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("address is required")
	}
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		return fmt.Errorf("invalid address (expected host:port): %w", err)
	}
	if host == "" {
		return fmt.Errorf("host cannot be empty")
	}
	if port == "" {
		return fmt.Errorf("port cannot be empty")
	}
	return nil
}

// ValidateNonEmpty checks that s is not empty after trimming whitespace.
func ValidateNonEmpty(s string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("value is required")
	}
	return nil
}

// ValidatePort checks that s is a valid port number (1-65535).
func ValidatePort(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("port is required")
	}
	port := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return fmt.Errorf("port must be a number")
		}
		port = port*10 + int(c-'0')
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return nil
}

// ValidateOptionalHexID checks a hex ID only if non-empty.
func ValidateOptionalHexID(s string) error {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return ValidateHexID(s)
}

// ValidateOptionalHostPort checks a host:port only if non-empty.
func ValidateOptionalHostPort(s string) error {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return ValidateHostPort(s)
}
