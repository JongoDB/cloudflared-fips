package wizard

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/common"
)

var (
	progressStyle = lipgloss.NewStyle().
			Foreground(common.ColorPrimary).
			Bold(true)

	progressDimStyle = lipgloss.NewStyle().
				Foreground(common.ColorMuted)

	navHintStyle = lipgloss.NewStyle().
			Foreground(common.ColorDim)
)

// renderProgress renders "Step N of M — Title" with a simple dot indicator.
func renderProgress(current, total int, title string) string {
	dots := ""
	for i := 1; i <= total; i++ {
		if i == current {
			dots += progressStyle.Render("●")
		} else if i < current {
			dots += common.SuccessStyle.Render("●")
		} else {
			dots += progressDimStyle.Render("○")
		}
		if i < total {
			dots += " "
		}
	}

	step := progressStyle.Render(fmt.Sprintf("Step %d of %d", current, total))
	titleStr := lipgloss.NewStyle().Foreground(common.ColorWhite).Render(" — " + title)

	return dots + "  " + step + titleStr
}

// renderNavHints renders the footer navigation hints.
func renderNavHints(isFirst, isLast bool) string {
	hints := ""
	if !isFirst {
		hints += "Shift+Tab=Back  "
	}
	if isLast {
		hints += "Enter=Review & Provision  "
	} else {
		hints += "Tab/Enter=Next  "
	}
	hints += "Ctrl+C=Quit"
	return navHintStyle.Render(hints)
}
