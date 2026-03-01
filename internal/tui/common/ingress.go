package common

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// IngressEntry represents a single hostname→service mapping.
type IngressEntry struct {
	Hostname string
	Service  string
}

// IngressEditor manages a list of ingress rules with add/remove.
type IngressEditor struct {
	Label    string
	Entries  []IngressEntry
	Focused  bool
	Cursor   int  // which entry is selected
	Adding   bool // in add mode
	EditHost textinput.Model
	EditSvc  textinput.Model
	Field    int // 0=hostname, 1=service during add
	Err      string
}

// NewIngressEditor creates a new ingress rule list editor.
func NewIngressEditor(label string) IngressEditor {
	host := textinput.New()
	host.Placeholder = "app.example.com"
	host.CharLimit = 128
	host.Width = 40

	svc := textinput.New()
	svc.Placeholder = "https://localhost:8443"
	svc.CharLimit = 128
	svc.Width = 40

	return IngressEditor{
		Label:    label,
		EditHost: host,
		EditSvc:  svc,
	}
}

// Focus gives focus.
func (e *IngressEditor) Focus() {
	e.Focused = true
}

// Blur removes focus.
func (e *IngressEditor) Blur() {
	e.Focused = false
	e.Adding = false
	e.EditHost.Blur()
	e.EditSvc.Blur()
}

// Update handles key events.
func (e *IngressEditor) Update(msg tea.Msg) tea.Cmd {
	if !e.Focused {
		return nil
	}

	if e.Adding {
		return e.updateAdding(msg)
	}

	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "a":
			e.Adding = true
			e.Field = 0
			e.EditHost.SetValue("")
			e.EditSvc.SetValue("")
			e.EditHost.Focus()
			e.Err = ""
			return nil
		case "d", "delete", "backspace":
			if len(e.Entries) > 0 {
				e.Entries = append(e.Entries[:e.Cursor], e.Entries[e.Cursor+1:]...)
				if e.Cursor >= len(e.Entries) && e.Cursor > 0 {
					e.Cursor--
				}
			}
			return nil
		case "up", "k":
			if e.Cursor > 0 {
				e.Cursor--
			}
		case "down", "j":
			if e.Cursor < len(e.Entries)-1 {
				e.Cursor++
			}
		}
	}
	return nil
}

func (e *IngressEditor) updateAdding(msg tea.Msg) tea.Cmd {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "esc":
			e.Adding = false
			e.EditHost.Blur()
			e.EditSvc.Blur()
			return nil
		case "tab", "enter":
			if e.Field == 0 {
				if strings.TrimSpace(e.EditHost.Value()) == "" {
					e.Err = "hostname is required"
					return nil
				}
				e.Field = 1
				e.EditHost.Blur()
				return e.EditSvc.Focus()
			}
			// Field 1: confirm add
			if strings.TrimSpace(e.EditSvc.Value()) == "" {
				e.Err = "service URL is required"
				return nil
			}
			e.Entries = append(e.Entries, IngressEntry{
				Hostname: strings.TrimSpace(e.EditHost.Value()),
				Service:  strings.TrimSpace(e.EditSvc.Value()),
			})
			e.Adding = false
			e.Err = ""
			e.EditSvc.Blur()
			return nil
		}
	}

	var cmd tea.Cmd
	if e.Field == 0 {
		e.EditHost, cmd = e.EditHost.Update(msg)
	} else {
		e.EditSvc, cmd = e.EditSvc.Update(msg)
	}
	e.Err = ""
	return cmd
}

// View renders the ingress editor.
func (e *IngressEditor) View() string {
	var b strings.Builder
	b.WriteString(LabelStyle.Render(e.Label))
	b.WriteString(HintStyle.Render("  [a]dd  [d]elete"))
	b.WriteString("\n")

	if len(e.Entries) == 0 && !e.Adding {
		b.WriteString(MutedStyle.Render("  (no rules — press 'a' to add)") + "\n")
	}

	for i, entry := range e.Entries {
		cursor := "  "
		if i == e.Cursor && e.Focused && !e.Adding {
			cursor = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render("▸ ")
		}
		style := MutedStyle
		if i == e.Cursor && e.Focused {
			style = lipgloss.NewStyle().Bold(true).Foreground(ColorWhite)
		}
		b.WriteString(cursor + style.Render(fmt.Sprintf("%s → %s", entry.Hostname, entry.Service)) + "\n")
	}

	// Always show the catch-all
	b.WriteString(MutedStyle.Render("  * → http_status:404 (catch-all)") + "\n")

	if e.Adding {
		b.WriteString("\n")
		activeField := "hostname"
		if e.Field == 1 {
			activeField = "service"
		}
		b.WriteString(HintStyle.Render(fmt.Sprintf("  Adding rule (%s):", activeField)) + "\n")
		b.WriteString("  Hostname: " + e.EditHost.View() + "\n")
		b.WriteString("  Service:  " + e.EditSvc.View() + "\n")
		if e.Err != "" {
			b.WriteString("  " + ErrorStyle.Render("! "+e.Err) + "\n")
		}
		b.WriteString(HintStyle.Render("  Tab/Enter=next field  Esc=cancel") + "\n")
	}

	return b.String()
}
