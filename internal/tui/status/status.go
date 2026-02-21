// Package status implements the terminal-based compliance status monitor.
// It polls the dashboard API and renders pass/warn/fail for all checklist items.
package status

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/cloudflared-fips/cloudflared-fips/internal/compliance"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/buildinfo"
)

// StatusModel is the Bubbletea model for the compliance status view.
type StatusModel struct {
	apiAddr  string
	interval time.Duration
	viewport viewport.Model
	report   *compliance.ComplianceReport
	lastPoll time.Time
	err      error
	width    int
	height   int
	ready    bool
}

// NewStatusModel creates a new status monitor.
func NewStatusModel(apiAddr string, interval time.Duration) StatusModel {
	return StatusModel{
		apiAddr:  apiAddr,
		interval: interval,
	}
}

// pollMsg carries a compliance report from a poll tick.
type pollMsg struct {
	report *compliance.ComplianceReport
	err    error
}

// tickMsg triggers the next poll.
type tickMsg struct{}

// pollAPI fetches the compliance report from the dashboard API.
func pollAPI(addr string) tea.Cmd {
	return func() tea.Msg {
		url := fmt.Sprintf("http://%s/api/v1/compliance", addr)
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(url)
		if err != nil {
			return pollMsg{err: fmt.Errorf("connect to %s: %w", addr, err)}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return pollMsg{err: fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))}
		}

		var report compliance.ComplianceReport
		if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
			return pollMsg{err: fmt.Errorf("decode response: %w", err)}
		}
		return pollMsg{report: &report}
	}
}

// scheduleTick returns a command that sends a tickMsg after the interval.
func scheduleTick(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(_ time.Time) tea.Msg {
		return tickMsg{}
	})
}

// Init starts the first poll and schedules the tick loop.
func (m StatusModel) Init() tea.Cmd {
	return tea.Batch(pollAPI(m.apiAddr), scheduleTick(m.interval))
}

// Update handles messages.
func (m StatusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		contentH := msg.Height - 6 // reserve for header/footer
		if contentH < 5 {
			contentH = 5
		}
		if !m.ready {
			m.viewport = viewport.New(msg.Width, contentH)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = contentH
		}
		m.viewport.SetContent(m.renderContent())
		return m, nil

	case pollMsg:
		m.lastPoll = time.Now()
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.err = nil
			m.report = msg.report
		}
		if m.ready {
			m.viewport.SetContent(m.renderContent())
		}
		return m, nil

	case tickMsg:
		return m, tea.Batch(pollAPI(m.apiAddr), scheduleTick(m.interval))

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			// Force refresh
			return m, pollAPI(m.apiAddr)
		}
	}

	// Delegate to viewport for scrolling
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View renders the status monitor.
func (m StatusModel) View() string {
	var b strings.Builder

	// Header
	header := headerStyle.Render(
		titleStyle.Render("cloudflared-fips") +
			dimStyle.Render(" "+buildinfo.Version) +
			dimStyle.Render(" | Compliance Status") +
			m.renderLastUpdate())
	b.WriteString(header)
	b.WriteString("\n")

	if !m.ready {
		b.WriteString("\n  Initializing...\n")
		return b.String()
	}

	// Content
	b.WriteString(m.viewport.View())
	b.WriteString("\n")

	// Footer
	footer := m.renderFooter()
	b.WriteString(footerStyle.Render(footer))

	return b.String()
}

func (m StatusModel) renderLastUpdate() string {
	if m.lastPoll.IsZero() {
		return dimStyle.Render(" | Connecting...")
	}
	return dimStyle.Render(fmt.Sprintf(" | Updated %s", m.lastPoll.Format("15:04:05")))
}

func (m StatusModel) renderContent() string {
	var b strings.Builder

	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(failStyle.Render(fmt.Sprintf("  Error: %v", m.err)))
		b.WriteString("\n\n")
		b.WriteString(dimStyle.Render("  Press 'r' to retry | Check that the dashboard is running"))
		b.WriteString("\n")
		return b.String()
	}

	if m.report == nil {
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  Waiting for first poll..."))
		b.WriteString("\n")
		return b.String()
	}

	// Summary bar
	b.WriteString("\n")
	b.WriteString(renderSummaryBar(m.report.Summary, m.width))
	b.WriteString("\n")

	// Sections
	for _, section := range m.report.Sections {
		b.WriteString(renderSection(section))
	}

	return b.String()
}

func (m StatusModel) renderFooter() string {
	connStatus := passStyle.Render("Connected")
	if m.err != nil {
		connStatus = failStyle.Render("Disconnected")
	}

	return fmt.Sprintf(" [q] Quit  [r] Refresh  | Polling every %s | %s to %s",
		m.interval, connStatus, m.apiAddr)
}
