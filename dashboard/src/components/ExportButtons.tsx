import type { ChecklistSection, BuildManifest, ComplianceSummary } from '../types/compliance'

interface ExportButtonsProps {
  sections: ChecklistSection[]
  manifest: BuildManifest
  summary: ComplianceSummary
}

function buildJsonReport(sections: ChecklistSection[], manifest: BuildManifest, summary: ComplianceSummary) {
  return {
    report_type: 'cloudflared-fips-compliance',
    generated_at: new Date().toISOString(),
    build_manifest: manifest,
    summary,
    sections: sections.map((s) => ({
      id: s.id,
      name: s.name,
      items: s.items.map((i) => ({
        id: i.id,
        name: i.name,
        status: i.status,
        severity: i.severity,
        nist_ref: i.nistRef,
      })),
    })),
  }
}

function downloadJson(sections: ChecklistSection[], manifest: BuildManifest, summary: ComplianceSummary) {
  const report = buildJsonReport(sections, manifest, summary)
  const blob = new Blob([JSON.stringify(report, null, 2)], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `compliance-report-${new Date().toISOString().slice(0, 10)}.json`
  a.click()
  URL.revokeObjectURL(url)
}

function downloadPdfStub() {
  // PDF export requires a backend (pandoc or puppeteer).
  // This stub demonstrates the UX; wire to /api/v1/compliance/export?format=pdf in production.
  alert(
    'PDF export requires the Go dashboard backend.\n\n' +
    'In production, this calls /api/v1/compliance/export?format=pdf which uses pandoc to render the report.\n\n' +
    'Use JSON export for now, or run the dashboard backend: go run ./cmd/dashboard'
  )
}

export default function ExportButtons({ sections, manifest, summary }: ExportButtonsProps) {
  return (
    <div className="flex gap-2">
      <button
        onClick={() => downloadJson(sections, manifest, summary)}
        className="inline-flex items-center px-3 py-1.5 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50"
      >
        Export JSON
      </button>
      <button
        onClick={downloadPdfStub}
        className="inline-flex items-center px-3 py-1.5 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50"
      >
        Export PDF
      </button>
    </div>
  )
}
