import type { Status } from '../types/compliance'

interface StatusBadgeProps {
  status: Status
  size?: 'sm' | 'md'
}

const statusConfig: Record<Status, { bg: string; text: string; label: string }> = {
  pass: { bg: 'bg-green-100', text: 'text-green-800', label: 'Pass' },
  fail: { bg: 'bg-red-100', text: 'text-red-800', label: 'Fail' },
  warning: { bg: 'bg-yellow-100', text: 'text-yellow-800', label: 'Warning' },
  unknown: { bg: 'bg-gray-100', text: 'text-gray-800', label: 'Unknown' },
}

const dotColor: Record<Status, string> = {
  pass: 'bg-green-500',
  fail: 'bg-red-500',
  warning: 'bg-yellow-500',
  unknown: 'bg-gray-400',
}

export default function StatusBadge({ status, size = 'sm' }: StatusBadgeProps) {
  const config = statusConfig[status]
  const sizeClasses = size === 'sm' ? 'px-2 py-0.5 text-xs' : 'px-2.5 py-1 text-sm'

  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full font-medium ${config.bg} ${config.text} ${sizeClasses}`}
    >
      <span className={`w-1.5 h-1.5 rounded-full ${dotColor[status]}`} />
      {config.label}
    </span>
  )
}
