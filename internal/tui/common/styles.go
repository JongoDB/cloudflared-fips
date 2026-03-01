// Package common provides shared TUI components and styling for the
// cloudflared-fips setup wizard and status monitor.
package common

import "github.com/charmbracelet/lipgloss"

// Color palette.
var (
	ColorPrimary   = lipgloss.Color("#4A9EFF") // Cloudflare blue
	ColorSuccess   = lipgloss.Color("#22C55E") // Green
	ColorWarning   = lipgloss.Color("#EAB308") // Yellow
	ColorDanger    = lipgloss.Color("#EF4444") // Red
	ColorMuted     = lipgloss.Color("#6B7280") // Gray
	ColorHighlight = lipgloss.Color("#A78BFA") // Purple
	ColorWhite     = lipgloss.Color("#F9FAFB")
	ColorDim       = lipgloss.Color("#9CA3AF")
)

// Text styles.
var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			MarginBottom(1)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ColorDim).
			Italic(true)

	LabelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorWhite)

	HintStyle = lipgloss.NewStyle().
			Foreground(ColorDim).
			Italic(true)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorDanger)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	WarningStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	MutedStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)
)

// Layout styles.
var (
	PageStyle = lipgloss.NewStyle().
			Padding(1, 2)

	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(ColorMuted).
			MarginBottom(1).
			PaddingBottom(1)

	FooterStyle = lipgloss.NewStyle().
			Foreground(ColorDim).
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(ColorMuted).
			MarginTop(1).
			PaddingTop(1)

	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorMuted).
			Padding(0, 1)

	SelectedBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary).
				Padding(0, 1)
)

// Input prompt styles — show cursor indicator only when focused.
var (
	FocusedPrompt = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render("▸ ")
	BlurredPrompt = "  "
)

// Badge styles.
var (
	PassBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorSuccess)

	FailBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorDanger)

	WarnBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorWarning)

	UnknownBadge = lipgloss.NewStyle().
			Foreground(ColorMuted)
)

// ProgressBar renders a simple text progress bar.
func ProgressBar(current, total, width int) string {
	if total == 0 {
		return ""
	}
	filled := (current * width) / total
	if filled > width {
		filled = width
	}
	bar := ""
	for i := 0; i < width; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}
	return bar
}
