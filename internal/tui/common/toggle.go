package common

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Toggle is a boolean [x]/[ ] toggle component.
type Toggle struct {
	Label    string
	Hint     string
	HelpText string // Contextual help shown when focused
	Enabled  bool
	Focused  bool
}

// NewToggle creates a new toggle component.
func NewToggle(label, hint string, defaultOn bool) Toggle {
	return Toggle{
		Label:   label,
		Hint:    hint,
		Enabled: defaultOn,
	}
}

// Focus sets this toggle as focused.
func (t *Toggle) Focus() {
	t.Focused = true
}

// Blur removes focus.
func (t *Toggle) Blur() {
	t.Focused = false
}

// Update handles key events — space or enter to toggle.
func (t *Toggle) Update(msg tea.Msg) {
	if !t.Focused {
		return
	}
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case " ", "enter":
			t.Enabled = !t.Enabled
		}
	}
}

// View renders the toggle.
func (t *Toggle) View() string {
	check := lipgloss.NewStyle().Foreground(ColorMuted).Render("[ ]")
	if t.Enabled {
		check = lipgloss.NewStyle().Bold(true).Foreground(ColorSuccess).Render("[✓]")
	}

	label := t.Label
	if t.Focused {
		label = lipgloss.NewStyle().Bold(true).Foreground(ColorWhite).Render(label)
	} else {
		label = MutedStyle.Render(label)
	}

	hint := ""
	if t.Hint != "" {
		hint = "  " + HintStyle.Render(t.Hint)
	}

	cursor := "  "
	if t.Focused {
		cursor = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render("▸ ")
	}

	result := cursor + check + " " + label + hint
	if t.HelpText != "" && t.Focused {
		result += "\n" + HelpTextStyle.Render(t.HelpText)
	}
	return result
}
