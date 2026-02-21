import { useMemo, useState } from 'react'
import Layout from '../components/Layout'
import SummaryBar from '../components/SummaryBar'
import BuildManifestPanel from '../components/BuildManifestPanel'
import ChecklistSection from '../components/ChecklistSection'
import ExportButtons from '../components/ExportButtons'
import SunsetBanner from '../components/SunsetBanner'
import DeploymentTierBadge from '../components/DeploymentTierBadge'
import { mockSections, mockManifest } from '../data/mockData'
import { useComplianceSSE } from '../hooks/useComplianceSSE'
import type { ComplianceSummary } from '../types/compliance'

// Migration status — in production, fetched from /api/v1/migration
const migrationStatus = {
  sunsetDate: '2026-09-21',
  currentStandard: '140-2',
  migrationUrgency: 'medium',
  recommendedAction:
    'FIPS 140-2 sunset approaching. Test FIPS 140-3 modules (BoringCrypto #4735 or Go native) in staging.',
}

// Deployment tier — in production, fetched from /api/v1/deployment
const deploymentTier = 'standard'

export default function DashboardPage() {
  const [sseEnabled, setSSEEnabled] = useState(false)

  const { sections, sseStatus } = useComplianceSSE({
    enabled: sseEnabled,
    fallbackSections: mockSections,
  })

  const summary: ComplianceSummary = useMemo(() => {
    const result = { total: 0, passed: 0, failed: 0, warnings: 0, unknown: 0 }
    for (const section of sections) {
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
  }, [sections])

  return (
    <Layout>
      <SunsetBanner
        sunsetDate={migrationStatus.sunsetDate}
        currentStandard={migrationStatus.currentStandard}
        migrationUrgency={migrationStatus.migrationUrgency}
        recommendedAction={migrationStatus.recommendedAction}
      />
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-4">
          <DeploymentTierBadge tier={deploymentTier} />
          <p className="text-sm text-gray-500">
            Localhost-only &mdash; air-gap friendly
          </p>
        </div>
        <div className="flex items-center gap-4">
          <SSEToggle
            enabled={sseEnabled}
            onToggle={() => setSSEEnabled(!sseEnabled)}
            status={sseStatus}
          />
          <ExportButtons sections={sections} manifest={mockManifest} summary={summary} />
        </div>
      </div>
      <SummaryBar summary={summary} />
      <BuildManifestPanel manifest={mockManifest} />
      <div>
        <h2 className="text-lg font-semibold text-gray-900 mb-4">
          Compliance Checklist
        </h2>
        {sections.map((section) => (
          <ChecklistSection key={section.id} section={section} />
        ))}
      </div>
    </Layout>
  )
}

function SSEToggle({
  enabled,
  onToggle,
  status,
}: {
  enabled: boolean
  onToggle: () => void
  status: { connected: boolean; lastUpdate: Date | null; error: string | null }
}) {
  return (
    <div className="flex items-center gap-2">
      <button
        onClick={onToggle}
        className={`relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 ${
          enabled ? 'bg-blue-600' : 'bg-gray-200'
        }`}
        role="switch"
        aria-checked={enabled}
        aria-label="Toggle live updates"
      >
        <span
          className={`pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out ${
            enabled ? 'translate-x-4' : 'translate-x-0'
          }`}
        />
      </button>
      <span className="text-xs text-gray-500">Live</span>
      {enabled && (
        <span
          className={`inline-flex items-center gap-1 text-xs ${
            status.connected ? 'text-green-600' : 'text-gray-400'
          }`}
          title={
            status.error ??
            (status.lastUpdate
              ? `Last update: ${status.lastUpdate.toLocaleTimeString()}`
              : 'Connecting...')
          }
        >
          <span
            className={`w-1.5 h-1.5 rounded-full ${
              status.connected ? 'bg-green-500 animate-pulse' : 'bg-gray-400'
            }`}
          />
          {status.connected ? 'Connected' : status.error ? 'Retrying' : 'Connecting'}
        </span>
      )}
    </div>
  )
}
