import { useState } from 'react'
import type { ChecklistSection as ChecklistSectionType } from '../types/compliance'
import ChecklistItem from './ChecklistItem'
import StatusBadge from './StatusBadge'
import type { Status } from '../types/compliance'

interface ChecklistSectionProps {
  section: ChecklistSectionType
}

function getSectionStatus(section: ChecklistSectionType): Status {
  const hasFailure = section.items.some((item) => item.status === 'fail')
  if (hasFailure) return 'fail'
  const hasWarning = section.items.some((item) => item.status === 'warning')
  if (hasWarning) return 'warning'
  const hasUnknown = section.items.some((item) => item.status === 'unknown')
  if (hasUnknown) return 'unknown'
  return 'pass'
}

export default function ChecklistSection({ section }: ChecklistSectionProps) {
  const [collapsed, setCollapsed] = useState(false)
  const sectionStatus = getSectionStatus(section)
  const passCount = section.items.filter((i) => i.status === 'pass').length

  return (
    <div className="mb-6">
      <button
        onClick={() => setCollapsed(!collapsed)}
        className="w-full flex items-center justify-between px-4 py-3 bg-white border border-gray-200 rounded-lg shadow-sm hover:bg-gray-50 transition-colors"
      >
        <div className="flex items-center gap-3">
          <svg
            className={`w-5 h-5 text-gray-400 transition-transform ${
              collapsed ? '-rotate-90' : ''
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
          <div className="text-left">
            <h3 className="text-base font-semibold text-gray-900">
              {section.name}
            </h3>
            <p className="text-sm text-gray-500">{section.description}</p>
          </div>
        </div>
        <div className="flex items-center gap-3 shrink-0">
          <span className="text-sm text-gray-500">
            {passCount}/{section.items.length} passed
          </span>
          <StatusBadge status={sectionStatus} size="md" />
        </div>
      </button>
      {!collapsed && (
        <div className="mt-2 space-y-2 pl-4">
          {section.items.map((item) => (
            <ChecklistItem key={item.id} item={item} />
          ))}
        </div>
      )}
    </div>
  )
}
