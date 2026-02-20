import type { ComplianceSummary } from '../types/compliance'

interface SummaryBarProps {
  summary: ComplianceSummary
}

export default function SummaryBar({ summary }: SummaryBarProps) {
  const stats = [
    {
      label: 'Total Checks',
      value: summary.total,
      color: 'text-gray-900',
      bg: 'bg-gray-50',
    },
    {
      label: 'Passed',
      value: summary.passed,
      color: 'text-green-700',
      bg: 'bg-green-50',
    },
    {
      label: 'Warnings',
      value: summary.warnings,
      color: 'text-yellow-700',
      bg: 'bg-yellow-50',
    },
    {
      label: 'Failed',
      value: summary.failed,
      color: 'text-red-700',
      bg: 'bg-red-50',
    },
  ]

  const passRate =
    summary.total > 0
      ? Math.round((summary.passed / summary.total) * 100)
      : 0

  return (
    <div className="mb-8">
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-4">
        {stats.map((stat) => (
          <div
            key={stat.label}
            className={`${stat.bg} rounded-lg p-4 border border-gray-200`}
          >
            <p className="text-sm font-medium text-gray-500">{stat.label}</p>
            <p className={`text-2xl font-bold ${stat.color}`}>{stat.value}</p>
          </div>
        ))}
      </div>
      <div className="bg-white rounded-lg border border-gray-200 p-4">
        <div className="flex items-center justify-between mb-2">
          <span className="text-sm font-medium text-gray-700">
            Overall Compliance
          </span>
          <span className="text-sm font-bold text-gray-900">{passRate}%</span>
        </div>
        <div className="w-full bg-gray-200 rounded-full h-3">
          <div
            className="h-3 rounded-full transition-all duration-500"
            style={{
              width: `${passRate}%`,
              backgroundColor:
                passRate >= 90
                  ? '#16a34a'
                  : passRate >= 70
                    ? '#ca8a04'
                    : '#dc2626',
            }}
          />
        </div>
      </div>
    </div>
  )
}
