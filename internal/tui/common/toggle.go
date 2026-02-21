package common

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Toggle is a boolean [x]/[ ] toggle component.
type Toggle struct {
	Label   string
	Hint    string
	Enabled bool
	Focused bool
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
	check := "[ ]"
	if t.Enabled {
		check = SuccessStyle.Render("[x]")
	}

	label := t.Label
	if t.Focused {
		label = LabelStyle.Render(label)
	} else {
		label = MutedStyle.Render(label)
	}

	hint := ""
	if t.Hint != "" {
		hint = "  " + HintStyle.Render(t.Hint)
	}

	cursor := "  "
	if t.Focused {
		cursor = "> "
	}

	return cursor + check + " " + label + hint
}
