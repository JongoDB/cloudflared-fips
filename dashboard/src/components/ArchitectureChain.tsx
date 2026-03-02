import type { ChecklistSection, Status } from '../types/compliance'

interface ArchitectureChainProps {
  sections: ChecklistSection[]
}

interface ChainNode {
  sectionId: string
  label: string
  shortLabel: string
  connectionLabel?: string
  cryptoNote: string
  verificationNote: string
}

const CHAIN_NODES: ChainNode[] = [
  {
    sectionId: 'client-posture',
    label: 'Client Posture',
    shortLabel: 'Client',
    connectionLabel: 'HTTPS (TLS 1.2+)',
    cryptoNote: 'FIPS OS',
    verificationNote: 'Reported',
  },
  {
    sectionId: 'cloudflare-edge',
    label: 'Cloudflare Edge',
    shortLabel: 'Edge',
    connectionLabel: 'QUIC / HTTP/2',
    cryptoNote: 'FedRAMP',
    verificationNote: 'Inherited',
  },
  {
    sectionId: 'tunnel',
    label: 'cloudflared Tunnel',
    shortLabel: 'Tunnel',
    connectionLabel: 'TLS / Loopback',
    cryptoNote: 'BoringCrypto',
    verificationNote: 'Direct',
  },
  {
    sectionId: 'local-service',
    label: 'Origin Service',
    shortLabel: 'Origin',
    cryptoNote: 'Loopback',
    verificationNote: 'Probe',
  },
]

function getSectionStatus(section: ChecklistSection): Status {
  if (section.items.some((i) => i.status === 'fail')) return 'fail'
  if (section.items.some((i) => i.status === 'warning')) return 'warning'
  if (section.items.some((i) => i.status === 'unknown')) return 'unknown'
  return 'pass'
}

const statusColors: Record<Status, { bg: string; border: string; text: string; dot: string }> = {
  pass: { bg: 'bg-green-50', border: 'border-green-300', text: 'text-green-700', dot: 'bg-green-500' },
  warning: { bg: 'bg-yellow-50', border: 'border-yellow-300', text: 'text-yellow-700', dot: 'bg-yellow-500' },
  fail: { bg: 'bg-red-50', border: 'border-red-300', text: 'text-red-700', dot: 'bg-red-500' },
  unknown: { bg: 'bg-gray-50', border: 'border-gray-300', text: 'text-gray-500', dot: 'bg-gray-400' },
}

const statusIcon: Record<Status, string> = {
  pass: '\u2713',
  warning: '\u26A0',
  fail: '\u2717',
  unknown: '?',
}

function scrollToSection(sectionId: string) {
  const el = document.getElementById(`section-${sectionId}`)
  if (el) el.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

export default function ArchitectureChain({ sections }: ArchitectureChainProps) {
  const sectionMap = new Map(sections.map((s) => [s.id, s]))

  return (
    <div className="mb-6">
      {/* Desktop: horizontal chain */}
      <div className="hidden md:flex items-stretch justify-center gap-0">
        {CHAIN_NODES.map((node, idx) => {
          const section = sectionMap.get(node.sectionId)
          const status: Status = section ? getSectionStatus(section) : 'unknown'
          const passCount = section ? section.items.filter((i) => i.status === 'pass').length : 0
          const totalCount = section ? section.items.length : 0
          const colors = statusColors[status]

          return (
            <div key={node.sectionId} className="flex items-center">
              <button
                onClick={() => scrollToSection(node.sectionId)}
                className={`relative flex flex-col items-center justify-center px-5 py-3 rounded-lg border-2 ${colors.bg} ${colors.border} hover:shadow-md transition-shadow cursor-pointer min-w-[140px]`}
                title={`Scroll to ${node.label}`}
              >
                <span className="text-xs font-semibold text-gray-900">{node.label}</span>
                <span className={`text-lg font-bold ${colors.text}`}>
                  {passCount}/{totalCount} {statusIcon[status]}
                </span>
                <span className="text-[10px] font-medium text-gray-500">{node.cryptoNote}</span>
                <span className={`text-[10px] italic ${colors.text}`}>({node.verificationNote})</span>
              </button>
              {node.connectionLabel && idx < CHAIN_NODES.length - 1 && (
                <div className="flex flex-col items-center px-2 shrink-0">
                  <span className="text-[10px] text-gray-400 whitespace-nowrap">{node.connectionLabel}</span>
                  <svg className="w-8 h-4 text-gray-300" viewBox="0 0 32 16" fill="none">
                    <line x1="0" y1="8" x2="24" y2="8" stroke="currentColor" strokeWidth="2" />
                    <path d="M22 4 L30 8 L22 12" fill="currentColor" />
                  </svg>
                </div>
              )}
            </div>
          )
        })}
      </div>

      {/* Mobile: vertical chain */}
      <div className="flex md:hidden flex-col items-center gap-0">
        {CHAIN_NODES.map((node, idx) => {
          const section = sectionMap.get(node.sectionId)
          const status: Status = section ? getSectionStatus(section) : 'unknown'
          const passCount = section ? section.items.filter((i) => i.status === 'pass').length : 0
          const totalCount = section ? section.items.length : 0
          const colors = statusColors[status]

          return (
            <div key={node.sectionId} className="flex flex-col items-center">
              <button
                onClick={() => scrollToSection(node.sectionId)}
                className={`flex items-center gap-3 w-full max-w-xs px-4 py-2.5 rounded-lg border-2 ${colors.bg} ${colors.border} hover:shadow-md transition-shadow cursor-pointer`}
                title={`Scroll to ${node.label}`}
              >
                <span className={`w-2.5 h-2.5 rounded-full shrink-0 ${colors.dot}`} />
                <div className="flex-1 min-w-0">
                  <span className="text-sm font-semibold text-gray-900">{node.label}</span>
                  <span className={`ml-2 text-sm font-bold ${colors.text}`}>
                    {passCount}/{totalCount} {statusIcon[status]}
                  </span>
                </div>
                <span className="text-[10px] text-gray-400">{node.verificationNote}</span>
              </button>
              {node.connectionLabel && idx < CHAIN_NODES.length - 1 && (
                <div className="flex flex-col items-center py-1">
                  <svg className="w-4 h-6 text-gray-300" viewBox="0 0 16 24" fill="none">
                    <line x1="8" y1="0" x2="8" y2="18" stroke="currentColor" strokeWidth="2" />
                    <path d="M4 16 L8 23 L12 16" fill="currentColor" />
                  </svg>
                  <span className="text-[10px] text-gray-400">{node.connectionLabel}</span>
                </div>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}
