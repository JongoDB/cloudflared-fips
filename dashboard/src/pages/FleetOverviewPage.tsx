import { useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import Layout from '../components/Layout'
import FleetHealthBar from '../components/FleetHealthBar'
import NodeTable from '../components/NodeTable'
import NodeFilters from '../components/NodeFilters'
import { useFleetNodes } from '../hooks/useFleetNodes'
import type { FleetNode, FleetSummary } from '../types/fleet'

const emptySummary: FleetSummary = {
  total_nodes: 0,
  online: 0,
  degraded: 0,
  offline: 0,
  by_role: {},
  by_region: {},
  fully_compliant: 0,
  updated_at: '',
}

export default function FleetOverviewPage() {
  const navigate = useNavigate()
  const { nodes, summary, loading, error, refresh, applyFilter, filter } = useFleetNodes()

  const regions = useMemo(() => {
    const set = new Set<string>()
    nodes.forEach((n) => { if (n.region) set.add(n.region) })
    return Array.from(set).sort()
  }, [nodes])

  const handleNodeClick = (node: FleetNode) => {
    navigate(`/fleet/nodes/${node.id}`)
  }

  return (
    <Layout>
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 mb-6">
        <div>
          <h2 className="text-lg font-semibold text-gray-900">Fleet Overview</h2>
          <p className="text-sm text-gray-500">
            Zero-trust FIPS compliance across all nodes
          </p>
        </div>
        <div className="flex items-center gap-3">
          <button
            onClick={refresh}
            className="text-sm px-3 py-1.5 border border-gray-300 rounded-md text-gray-700 hover:bg-gray-50"
          >
            Refresh
          </button>
          <button
            onClick={() => navigate('/node')}
            className="text-sm px-3 py-1.5 bg-blue-600 text-white rounded-md hover:bg-blue-700"
          >
            Local Node
          </button>
        </div>
      </div>

      {error && (
        <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-lg text-sm text-red-700">
          {error}
        </div>
      )}

      <FleetHealthBar summary={summary || emptySummary} />

      <div className="mb-4">
        <h3 className="text-base font-medium text-gray-900 mb-3">Nodes</h3>
        <NodeFilters filter={filter} onChange={applyFilter} regions={regions} />
      </div>

      {loading && nodes.length === 0 ? (
        <div className="text-center py-8 text-gray-500">Loading fleet data...</div>
      ) : (
        <NodeTable nodes={nodes} onNodeClick={handleNodeClick} />
      )}
    </Layout>
  )
}
