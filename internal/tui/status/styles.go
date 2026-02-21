package status

import "github.com/charmbracelet/lipgloss"

// Color palette matching the web dashboard.
var (
	colorPass    = lipgloss.Color("#22C55E")
	colorWarn    = lipgloss.Color("#EAB308")
	colorFail    = lipgloss.Color("#EF4444")
	colorUnknown = lipgloss.Color("#6B7280")
	colorPrimary = lipgloss.Color("#4A9EFF")
	colorDim     = lipgloss.Color("#9CA3AF")
	colorWhite   = lipgloss.Color("#F9FAFB")
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(colorDim)

	sectionNameStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorWhite).
				MarginTop(1)

	sectionCountStyle = lipgloss.NewStyle().
				Foreground(colorDim)

	footerStyle = lipgloss.NewStyle().
			Foreground(colorDim).
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(colorDim)

	summaryBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorDim).
			Padding(0, 1).
			MarginBottom(1)

	passStyle    = lipgloss.NewStyle().Bold(true).Foreground(colorPass)
	warnStyle    = lipgloss.NewStyle().Bold(true).Foreground(colorWarn)
	failStyle    = lipgloss.NewStyle().Bold(true).Foreground(colorFail)
	unknownStyle = lipgloss.NewStyle().Foreground(colorUnknown)
	dimStyle     = lipgloss.NewStyle().Foreground(colorDim)
)
