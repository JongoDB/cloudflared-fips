package common

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SelectorOption is a single choice in a radio-select.
type SelectorOption struct {
	Value       string
	Label       string
	Description string
}

// Selector is a vertical radio-select component with descriptions.
type Selector struct {
	Label    string
	HelpText string // Contextual help shown when focused
	Options  []SelectorOption
	Cursor   int
	Focused  bool
}

// NewSelector creates a new radio-select.
func NewSelector(label string, opts []SelectorOption) Selector {
	return Selector{
		Label:   label,
		Options: opts,
	}
}

// Focus gives focus.
func (s *Selector) Focus() {
	s.Focused = true
}

// Blur removes focus.
func (s *Selector) Blur() {
	s.Focused = false
}

// Selected returns the currently selected option value.
func (s *Selector) Selected() string {
	if s.Cursor < 0 || s.Cursor >= len(s.Options) {
		return ""
	}
	return s.Options[s.Cursor].Value
}

// SelectedIndex returns the cursor position.
func (s *Selector) SelectedIndex() int {
	return s.Cursor
}

// SetSelected sets the cursor to the option matching value.
func (s *Selector) SetSelected(value string) {
	for i, opt := range s.Options {
		if opt.Value == value {
			s.Cursor = i
			return
		}
	}
}

// Update handles key events.
func (s *Selector) Update(msg tea.Msg) {
	if !s.Focused {
		return
	}
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "up", "k":
			if s.Cursor > 0 {
				s.Cursor--
			}
		case "down", "j":
			if s.Cursor < len(s.Options)-1 {
				s.Cursor++
			}
		}
	}
}

// View renders the selector.
func (s *Selector) View() string {
	var b strings.Builder
	b.WriteString(LabelStyle.Render(s.Label))
	b.WriteString("\n")

	for i, opt := range s.Options {
		selected := i == s.Cursor

		cursor := "  "
		radio := lipgloss.NewStyle().Foreground(ColorMuted).Render("○")
		if selected {
			if s.Focused {
				cursor = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render("▸ ")
				radio = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render("●")
			} else {
				cursor = "  "
				radio = lipgloss.NewStyle().Foreground(ColorPrimary).Render("●")
			}
		}

		label := opt.Label
		if selected && s.Focused {
			label = lipgloss.NewStyle().Bold(true).Foreground(ColorWhite).Render(label)
		} else if selected {
			label = lipgloss.NewStyle().Foreground(ColorWhite).Render(label)
		} else {
			label = lipgloss.NewStyle().Foreground(ColorDim).Render(label)
		}

		b.WriteString(cursor + radio + " " + label + "\n")
		if opt.Description != "" {
			descStyle := HintStyle
			if selected && s.Focused {
				descStyle = lipgloss.NewStyle().Foreground(ColorDim)
			}
			desc := descStyle.Render("    " + opt.Description)
			b.WriteString(desc + "\n")
		}
	}
	if s.HelpText != "" && s.Focused {
		b.WriteString(HelpTextStyle.Render(s.HelpText))
		b.WriteString("\n")
	}
	return b.String()
}
