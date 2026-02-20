import { useState } from 'react'
import type { ChecklistItem as ChecklistItemType } from '../types/compliance'
import StatusBadge from './StatusBadge'

interface ChecklistItemProps {
  item: ChecklistItemType
}

export default function ChecklistItem({ item }: ChecklistItemProps) {
  const [expanded, setExpanded] = useState(false)

  return (
    <div className="border border-gray-200 rounded-lg bg-white">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full px-4 py-3 flex items-center justify-between text-left hover:bg-gray-50 transition-colors"
      >
        <div className="flex items-center gap-3 min-w-0">
          <StatusBadge status={item.status} />
          <span className="text-sm font-medium text-gray-900 truncate">
            {item.name}
          </span>
          <span className="hidden sm:inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-100 text-gray-600">
            {item.severity}
          </span>
        </div>
        <svg
          className={`w-5 h-5 text-gray-400 shrink-0 transition-transform ${
            expanded ? 'rotate-180' : ''
          }`}
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M19 9l-7 7-7-7"
          />
        </svg>
      </button>
      {expanded && (
        <div className="px-4 pb-4 border-t border-gray-100">
          <dl className="mt-3 space-y-3">
            <div>
              <dt className="text-xs font-semibold text-gray-500 uppercase tracking-wider">
                What
              </dt>
              <dd className="mt-1 text-sm text-gray-700">{item.what}</dd>
            </div>
            <div>
              <dt className="text-xs font-semibold text-gray-500 uppercase tracking-wider">
                Why
              </dt>
              <dd className="mt-1 text-sm text-gray-700">{item.why}</dd>
            </div>
            <div>
              <dt className="text-xs font-semibold text-gray-500 uppercase tracking-wider">
                Remediation
              </dt>
              <dd className="mt-1 text-sm text-gray-700">{item.remediation}</dd>
            </div>
            <div>
              <dt className="text-xs font-semibold text-gray-500 uppercase tracking-wider">
                NIST Reference
              </dt>
              <dd className="mt-1">
                <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-mono font-medium bg-blue-50 text-blue-700">
                  {item.nistRef}
                </span>
              </dd>
            </div>
          </dl>
        </div>
      )}
    </div>
  )
}
