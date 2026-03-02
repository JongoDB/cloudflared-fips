import type { FleetSummary, NodeRole } from '../types/fleet'

interface FleetTopologyProps {
  summary: FleetSummary
  onRoleClick: (role: NodeRole | undefined) => void
  activeRole?: NodeRole
}

interface RoleInfo {
  role: NodeRole
  label: string
  icon: string
  count: number
}

function getRoleStatus(count: number): 'active' | 'empty' {
  return count > 0 ? 'active' : 'empty'
}

function getRoleBorderColor(count: number): string {
  if (count === 0) return 'border-gray-300 bg-gray-50'
  return 'border-green-400 bg-green-50'
}

function getRoleIconBg(count: number): string {
  if (count === 0) return 'bg-gray-200 text-gray-400'
  return 'bg-green-100 text-green-700'
}

export default function FleetTopology({ summary, onRoleClick, activeRole }: FleetTopologyProps) {
  const byRole = summary.by_role || {}

  const controllerCount = byRole['controller'] || 0
  const childRoles: RoleInfo[] = [
    { role: 'server', label: 'Server', icon: '\u{1F5A5}', count: byRole['server'] || 0 },
    { role: 'proxy', label: 'Proxy', icon: '\u{1F6E1}', count: byRole['proxy'] || 0 },
    { role: 'client', label: 'Client', icon: '\u{1F4BB}', count: byRole['client'] || 0 },
  ]

  const controllerOnline = controllerCount > 0

  return (
    <div className="mb-6">
      <div className="flex flex-col items-center">
        {/* Controller node at top */}
        <button
          onClick={() => onRoleClick(activeRole === 'controller' ? undefined : 'controller')}
          className={`
            relative rounded-lg border-2 px-6 py-3 min-w-[180px] text-center cursor-pointer
            transition-all duration-200 hover:shadow-md
            ${activeRole === 'controller' ? 'ring-2 ring-blue-500 ring-offset-2' : ''}
            ${getRoleBorderColor(controllerCount)}
          `}
        >
          <div className="flex items-center justify-center gap-2 mb-1">
            <span className={`inline-flex items-center justify-center w-6 h-6 rounded-full text-xs ${getRoleIconBg(controllerCount)}`}>
              {'\u{1F451}'}
            </span>
            <span className="text-sm font-semibold text-gray-800 uppercase tracking-wide">Controller</span>
          </div>
          <div className="text-xs text-gray-600">
            {controllerCount > 0 ? (
              <>
                <span className={`inline-block w-2 h-2 rounded-full mr-1 ${controllerOnline ? 'bg-green-500' : 'bg-gray-400'}`} />
                {controllerCount} node{controllerCount !== 1 ? 's' : ''}
              </>
            ) : (
              <span className="text-gray-400">No nodes</span>
            )}
          </div>
        </button>

        {/* Connection lines */}
        <div className="relative w-full flex justify-center" style={{ height: '40px' }}>
          {/* Vertical line from controller */}
          <div className="absolute top-0 left-1/2 w-px h-3 bg-gray-300 -translate-x-px" />
          {/* Horizontal connector bar */}
          <div className="absolute top-3 bg-gray-300" style={{
            left: `calc(${100 / 6}% + 45px)`,
            right: `calc(${100 / 6}% + 45px)`,
            height: '1px',
          }} />
          {/* Vertical drops to each child */}
          {childRoles.map((_, i) => (
            <div
              key={i}
              className="absolute bg-gray-300"
              style={{
                top: '12px',
                left: `calc(${(2 * i + 1) * 100 / 6}%)`,
                width: '1px',
                height: '28px',
                transform: 'translateX(-0.5px)',
              }}
            />
          ))}
        </div>

        {/* Child role nodes */}
        <div className="grid grid-cols-3 gap-4 w-full max-w-2xl">
          {childRoles.map((info) => {
            const status = getRoleStatus(info.count)
            return (
              <button
                key={info.role}
                onClick={() => onRoleClick(activeRole === info.role ? undefined : info.role)}
                className={`
                  rounded-lg border-2 px-4 py-3 text-center cursor-pointer
                  transition-all duration-200 hover:shadow-md
                  ${activeRole === info.role ? 'ring-2 ring-blue-500 ring-offset-2' : ''}
                  ${getRoleBorderColor(info.count)}
                `}
              >
                <div className="flex items-center justify-center gap-2 mb-1">
                  <span className={`inline-flex items-center justify-center w-6 h-6 rounded-full text-xs ${getRoleIconBg(info.count)}`}>
                    {info.icon}
                  </span>
                  <span className="text-sm font-semibold text-gray-800 uppercase tracking-wide">{info.label}</span>
                </div>
                <div className="text-xs text-gray-600">
                  {info.count > 0 ? (
                    <>
                      <span className={`inline-block w-2 h-2 rounded-full mr-1 ${status === 'active' ? 'bg-green-500' : 'bg-gray-400'}`} />
                      {info.count} node{info.count !== 1 ? 's' : ''}
                    </>
                  ) : (
                    <span className="text-gray-400">No nodes</span>
                  )}
                </div>
              </button>
            )
          })}
        </div>
      </div>
    </div>
  )
}
