package status

import (
	"fmt"
	"strings"

	"github.com/cloudflared-fips/cloudflared-fips/internal/compliance"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/fleet"
)

// statusIcon returns a colored icon for a compliance status.
func statusIcon(s compliance.Status) string {
	switch s {
	case compliance.StatusPass:
		return passStyle.Render("●")
	case compliance.StatusWarning:
		return warnStyle.Render("○")
	case compliance.StatusFail:
		return failStyle.Render("✖")
	default:
		return unknownStyle.Render("?")
	}
}

// statusLabel returns a colored status label.
func statusLabel(s compliance.Status) string {
	switch s {
	case compliance.StatusPass:
		return passStyle.Render("PASS")
	case compliance.StatusWarning:
		return warnStyle.Render("WARN")
	case compliance.StatusFail:
		return failStyle.Render("FAIL")
	default:
		return unknownStyle.Render("UNKN")
	}
}

// renderSection renders a compliance section with its items.
func renderSection(section compliance.Section) string {
	var b strings.Builder

	passCount := 0
	for _, item := range section.Items {
		if item.Status == compliance.StatusPass {
			passCount++
		}
	}

	name := sectionNameStyle.Render(section.Name)
	count := sectionCountStyle.Render(fmt.Sprintf("%d/%d pass", passCount, len(section.Items)))

	// Right-align the count
	b.WriteString(fmt.Sprintf(" %s  %s\n", name, count))

	for _, item := range section.Items {
		b.WriteString(renderItem(item))
		b.WriteString("\n")
	}

	return b.String()
}

// renderItem renders a single checklist item line.
func renderItem(item compliance.ChecklistItem) string {
	icon := statusIcon(item.Status)
	label := statusLabel(item.Status)
	name := item.Name

	// Dim passing items to reduce noise, highlight failures
	if item.Status == compliance.StatusPass {
		name = dimStyle.Render(name)
	} else if item.Status == compliance.StatusFail {
		name = failStyle.Render(name)
	} else if item.Status == compliance.StatusWarning {
		name = warnStyle.Render(name)
	}

	return fmt.Sprintf("   %s %-44s %s", icon, name, label)
}

// renderSummaryBar renders the top-level summary bar.
func renderSummaryBar(summary compliance.Summary, width int) string {
	total := summary.Total
	if total == 0 {
		return dimStyle.Render("No compliance data")
	}

	percent := 0
	if total > 0 {
		percent = (summary.Passed * 100) / total
	}

	parts := []string{}
	parts = append(parts, passStyle.Render(fmt.Sprintf("%d PASS", summary.Passed)))
	if summary.Warnings > 0 {
		parts = append(parts, warnStyle.Render(fmt.Sprintf("%d WARN", summary.Warnings)))
	}
	if summary.Failed > 0 {
		parts = append(parts, failStyle.Render(fmt.Sprintf("%d FAIL", summary.Failed)))
	}
	if summary.Unknown > 0 {
		parts = append(parts, unknownStyle.Render(fmt.Sprintf("%d UNKN", summary.Unknown)))
	}

	counts := fmt.Sprintf("  %d/%d  %s", summary.Passed, total, strings.Join(parts, "   "))

	// Progress bar
	barWidth := 20
	if width > 80 {
		barWidth = 30
	}
	filled := (summary.Passed * barWidth) / total
	bar := ""
	for i := 0; i < barWidth; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}

	percentStr := fmt.Sprintf("%d%%", percent)
	var pStyle func(...string) string
	if percent >= 90 {
		pStyle = passStyle.Render
	} else if percent >= 70 {
		pStyle = warnStyle.Render
	} else {
		pStyle = failStyle.Render
	}

	return summaryBoxStyle.Render(counts + "   " + pStyle(percentStr) + " " + pStyle(bar))
}

// renderFleetTopology renders the fleet hub-spoke topology diagram.
func renderFleetTopology(summary *fleet.FleetSummary, fleetErr error) string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(sectionNameStyle.Render(" Fleet Topology"))
	b.WriteString("\n")

	if fleetErr != nil {
		b.WriteString(dimStyle.Render(fmt.Sprintf("   Fleet data unavailable: %v", fleetErr)))
		b.WriteString("\n")
		return b.String()
	}

	if summary == nil {
		b.WriteString(dimStyle.Render("   Waiting for fleet data..."))
		b.WriteString("\n")
		return b.String()
	}

	byRole := summary.ByRole
	if byRole == nil {
		byRole = map[string]int{}
	}

	ctrlCount := byRole["controller"]
	serverCount := byRole["server"]
	proxyCount := byRole["proxy"]
	clientCount := byRole["client"]

	// Controller at the top
	ctrlStatus := passStyle.Render("● Online")
	if ctrlCount == 0 {
		ctrlStatus = unknownStyle.Render("○ None")
	}
	b.WriteString(fmt.Sprintf("                  CONTROLLER (%d)  %s\n", ctrlCount, ctrlStatus))
	b.WriteString(dimStyle.Render("             ┌──────────┼──────────┐"))
	b.WriteString("\n")

	// Child roles
	srvLabel := renderRoleCount("SERVER", serverCount)
	prxLabel := renderRoleCount("PROXY", proxyCount)
	cliLabel := renderRoleCount("CLIENT", clientCount)

	b.WriteString(fmt.Sprintf("        %-18s %-18s %s\n", srvLabel, prxLabel, cliLabel))

	// Fleet totals
	totalLine := dimStyle.Render(fmt.Sprintf(
		"   Total: %d nodes  %s online  %s degraded  %s offline  %s compliant",
		summary.TotalNodes,
		passStyle.Render(fmt.Sprintf("%d", summary.Online)),
		warnStyle.Render(fmt.Sprintf("%d", summary.Degraded)),
		failStyle.Render(fmt.Sprintf("%d", summary.Offline)),
		passStyle.Render(fmt.Sprintf("%d", summary.FullyCompliant)),
	))
	b.WriteString(totalLine)
	b.WriteString("\n")

	return b.String()
}

// renderRoleCount renders a role name with its node count, colored by status.
func renderRoleCount(role string, count int) string {
	if count == 0 {
		return unknownStyle.Render(fmt.Sprintf("%s(0)", role))
	}
	return passStyle.Render(fmt.Sprintf("%s(%d)", role, count))
}
