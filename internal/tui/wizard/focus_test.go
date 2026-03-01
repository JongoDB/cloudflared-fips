package wizard

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func simulateTab() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyTab}
}

func simulateShiftTab() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyShiftTab}
}

// TestWizardFocus_RoleTierPage verifies that the RoleTierPage starts with
// visible focus on the first field (role selector).
func TestWizardFocus_RoleTierPage(t *testing.T) {
	m := NewWizardModel()
	m.width = 120
	m.height = 40
	for _, p := range m.pages {
		p.SetSize(116, 32)
	}

	// Focus the first page (Init would do this)
	cmd := m.pages[0].Focus()
	if cmd == nil {
		t.Error("RoleTierPage.Focus() returned nil — should return a command to trigger re-render")
	}
	m.syncViewport()

	view := m.View()

	// The role selector should show a focused cursor ">"
	if !strings.Contains(view, "▸") {
		t.Error("RoleTierPage should show '> ' cursor indicator when focused")
	}
}

// TestWizardFocus_AdvanceToControllerConfig verifies that after tabbing through
// all RoleTierPage fields, the ControllerConfigPage gets proper focus.
func TestWizardFocus_AdvanceToControllerConfig(t *testing.T) {
	m := NewWizardModel()
	m.width = 120
	m.height = 40
	for _, p := range m.pages {
		p.SetSize(116, 32)
	}

	// Focus page 1
	m.pages[0].Focus()

	// Controller role has 3 fields: role, tier, skipFIPS
	// Tab through all 3 fields to advance to next page
	var model tea.Model = m

	// Tab 1: role → tier
	model, _ = model.Update(simulateTab())
	m = model.(WizardModel)
	if m.pageIndex != 0 {
		t.Fatalf("after tab 1, still on page 0, got page %d", m.pageIndex)
	}

	// Tab 2: tier → skipFIPS
	model, _ = model.Update(simulateTab())
	m = model.(WizardModel)
	if m.pageIndex != 0 {
		t.Fatalf("after tab 2, still on page 0, got page %d", m.pageIndex)
	}

	// Tab 3: skipFIPS → advances to page 1 (ControllerConfigPage)
	model, cmd := model.Update(simulateTab())
	m = model.(WizardModel)
	if m.pageIndex != 1 {
		t.Fatalf("after tab 3, expected page 1, got page %d", m.pageIndex)
	}

	// Focus command should NOT be nil
	if cmd == nil {
		t.Error("advancing to ControllerConfigPage should return a non-nil focus command")
	}

	// Verify the view shows the admin key input with cursor
	view := m.View()
	if !strings.Contains(view, "Admin API Key") {
		t.Error("ControllerConfigPage should show 'Admin API Key' field")
	}
	if !strings.Contains(view, "Controller & Tunnel") {
		t.Error("should show 'Controller & Tunnel' in page title or header")
	}
}

// TestWizardFocus_AdvanceToFIPSOptions verifies focus works when entering the
// FIPSOptionsPage (first field is a Toggle, not TextInput).
func TestWizardFocus_AdvanceToFIPSOptions(t *testing.T) {
	fips := NewFIPSOptionsPage()
	fips.SetSize(116, 32)

	cmd := fips.Focus()
	if cmd == nil {
		t.Error("FIPSOptionsPage.Focus() returned nil — should return a command")
	}

	// First field (selfTestOnStart) should be focused
	if !fips.selfTestOnStart.Focused {
		t.Error("selfTestOnStart should be focused after Focus()")
	}

	// View should show cursor on first toggle
	view := fips.View()
	if !strings.Contains(view, "▸") {
		t.Error("FIPSOptionsPage should show '> ' cursor on focused toggle")
	}
}

// TestWizardFocus_AdvanceToTierSpecific_Tier2 verifies focus for Tier 2
// TierSpecificPage (first field is a TextInput).
func TestWizardFocus_AdvanceToTierSpecific_Tier2(t *testing.T) {
	page := NewTierSpecificPage("regional_keyless")
	page.SetSize(116, 32)

	cmd := page.Focus()
	if cmd == nil {
		t.Error("TierSpecificPage(tier2).Focus() returned nil — should return TextInput focus command")
	}

	view := page.View()
	if !strings.Contains(view, "Keyless SSL Host") {
		t.Error("Tier 2 page should show 'Keyless SSL Host'")
	}
}

// TestWizardFocus_AdvanceToTierSpecific_Tier3 verifies focus for Tier 3.
func TestWizardFocus_AdvanceToTierSpecific_Tier3(t *testing.T) {
	page := NewTierSpecificPage("self_hosted")
	page.SetSize(116, 32)

	cmd := page.Focus()
	if cmd == nil {
		t.Error("TierSpecificPage(tier3).Focus() returned nil — should return TextInput focus command")
	}

	view := page.View()
	if !strings.Contains(view, "Proxy Listen Address") {
		t.Error("Tier 3 page should show 'Proxy Listen Address'")
	}
}

