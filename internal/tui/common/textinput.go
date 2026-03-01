package common

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ValidateFunc validates a string, returning an error message or "".
type ValidateFunc func(string) error

// TextInput is a labeled text input with inline validation.
type TextInput struct {
	Label       string
	Hint        string
	Input       textinput.Model
	Validate    ValidateFunc
	Err         string
	IsMasked    bool
}

// NewTextInput creates a labeled text input.
func NewTextInput(label, placeholder, hint string) TextInput {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.CharLimit = 256
	ti.Width = 50
	ti.Prompt = BlurredPrompt
	ti.TextStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	return TextInput{
		Label: label,
		Hint:  hint,
		Input: ti,
	}
}

// NewPasswordInput creates a masked text input for secrets.
func NewPasswordInput(label, placeholder, hint string) TextInput {
	ti := NewTextInput(label, placeholder, hint)
	ti.Input.EchoMode = textinput.EchoPassword
	ti.Input.EchoCharacter = '*'
	ti.IsMasked = true
	return ti
}

// Focus gives focus to this input and updates visual styling.
func (t *TextInput) Focus() tea.Cmd {
	t.applyFocusStyle()
	return t.Input.Focus()
}

// Blur removes focus from this input and updates visual styling.
func (t *TextInput) Blur() {
	t.Input.Prompt = BlurredPrompt
	t.Input.TextStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	t.Input.Blur()
}

// applyFocusStyle sets focused visual styling (prompt and text color).
func (t *TextInput) applyFocusStyle() {
	t.Input.Prompt = FocusedPrompt
	t.Input.TextStyle = lipgloss.NewStyle().Foreground(ColorWhite)
}

// Focused returns whether this input is currently focused.
func (t *TextInput) Focused() bool {
	return t.Input.Focused()
}

// Value returns the current input value.
func (t *TextInput) Value() string {
	return t.Input.Value()
}

// SetValue sets the input value.
func (t *TextInput) SetValue(s string) {
	t.Input.SetValue(s)
}

// Update handles input events.
func (t *TextInput) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	t.Input, cmd = t.Input.Update(msg)
	// Clear error on edit
	t.Err = ""
	return cmd
}

// RunValidation runs the validator and sets the error.
// Returns true if valid.
func (t *TextInput) RunValidation() bool {
	if t.Validate == nil {
		return true
	}
	if err := t.Validate(t.Value()); err != nil {
		t.Err = err.Error()
		return false
	}
	t.Err = ""
	return true
}

// View renders the labeled input.
func (t *TextInput) View() string {
	label := LabelStyle.Render(t.Label)
	hint := ""
	if t.Hint != "" {
		hint = " " + HintStyle.Render(t.Hint)
	}
	errLine := ""
	if t.Err != "" {
		errLine = "\n  " + ErrorStyle.Render("! "+t.Err)
	}
	inputStyle := lipgloss.NewStyle().MarginLeft(2)
	return label + hint + "\n" + inputStyle.Render(t.Input.View()) + errLine
}
