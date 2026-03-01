package wizard

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

// fieldNavMsg is a no-op message returned when the page handles Tab/Enter
// internally (field-to-field navigation). The wizard uses non-nil cmd to
// distinguish "page handled it" from "page is done, advance wizard".
type fieldNavMsg struct{}

func fieldNav() tea.Msg { return fieldNavMsg{} }

// parseInt parses a non-negative integer from a string.
// Returns 0 for empty or non-numeric input.
func parseInt(s string) int {
	var n int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			return 0
		}
	}
	return n
}

// defaultHostname returns the short hostname, or "node" as fallback.
func defaultHostname() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "node"
	}
	return h
}

// tierNumber converts a deployment tier string to its numeric form.
func tierNumber(tier string) string {
	switch tier {
	case "regional_keyless":
		return "2"
	case "self_hosted":
		return "3"
	default:
		return "1"
	}
}