// TestWizardFocus_FullControllerFlow simulates the controller wizard flow,
// checking focus at page transitions. ControllerConfigPage requires a non-empty
// tunnel token to validate, so we verify focus behavior within the page.
func TestWizardFocus_FullControllerFlow(t *testing.T) {
	m := NewWizardModel()
	m.width = 120
	m.height = 40
	for _, p := range m.pages {
		p.SetSize(116, 32)
	}
	m.pages[0].Focus()

	var model tea.Model = m
	var cmd tea.Cmd

	// Page 0: RoleTierPage — tab through 3 fields (role, tier, skipFIPS)
	for i := 0; i < 3; i++ {
		model, cmd = model.Update(simulateTab())
	}
	m = model.(WizardModel)
	if m.pageIndex != 1 {
		t.Fatalf("expected page 1 after RoleTierPage, got %d", m.pageIndex)
	}
	if cmd == nil {
		t.Error("focus command should not be nil when entering ControllerConfigPage")
	}

	// On ControllerConfigPage, verify focus moves through fields
	// Tab through first 3 fields (adminKey → nodeName → region)
	for i := 0; i < 3; i++ {
		model, cmd = model.Update(simulateTab())
	}
	m = model.(WizardModel)
	if m.pageIndex != 1 {
		t.Fatalf("should still be on page 1, got %d", m.pageIndex)
	}

	// Each tab should produce a non-nil command (fieldNav)
	if cmd == nil {
		t.Error("tabbing within ControllerConfigPage should return non-nil cmd")
	}

	// Verify view shows focus indicator on the correct field
	view := m.View()
	if !strings.Contains(view, "Controller & Tunnel") {
		t.Error("should show Controller & Tunnel page")
	}
}

// TestWizardViewport_ScrollsToFocusedField verifies that on a tall page
// (ControllerConfigPage, 10 fields), the viewport auto-scrolls so the
// focused field is visible. With a short viewport (15 lines), the first
// field should be visible initially, and tabbing far enough should scroll.
func TestWizardViewport_ScrollsToFocusedField(t *testing.T) {
	m := NewWizardModel()
	// Simulate a small terminal so viewport clips the content.
	m.width = 80
	m.height = 25
	vpHeight := m.height - headerLines - footerLines
	m.viewport.Width = m.width - 4
	m.viewport.Height = vpHeight
	for _, p := range m.pages {
		p.SetSize(m.width-4, m.height-8)
	}
	m.pages[0].Focus()

	var model tea.Model = m

	// Advance past RoleTierPage (3 tabs) → ControllerConfigPage
	for i := 0; i < 3; i++ {
		model, _ = model.Update(simulateTab())
	}
	m = model.(WizardModel)
	if m.pageIndex != 1 {
		t.Fatalf("expected page 1, got %d", m.pageIndex)
	}

	// First field focused — viewport should show "Admin API Key"
	view := m.View()
	if !strings.Contains(view, "Admin API Key") {
		t.Error("viewport should show 'Admin API Key' when first field is focused")
	}

	// Tab down to field 6 (enforcementMode) — this is far enough to require scrolling.
	for i := 0; i < 6; i++ {
		model, _ = model.Update(simulateTab())
	}
	m = model.(WizardModel)

	view = m.View()
	if !strings.Contains(view, "Enforcement Mode") {
		t.Error("after tabbing to field 6, viewport should show 'Enforcement Mode'")
	}
}

// TestWizardFocus_BackNavigation verifies that shift+tab correctly restores
// focus when going back to a previous page.
func TestWizardFocus_BackNavigation(t *testing.T) {
	m := NewWizardModel()
	m.width = 120
	m.height = 40
	for _, p := range m.pages {
		p.SetSize(116, 32)
	}
	m.pages[0].Focus()

	var model tea.Model = m

	// Advance past RoleTierPage (3 tabs)
	for i := 0; i < 3; i++ {
		model, _ = model.Update(simulateTab())
	}
	m = model.(WizardModel)
	if m.pageIndex != 1 {
		t.Fatalf("expected page 1, got %d", m.pageIndex)
	}

	// Go back with shift+tab
	model, cmd := model.Update(simulateShiftTab())
	m = model.(WizardModel)
	if m.pageIndex != 0 {
		t.Fatalf("expected page 0 after shift+tab, got %d", m.pageIndex)
	}
	if cmd == nil {
		t.Error("going back to RoleTierPage should return a non-nil focus command")
	}

	// Verify RoleTierPage shows focus
	view := m.View()
	if !strings.Contains(view, "▸") {
		t.Error("RoleTierPage should show cursor after returning")
	}
}
