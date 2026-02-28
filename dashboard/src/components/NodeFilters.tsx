import type { NodeFilter, NodeRole, NodeStatus } from '../types/fleet'

interface NodeFiltersProps {
  filter: NodeFilter
  onChange: (filter: NodeFilter) => void
  regions: string[]
}

export default function NodeFilters({ filter, onChange, regions }: NodeFiltersProps) {
  const roles: (NodeRole | '')[] = ['', 'controller', 'server', 'proxy', 'client']
  const statuses: (NodeStatus | '')[] = ['', 'online', 'degraded', 'offline']

  return (
    <div className="flex flex-wrap gap-3 mb-4">
      <select
        value={filter.role || ''}
        onChange={(e) => onChange({ ...filter, role: (e.target.value || undefined) as NodeRole | undefined })}
        className="text-sm border border-gray-300 rounded-md px-3 py-1.5 bg-white text-gray-700 focus:outline-none focus:ring-2 focus:ring-blue-500"
        aria-label="Filter by role"
      >
        <option value="">All Roles</option>
        {roles.filter(Boolean).map((r) => (
          <option key={r} value={r} className="capitalize">{r}</option>
        ))}
      </select>

      <select
        value={filter.status || ''}
        onChange={(e) => onChange({ ...filter, status: (e.target.value || undefined) as NodeStatus | undefined })}
        className="text-sm border border-gray-300 rounded-md px-3 py-1.5 bg-white text-gray-700 focus:outline-none focus:ring-2 focus:ring-blue-500"
        aria-label="Filter by status"
      >
        <option value="">All Statuses</option>
        {statuses.filter(Boolean).map((s) => (
          <option key={s} value={s} className="capitalize">{s}</option>
        ))}
      </select>

      {regions.length > 0 && (
        <select
          value={filter.region || ''}
          onChange={(e) => onChange({ ...filter, region: e.target.value || undefined })}
          className="text-sm border border-gray-300 rounded-md px-3 py-1.5 bg-white text-gray-700 focus:outline-none focus:ring-2 focus:ring-blue-500"
          aria-label="Filter by region"
        >
          <option value="">All Regions</option>
          {regions.map((r) => (
            <option key={r} value={r}>{r}</option>
          ))}
        </select>
      )}

      {(filter.role || filter.status || filter.region) && (
        <button
          onClick={() => onChange({})}
          className="text-sm text-blue-600 hover:text-blue-800 px-2 py-1"
        >
          Clear filters
        </button>
      )}
    </div>
  )
}
