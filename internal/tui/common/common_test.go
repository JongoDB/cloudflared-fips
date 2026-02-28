package common

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// ---------------------------------------------------------------------------
// isNotLoggedIn
// ---------------------------------------------------------------------------

func TestIsNotLoggedIn(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   bool
	}{
		{"origincert message", "Cannot read origincert from /path", true},
		{"origin cert message", "No origin cert found", true},
		{"cert.pem message", "Missing cert.pem file", true},
		{"unrelated error", "connection timeout", false},
		{"empty", "", false},
		{"mixed case origincert", "Please provide ORIGINCERT", false}, // case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNotLoggedIn(tt.output)
			if got != tt.want {
				t.Errorf("isNotLoggedIn(%q) = %v, want %v", tt.output, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// firstLine
// ---------------------------------------------------------------------------

func TestFirstLine(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"single line", "hello world", "hello world"},
		{"multi line", "first\nsecond\nthird", "first"},
		{"leading newlines", "\n\nthird line", "third line"},
		{"trailing newlines", "hello\n\n", "hello"},
		{"whitespace lines", "  \n  \n  real content  ", "real content"},
		{"empty", "", ""},
		{"only whitespace", "   \n  \n  ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := firstLine(tt.input)
			if got != tt.want {
				t.Errorf("firstLine(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Selector
// ---------------------------------------------------------------------------

func TestNewSelector(t *testing.T) {
	s := NewSelector("Test", []SelectorOption{
		{Value: "a", Label: "Option A"},
		{Value: "b", Label: "Option B"},
	})
	if s.Label != "Test" {
		t.Errorf("Label = %q, want Test", s.Label)
	}
	if len(s.Options) != 2 {
		t.Errorf("Options length = %d, want 2", len(s.Options))
	}
	if s.Cursor != 0 {
		t.Errorf("Cursor = %d, want 0", s.Cursor)
	}
	if s.Focused {
		t.Error("should not be focused initially")
	}
}

func TestSelector_FocusBlur(t *testing.T) {
	s := NewSelector("Test", []SelectorOption{{Value: "a"}})
	s.Focus()
	if !s.Focused {
		t.Error("should be focused after Focus()")
	}
	s.Blur()
	if s.Focused {
		t.Error("should not be focused after Blur()")
	}
}

func TestSelector_Selected(t *testing.T) {
	s := NewSelector("Test", []SelectorOption{
		{Value: "a", Label: "A"},
		{Value: "b", Label: "B"},
		{Value: "c", Label: "C"},
	})
	if s.Selected() != "a" {
		t.Errorf("Selected() = %q, want a", s.Selected())
	}
	if s.SelectedIndex() != 0 {
		t.Errorf("SelectedIndex() = %d, want 0", s.SelectedIndex())
	}
}

func TestSelector_SetSelected(t *testing.T) {
	s := NewSelector("Test", []SelectorOption{
		{Value: "a"}, {Value: "b"}, {Value: "c"},
	})
	s.SetSelected("c")
	if s.Selected() != "c" {
		t.Errorf("after SetSelected(c): Selected() = %q, want c", s.Selected())
	}
	if s.SelectedIndex() != 2 {
		t.Errorf("after SetSelected(c): SelectedIndex() = %d, want 2", s.SelectedIndex())
	}
}

func TestSelector_SetSelected_NotFound(t *testing.T) {
	s := NewSelector("Test", []SelectorOption{
		{Value: "a"}, {Value: "b"},
	})
	s.SetSelected("z") // not in options
	if s.Cursor != 0 {
		t.Errorf("Cursor should remain 0 when value not found, got %d", s.Cursor)
	}
}

func TestSelector_Selected_EmptyOptions(t *testing.T) {
	s := NewSelector("Test", nil)
	if s.Selected() != "" {
		t.Errorf("Selected() with no options = %q, want empty", s.Selected())
	}
}

func TestSelector_Update_Navigation(t *testing.T) {
	s := NewSelector("Test", []SelectorOption{
		{Value: "a"}, {Value: "b"}, {Value: "c"},
	})
	s.Focus()

	// Down arrow
	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if s.Cursor != 1 {
		t.Errorf("after j: Cursor = %d, want 1", s.Cursor)
	}

	// Down again
	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if s.Cursor != 2 {
		t.Errorf("after j,j: Cursor = %d, want 2", s.Cursor)
	}

	// Down at bottom — should not go past last
	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if s.Cursor != 2 {
		t.Errorf("at bottom after j: Cursor = %d, want 2", s.Cursor)
	}

	// Up arrow
	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if s.Cursor != 1 {
		t.Errorf("after k: Cursor = %d, want 1", s.Cursor)
	}

	// Up past top
	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if s.Cursor != 0 {
		t.Errorf("at top after k,k: Cursor = %d, want 0", s.Cursor)
	}
}

func TestSelector_Update_IgnoresWhenBlurred(t *testing.T) {
	s := NewSelector("Test", []SelectorOption{
		{Value: "a"}, {Value: "b"},
	})
	// Not focused
	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if s.Cursor != 0 {
		t.Errorf("blurred selector should ignore keys: Cursor = %d, want 0", s.Cursor)
	}
}

func TestSelector_View_NonEmpty(t *testing.T) {
	s := NewSelector("Pick one", []SelectorOption{
		{Value: "a", Label: "Alpha", Description: "First option"},
		{Value: "b", Label: "Beta"},
	})
	view := s.View()
	if view == "" {
		t.Error("View() should not be empty")
	}
}

// ---------------------------------------------------------------------------
// Toggle
// ---------------------------------------------------------------------------

func TestNewToggle(t *testing.T) {
	tog := NewToggle("Enable FIPS", "Recommended", true)
	if tog.Label != "Enable FIPS" {
		t.Errorf("Label = %q, want 'Enable FIPS'", tog.Label)
	}
	if tog.Hint != "Recommended" {
		t.Errorf("Hint = %q, want Recommended", tog.Hint)
	}
	if !tog.Enabled {
		t.Error("Enabled should be true (default on)")
	}
	if tog.Focused {
		t.Error("should not be focused initially")
	}
}

func TestToggle_FocusBlur(t *testing.T) {
	tog := NewToggle("Test", "", false)
	tog.Focus()
	if !tog.Focused {
		t.Error("should be focused after Focus()")
	}
	tog.Blur()
	if tog.Focused {
		t.Error("should not be focused after Blur()")
	}
}

func TestToggle_Update_SpaceToggles(t *testing.T) {
	tog := NewToggle("Test", "", false)
	tog.Focus()

	// Toggle on
	tog.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	if !tog.Enabled {
		t.Error("after space: should be enabled")
	}

	// Toggle off
	tog.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	if tog.Enabled {
		t.Error("after space x2: should be disabled")
	}
}

func TestToggle_Update_EnterToggles(t *testing.T) {
	tog := NewToggle("Test", "", false)
	tog.Focus()

	tog.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !tog.Enabled {
		t.Error("after enter: should be enabled")
	}
}

func TestToggle_Update_IgnoresWhenBlurred(t *testing.T) {
	tog := NewToggle("Test", "", false)
	// Not focused
	tog.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	if tog.Enabled {
		t.Error("blurred toggle should ignore keys")
	}
}

func TestToggle_View_NonEmpty(t *testing.T) {
	tog := NewToggle("Enable", "hint text", true)
	view := tog.View()
	if view == "" {
		t.Error("View() should not be empty")
	}
}

// ---------------------------------------------------------------------------
// ErrNotLoggedIn
// ---------------------------------------------------------------------------

func TestErrNotLoggedIn_Message(t *testing.T) {
	if ErrNotLoggedIn.Error() == "" {
		t.Error("ErrNotLoggedIn should have a message")
	}
}

// ---------------------------------------------------------------------------
// uuidFromOutput regex
// ---------------------------------------------------------------------------

func TestUUIDRegex(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Created tunnel test with id a1b2c3d4-e5f6-7890-abcd-ef1234567890", "a1b2c3d4-e5f6-7890-abcd-ef1234567890"},
		{"no uuid here", ""},
		{"partial 12345678-1234", ""},
	}

	for _, tt := range tests {
		got := uuidFromOutput.FindString(tt.input)
		if got != tt.want {
			t.Errorf("uuidFromOutput(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
