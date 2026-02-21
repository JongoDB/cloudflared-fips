package wizard

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/config"
)

// Page is the interface every wizard page must implement.
type Page interface {
	// Init initializes the page, returning any initial command.
	Init() tea.Cmd

	// Update handles messages for this page.
	Update(msg tea.Msg) (Page, tea.Cmd)

	// View renders the page content (excluding header/footer).
	View() string

	// Title returns the page title shown in the progress header.
	Title() string

	// Validate checks all inputs on the page.
	// Returns true if valid; sets inline errors if not.
	Validate() bool

	// Apply writes this page's values into the shared config.
	Apply(cfg *config.Config)

	// SetSize updates the page's available dimensions.
	SetSize(w, h int)

	// Focus is called when the page becomes active.
	Focus() tea.Cmd
}
