import { useMemo } from 'react'
import Layout from '../components/Layout'
import SummaryBar from '../components/SummaryBar'
import BuildManifestPanel from '../components/BuildManifestPanel'
import ChecklistSection from '../components/ChecklistSection'
import ExportButtons from '../components/ExportButtons'
import { mockSections, mockManifest } from '../data/mockData'
import type { ComplianceSummary } from '../types/compliance'

export default function DashboardPage() {
  const summary: ComplianceSummary = useMemo(() => {
    const result = { total: 0, passed: 0, failed: 0, warnings: 0, unknown: 0 }
    for (const section of mockSections) {
      for (const item of section.items) {
        result.total++
        switch (item.status) {
          case 'pass':
            result.passed++
            break
          case 'fail':
            result.failed++
            break
          case 'warning':
            result.warnings++
            break
          case 'unknown':
            result.unknown++
            break
        }
      }
    }
    return result
  }, [])

  return (
    <Layout>
      <div className="flex items-center justify-between mb-6">
        <div>
          <p className="text-sm text-gray-500">
            Localhost-only dashboard &mdash; all assets bundled, air-gap friendly
          </p>
        </div>
        <ExportButtons sections={mockSections} manifest={mockManifest} summary={summary} />
      </div>
      <SummaryBar summary={summary} />
      <BuildManifestPanel manifest={mockManifest} />
      <div>
        <h2 className="text-lg font-semibold text-gray-900 mb-4">
          Compliance Checklist
        </h2>
        {mockSections.map((section) => (
          <ChecklistSection key={section.id} section={section} />
        ))}
      </div>
    </Layout>
  )
}
