import type { FleetSummary } from '../types/fleet'

interface FleetHealthBarProps {
  summary: FleetSummary
}

export default function FleetHealthBar({ summary }: FleetHealthBarProps) {
  const total = summary.total_nodes || 1
  const onlinePct = Math.round((summary.online / total) * 100)
  const degradedPct = Math.round((summary.degraded / total) * 100)
  const offlinePct = Math.round((summary.offline / total) * 100)

  const stats = [
    { label: 'Total Nodes', value: summary.total_nodes, color: 'text-gray-900', bg: 'bg-gray-50' },
    { label: 'Online', value: summary.online, color: 'text-green-700', bg: 'bg-green-50' },
    { label: 'Degraded', value: summary.degraded, color: 'text-yellow-700', bg: 'bg-yellow-50' },
    { label: 'Offline', value: summary.offline, color: 'text-red-700', bg: 'bg-red-50' },
    { label: 'Fully Compliant', value: summary.fully_compliant, color: 'text-blue-700', bg: 'bg-blue-50' },
  ]

  return (
    <div className="mb-8">
      <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-5 gap-4 mb-4">
        {stats.map((stat) => (
          <div key={stat.label} className={`${stat.bg} rounded-lg p-4 border border-gray-200`}>
            <p className="text-sm font-medium text-gray-500">{stat.label}</p>
            <p className={`text-2xl font-bold ${stat.color}`}>{stat.value}</p>
          </div>
        ))}
      </div>
      <div className="bg-white rounded-lg border border-gray-200 p-4">
        <div className="flex items-center justify-between mb-2">
          <span className="text-sm font-medium text-gray-700">Fleet Health</span>
          <span className="text-sm text-gray-500">
            {onlinePct}% online
          </span>
        </div>
        <div className="w-full bg-gray-200 rounded-full h-3 flex overflow-hidden">
          {summary.online > 0 && (
            <div
              className="h-3 bg-green-500 transition-all duration-500"
              style={{ width: `${onlinePct}%` }}
              title={`${summary.online} online`}
            />
          )}
          {summary.degraded > 0 && (
            <div
              className="h-3 bg-yellow-500 transition-all duration-500"
              style={{ width: `${degradedPct}%` }}
              title={`${summary.degraded} degraded`}
            />
          )}
          {summary.offline > 0 && (
            <div
              className="h-3 bg-red-500 transition-all duration-500"
              style={{ width: `${offlinePct}%` }}
              title={`${summary.offline} offline`}
            />
          )}
        </div>
      </div>

      {/* Role and region breakdown */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mt-4">
        {Object.keys(summary.by_role).length > 0 && (
          <div className="bg-white rounded-lg border border-gray-200 p-4">
            <p className="text-sm font-medium text-gray-700 mb-2">By Role</p>
            <div className="space-y-1">
              {Object.entries(summary.by_role).map(([role, count]) => (
                <div key={role} className="flex justify-between text-sm">
                  <span className="text-gray-600 capitalize">{role}</span>
                  <span className="font-medium text-gray-900">{count}</span>
                </div>
              ))}
            </div>
          </div>
        )}
        {Object.keys(summary.by_region).length > 0 && (
          <div className="bg-white rounded-lg border border-gray-200 p-4">
            <p className="text-sm font-medium text-gray-700 mb-2">By Region</p>
            <div className="space-y-1">
              {Object.entries(summary.by_region).map(([region, count]) => (
                <div key={region} className="flex justify-between text-sm">
                  <span className="text-gray-600">{region}</span>
                  <span className="font-medium text-gray-900">{count}</span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
