// Package wizard implements the interactive setup wizard for cloudflared-fips.
// It walks through 5 pages: Tunnel, Dashboard Wiring, Deployment Tier,
// FIPS Options, and Review & Write.
package wizard

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/common"
	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/config"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/buildinfo"
)

const totalPages = 5

// WizardModel is the top-level Bubbletea model for the setup wizard.
type WizardModel struct {
	pages     []Page
	pageIndex int
	config    *config.Config
	width     int
	height    int
	done      bool
	err       string
}

// NewWizardModel creates a new setup wizard.
func NewWizardModel() WizardModel {
	cfg := config.NewDefaultConfig()

	tunnelPage := NewTunnelPage()
	dashPage := NewDashboardWiringPage()
	tierPage := NewDeploymentTierPage()
	fipsPage := NewFIPSOptionsPage()
	reviewPage := NewReviewPage()

	return WizardModel{
		pages: []Page{
			tunnelPage,
			dashPage,
			tierPage,
			fipsPage,
			reviewPage,
		},
		config: cfg,
	}
}

// Init initializes the wizard, focusing the first page.
func (m WizardModel) Init() tea.Cmd {
	return m.pages[0].Focus()
}

// Update handles messages for the wizard.
func (m WizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		for _, p := range m.pages {
			p.SetSize(msg.Width-4, msg.Height-8) // Reserve for chrome
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "tab", "enter":
			// Check if current page can handle this itself
			// (internal field navigation). The page's Update will consume it
			// if it can advance internally. We need to check if we should
			// advance the wizard page instead.
			//
			// Strategy: let the page update first, then check if we need
			// to advance.
			return m.handlePageAdvance(msg)

		case "shift+tab":
			return m.handlePageBack(msg)
		}
	}

	// Delegate to current page
	return m.delegateToPage(msg)
}

func (m WizardModel) handlePageAdvance(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// First, delegate to the page to handle internal navigation
	page, cmd := m.pages[m.pageIndex].Update(msg)
	m.pages[m.pageIndex] = page

	// For the review page, enter triggers config write — don't advance
	if m.pageIndex == totalPages-1 {
		return m, cmd
	}

	// If we're on the last internal field, advance the wizard page
	// We detect this by attempting to advance and checking if the page
	// would have consumed the key. Since our pages return without cmd
	// when at their last field, we use a simpler approach: try to advance
	// the wizard page directly when tab/enter is pressed, but only after
	// letting the page try to handle it.
	//
	// For simplicity: if no cmd was returned and the key was tab/enter,
	// try advancing the wizard.
	if cmd == nil && msg.String() == "tab" {
		return m.advancePage()
	}
	if cmd == nil && msg.String() == "enter" {
		return m.advancePage()
	}

	return m, cmd
}

func (m WizardModel) handlePageBack(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Let page handle internal back navigation first
	page, cmd := m.pages[m.pageIndex].Update(msg)
	m.pages[m.pageIndex] = page

	if cmd == nil && m.pageIndex > 0 {
		m.pageIndex--
		focusCmd := m.pages[m.pageIndex].Focus()
		return m, focusCmd
	}
	return m, cmd
}

func (m WizardModel) advancePage() (tea.Model, tea.Cmd) {
	// Validate current page
	if !m.pages[m.pageIndex].Validate() {
		m.err = "Please fix the errors above before continuing."
		return m, nil
	}
	m.err = ""

	// Apply values to shared config
	m.pages[m.pageIndex].Apply(m.config)

	if m.pageIndex < totalPages-1 {
		m.pageIndex++

		// Pre-populate cross-page data
		if m.pageIndex == 1 {
			if dp, ok := m.pages[1].(*DashboardWiringPage); ok {
				dp.PrePopulateTunnelID(m.config.Tunnel)
			}
		}
		if m.pageIndex == totalPages-1 {
			// Apply all pages before review
			for i := 0; i < totalPages-1; i++ {
				m.pages[i].Apply(m.config)
			}
			if rp, ok := m.pages[totalPages-1].(*ReviewPage); ok {
				rp.SetConfig(m.config)
			}
		}

		focusCmd := m.pages[m.pageIndex].Focus()
		return m, focusCmd
	}
	return m, nil
}

func (m WizardModel) delegateToPage(msg tea.Msg) (tea.Model, tea.Cmd) {
	page, cmd := m.pages[m.pageIndex].Update(msg)
	m.pages[m.pageIndex] = page
	return m, cmd
}

// View renders the complete wizard view.
func (m WizardModel) View() string {
	var b strings.Builder

	// Header
	header := common.HeaderStyle.Render(
		common.TitleStyle.Render("cloudflared-fips") +
			common.MutedStyle.Render(" "+buildinfo.Version+" Setup Wizard"))
	b.WriteString(header)
	b.WriteString("\n")

	// Progress
	b.WriteString(renderProgress(m.pageIndex+1, totalPages, m.pages[m.pageIndex].Title()))
	b.WriteString("\n\n")

	// Page content
	b.WriteString(m.pages[m.pageIndex].View())

	// Error
	if m.err != "" {
		b.WriteString("\n\n")
		b.WriteString(common.ErrorStyle.Render(m.err))
	}

	// Footer
	b.WriteString("\n")
	b.WriteString(common.FooterStyle.Render(renderNavHints(m.pageIndex == 0, m.pageIndex == totalPages-1)))

	return common.PageStyle.Render(b.String())
}
