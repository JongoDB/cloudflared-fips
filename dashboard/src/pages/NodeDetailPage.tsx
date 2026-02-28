import { useState, useEffect, useMemo } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import Layout from '../components/Layout'
import SummaryBar from '../components/SummaryBar'
import ChecklistSection from '../components/ChecklistSection'
import type { ChecklistSection as SectionType, ComplianceSummary } from '../types/compliance'
import type { FleetNode } from '../types/fleet'

/** Extract FIPS/non-FIPS counts from a "Gateway Clients" section's checklist items. */
function parseGatewayStats(section: SectionType) {
  let total = 0
  let fips = 0
  let nonFips = 0

  for (const item of section.items) {
    if (item.id === 'gw-1') {
      // "Total client TLS connections inspected: N"
      const m = item.what?.match(/inspected:\s*(\d+)/)
      if (m) total = parseInt(m[1], 10)
    }
    if (item.id === 'gw-2') {
      // "N of M clients are FIPS-capable (P%)"
      const m = item.what?.match(/(\d+)\s+of\s+\d+\s+clients/)
      if (m) fips = parseInt(m[1], 10)
    }
    if (item.id === 'gw-3') {
      // "N non-FIPS client connections detected"
      const m = item.what?.match(/(\d+)\s+non-FIPS/)
      if (m) nonFips = parseInt(m[1], 10)
    }
  }

  return { total, fips, nonFips }
}

export default function NodeDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [node, setNode] = useState<FleetNode | null>(null)
  const [sections, setSections] = useState<SectionType[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!id) return

    const fetchData = async () => {
      try {
        setLoading(true)
        setError(null)

        const [nodeRes, reportRes] = await Promise.all([
          fetch(`/api/v1/fleet/nodes/${id}`),
          fetch(`/api/v1/fleet/nodes/${id}/report`),
        ])

        if (nodeRes.ok) {
          setNode(await nodeRes.json())
        } else {
          setError('Node not found')
          return
        }

        if (reportRes.ok) {
          const report = await reportRes.json()
          if (report?.sections) {
            setSections(report.sections)
          }
        }
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load node')
      } finally {
        setLoading(false)
      }
    }

    fetchData()
    // Refresh every 30s
    const interval = setInterval(fetchData, 30000)
    return () => clearInterval(interval)
  }, [id])

  const summary: ComplianceSummary = useMemo(() => {
    const result = { total: 0, passed: 0, failed: 0, warnings: 0, unknown: 0 }
    for (const section of sections) {
      for (const item of section.items) {
        result.total++
        switch (item.status) {
          case 'pass': result.passed++; break
          case 'fail': result.failed++; break
          case 'warning': result.warnings++; break
          case 'unknown': result.unknown++; break
        }
      }
    }
    return result
  }, [sections])

  const gatewaySection = useMemo(
    () => sections.find((s) => s.id === 'gateway'),
    [sections],
  )

  const statusColors: Record<string, string> = {
    online: 'bg-green-100 text-green-800',
    degraded: 'bg-yellow-100 text-yellow-800',
    offline: 'bg-red-100 text-red-800',
  }

  const roleColors: Record<string, string> = {
    controller: 'bg-purple-100 text-purple-800',
    server: 'bg-blue-100 text-blue-800',
    proxy: 'bg-orange-100 text-orange-800',
    client: 'bg-gray-100 text-gray-800',
  }

  return (
    <Layout>
      <div className="mb-6">
        <button
          onClick={() => navigate('/fleet')}
          className="text-sm text-blue-600 hover:text-blue-800 mb-3 inline-flex items-center gap-1"
        >
          &larr; Back to Fleet
        </button>

        {loading && !node && (
          <div className="text-center py-8 text-gray-500">Loading node details...</div>
        )}

        {error && (
          <div className="p-3 bg-red-50 border border-red-200 rounded-lg text-sm text-red-700">
            {error}
          </div>
        )}

        {node && (
          <>
            <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 mb-6">
              <div>
                <h2 className="text-lg font-semibold text-gray-900">{node.name}</h2>
                <div className="flex items-center gap-2 mt-1">
                  <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${roleColors[node.role] || 'bg-gray-100 text-gray-800'}`}>
                    {node.role}
                  </span>
                  <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${statusColors[node.status] || 'bg-gray-100 text-gray-800'}`}>
                    {node.status}
                  </span>
                  {node.region && (
                    <span className="text-xs text-gray-500">{node.region}</span>
                  )}
                </div>
              </div>
              <div className="text-right text-sm text-gray-500">
                <p>FIPS: {node.fips_backend || 'unknown'}</p>
                <p>Version: {node.version || '--'}</p>
                <p>ID: <code className="text-xs bg-gray-100 px-1 rounded">{node.id.slice(0, 12)}...</code></p>
              </div>
            </div>

            {/* Gateway Client Stats card — shown when this node has a gateway section */}
            {gatewaySection && <GatewayClientCard section={gatewaySection} />}

            {sections.length > 0 ? (
              <>
                <SummaryBar summary={summary} />
                <div>
                  <h3 className="text-base font-medium text-gray-900 mb-4">
                    Compliance Checklist
                  </h3>
                  {sections.map((section) => (
                    <ChecklistSection key={section.id} section={section} />
                  ))}
                </div>
              </>
            ) : (
              <div className="bg-white rounded-lg border border-gray-200 p-8 text-center">
                <p className="text-gray-500">No compliance report available yet.</p>
                <p className="text-sm text-gray-400 mt-1">
                  The node will send its first report within {node.role === 'client' ? '60' : '30'} seconds.
                </p>
              </div>
            )}
          </>
        )}
      </div>
    </Layout>
  )
}

/** Renders a compact card with gateway client TLS stats and a FIPS/non-FIPS bar. */
function GatewayClientCard({ section }: { section: SectionType }) {
  const { total, fips, nonFips } = parseGatewayStats(section)
  const pct = total > 0 ? Math.round((fips / total) * 100) : 0
  const hasNonFips = nonFips > 0

  return (
    <div className="bg-white rounded-lg border border-gray-200 p-4 mb-6">
      <div className="flex items-center justify-between mb-3">
        <h3 className="text-sm font-semibold text-gray-900">Gateway Client Stats</h3>
        {hasNonFips && (
          <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-yellow-100 text-yellow-800">
            {nonFips} non-FIPS
          </span>
        )}
      </div>

      <div className="grid grid-cols-3 gap-4 text-center mb-3">
        <div>
          <p className="text-2xl font-bold text-gray-900">{total}</p>
          <p className="text-xs text-gray-500">Total Connections</p>
        </div>
        <div>
          <p className="text-2xl font-bold text-green-600">{fips}</p>
          <p className="text-xs text-gray-500">FIPS-Capable</p>
        </div>
        <div>
          <p className={`text-2xl font-bold ${hasNonFips ? 'text-yellow-600' : 'text-gray-400'}`}>{nonFips}</p>
          <p className="text-xs text-gray-500">Non-FIPS</p>
        </div>
      </div>

      {total > 0 && (
        <div>
          <div className="flex justify-between text-xs text-gray-500 mb-1">
            <span>FIPS compliance</span>
            <span>{pct}%</span>
          </div>
          <div className="w-full bg-gray-200 rounded-full h-2">
            <div
              className={`h-2 rounded-full ${pct === 100 ? 'bg-green-500' : pct >= 80 ? 'bg-yellow-400' : 'bg-red-400'}`}
              style={{ width: `${pct}%` }}
            />
          </div>
        </div>
      )}
    </div>
  )
}
