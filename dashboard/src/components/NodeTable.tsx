import type { FleetNode } from '../types/fleet'

interface NodeTableProps {
  nodes: FleetNode[]
  onNodeClick: (node: FleetNode) => void
}

function StatusDot({ status }: { status: FleetNode['status'] }) {
  const colors = {
    online: 'bg-green-500',
    degraded: 'bg-yellow-500',
    offline: 'bg-red-500',
  }
  return (
    <span
      className={`inline-block w-2.5 h-2.5 rounded-full ${colors[status] || 'bg-gray-400'}`}
      title={status}
    />
  )
}

function RoleBadge({ role }: { role: FleetNode['role'] }) {
  const colors = {
    controller: 'bg-purple-100 text-purple-800',
    server: 'bg-blue-100 text-blue-800',
    proxy: 'bg-orange-100 text-orange-800',
    client: 'bg-gray-100 text-gray-800',
  }
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${colors[role] || 'bg-gray-100 text-gray-800'}`}>
      {role}
    </span>
  )
}

function ComplianceScore({ pass, fail, warn }: { pass: number; fail: number; warn: number }) {
  const total = pass + fail + warn
  if (total === 0) return <span className="text-gray-400 text-sm">--</span>

  const pct = Math.round((pass / total) * 100)
  const color = pct >= 90 ? 'text-green-700' : pct >= 70 ? 'text-yellow-700' : 'text-red-700'

  return (
    <span className={`text-sm font-medium ${color}`} title={`${pass}/${total} passed`}>
      {pct}%
    </span>
  )
}

function timeAgo(dateStr: string): string {
  if (!dateStr) return '--'
  const d = new Date(dateStr)
  const now = new Date()
  const seconds = Math.floor((now.getTime() - d.getTime()) / 1000)

  if (seconds < 10) return 'just now'
  if (seconds < 60) return `${seconds}s ago`
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`
  return `${Math.floor(seconds / 86400)}d ago`
}

export default function NodeTable({ nodes, onNodeClick }: NodeTableProps) {
  if (nodes.length === 0) {
    return (
      <div className="bg-white rounded-lg border border-gray-200 p-8 text-center">
        <p className="text-gray-500">No nodes registered yet.</p>
        <p className="text-sm text-gray-400 mt-1">
          Use <code className="bg-gray-100 px-1 rounded">install.sh --role server</code> to add nodes.
        </p>
      </div>
    )
  }

  return (
    <div className="bg-white rounded-lg border border-gray-200 overflow-hidden">
      <div className="overflow-x-auto">
        <table className="min-w-full divide-y divide-gray-200">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Status</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Name</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Role</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Region</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">FIPS Backend</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Compliance</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Last Heartbeat</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Version</th>
            </tr>
          </thead>
          <tbody className="bg-white divide-y divide-gray-200">
            {nodes.map((node) => (
              <tr
                key={node.id}
                onClick={() => onNodeClick(node)}
                className="hover:bg-blue-50 cursor-pointer transition-colors"
              >
                <td className="px-4 py-3 whitespace-nowrap">
                  <StatusDot status={node.status} />
                </td>
                <td className="px-4 py-3 whitespace-nowrap">
                  <span className="text-sm font-medium text-gray-900">{node.name}</span>
                </td>
                <td className="px-4 py-3 whitespace-nowrap">
                  <RoleBadge role={node.role} />
                </td>
                <td className="px-4 py-3 whitespace-nowrap text-sm text-gray-600">
                  {node.region || '--'}
                </td>
                <td className="px-4 py-3 whitespace-nowrap text-sm text-gray-600">
                  {node.fips_backend || '--'}
                </td>
                <td className="px-4 py-3 whitespace-nowrap">
                  <ComplianceScore
                    pass={node.compliance_pass}
                    fail={node.compliance_fail}
                    warn={node.compliance_warn}
                  />
                </td>
                <td className="px-4 py-3 whitespace-nowrap text-sm text-gray-500">
                  {timeAgo(node.last_heartbeat)}
                </td>
                <td className="px-4 py-3 whitespace-nowrap text-sm text-gray-500">
                  {node.version || '--'}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
