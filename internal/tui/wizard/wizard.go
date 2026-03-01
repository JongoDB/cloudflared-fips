// Package wizard implements the interactive setup wizard for cloudflared-fips.
// Pages are built dynamically based on the selected role and deployment tier.
package wizard

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/common"
	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/config"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/buildinfo"
)

// WizardModel is the top-level Bubbletea model for the setup wizard.
type WizardModel struct {
	pages     []Page
	pageIndex int
	config    *config.Config
	width     int
	height    int
	done      bool
	err       string

	// Page 1 is always the RoleTierPage; kept as typed reference for
	// rebuild decisions.
	roleTierPage *RoleTierPage

	// Track last-built role+tier to avoid unnecessary rebuilds.
	lastRole string
	lastTier string
}

// NewWizardModel creates a new setup wizard starting with the Role & Tier page.
func NewWizardModel() WizardModel {
	cfg := config.NewDefaultConfig()
	rtp := NewRoleTierPage()

	m := WizardModel{
		config:       cfg,
		roleTierPage: rtp,
	}
	m.rebuildPages()
	return m
}

func (m WizardModel) totalPages() int {
	return len(m.pages)
}

// rebuildPages constructs the full page list based on the current role and tier
// selections. Page 1 (RoleTierPage) is always present; the rest are dynamic.
func (m *WizardModel) rebuildPages() {
	role := m.roleTierPage.SelectedRole()
	tier := m.roleTierPage.SelectedTier()

	pages := []Page{m.roleTierPage}

	switch role {
	case "controller":
		pages = append(pages, NewControllerConfigPage())
		pages = append(pages, NewDashboardWiringPage())
		if tier == "regional_keyless" || tier == "self_hosted" {
			pages = append(pages, NewTierSpecificPage(tier))
		}
	case "server":
		pages = append(pages, NewServerConfigPage())
	case "proxy":
		pages = append(pages, NewProxyConfigPage())
	case "client":
		pages = append(pages, NewAgentConfigPage())
	}

	pages = append(pages, NewFIPSOptionsPage())
	pages = append(pages, NewReviewPage())

	m.pages = pages
	m.lastRole = role
	m.lastTier = tier

	// Apply saved dimensions to all pages.
	if m.width > 0 {
		for _, p := range m.pages {
			p.SetSize(m.width-4, m.height-8)
		}
	}
}

// needsRebuild returns true if the role/tier selection has changed since the
// last rebuild.
func (m *WizardModel) needsRebuild() bool {
	return m.roleTierPage.SelectedRole() != m.lastRole ||
		m.roleTierPage.SelectedTier() != m.lastTier
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
			p.SetSize(msg.Width-4, msg.Height-8)
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "tab", "enter":
			return m.handlePageAdvance(msg)

		case "shift+tab":
			return m.handlePageBack(msg)
		}
	}

	// Delegate to current page
	return m.delegateToPage(msg)
}

func (m WizardModel) handlePageAdvance(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	total := m.totalPages()

	// First, delegate to the page to handle internal navigation
	page, cmd := m.pages[m.pageIndex].Update(msg)
	m.pages[m.pageIndex] = page

	// For the review page, enter triggers config write / provision — don't advance
	if m.pageIndex == total-1 {
		return m, cmd
	}

	// If no cmd was returned, the page has no more internal fields to advance
	// through — try advancing the wizard page.
	if cmd == nil {
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
	total := m.totalPages()

	// Validate current page
	if !m.pages[m.pageIndex].Validate() {
		m.err = "Please fix the errors above before continuing."
		return m, nil
	}
	m.err = ""

	// Apply values to shared config
	m.pages[m.pageIndex].Apply(m.config)

	// After page 1, rebuild the page list if role/tier changed
	if m.pageIndex == 0 && m.needsRebuild() {
		m.rebuildPages()
		total = m.totalPages()
	}

	if m.pageIndex < total-1 {
		m.pageIndex++

		// Pre-populate cross-page data: tunnel ID → dashboard wiring
		if dp, ok := m.pages[m.pageIndex].(*DashboardWiringPage); ok {
			dp.PrePopulateTunnelID(m.config.Tunnel)
		}

		// Before showing review page, apply all preceding pages
		if m.pageIndex == total-1 {
			for i := 0; i < total-1; i++ {
				m.pages[i].Apply(m.config)
			}
			if rp, ok := m.pages[total-1].(*ReviewPage); ok {
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
	total := m.totalPages()
	var b strings.Builder

	// Header
	header := common.HeaderStyle.Render(
		common.TitleStyle.Render("cloudflared-fips") +
			common.MutedStyle.Render(" "+buildinfo.Version+" Setup Wizard"))
	b.WriteString(header)
	b.WriteString("\n")

	// Progress
	b.WriteString(renderProgress(m.pageIndex+1, total, m.pages[m.pageIndex].Title()))
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
	b.WriteString(common.FooterStyle.Render(renderNavHints(m.pageIndex == 0, m.pageIndex == total-1)))

	return common.PageStyle.Render(b.String())
}
