interface SunsetBannerProps {
  sunsetDate: string   // ISO date string "2026-09-21"
  currentStandard: string  // "140-2", "140-3", "140-3 (pending)"
  migrationUrgency: string // "none" | "low" | "medium" | "high" | "critical"
  recommendedAction: string
}

export default function SunsetBanner({
  sunsetDate,
  currentStandard,
  migrationUrgency,
  recommendedAction,
}: SunsetBannerProps) {
  if (migrationUrgency === 'none') return null

  const sunset = new Date(sunsetDate)
  const now = new Date()
  const daysUntil = Math.ceil((sunset.getTime() - now.getTime()) / (1000 * 60 * 60 * 24))

  const urgencyStyles: Record<string, { bg: string; border: string; icon: string; text: string }> = {
    low: {
      bg: 'bg-blue-50',
      border: 'border-blue-200',
      icon: 'text-blue-500',
      text: 'text-blue-800',
    },
    medium: {
      bg: 'bg-yellow-50',
      border: 'border-yellow-200',
      icon: 'text-yellow-500',
      text: 'text-yellow-800',
    },
    high: {
      bg: 'bg-orange-50',
      border: 'border-orange-200',
      icon: 'text-orange-500',
      text: 'text-orange-800',
    },
    critical: {
      bg: 'bg-red-50',
      border: 'border-red-200',
      icon: 'text-red-500',
      text: 'text-red-800',
    },
  }

  const style = urgencyStyles[migrationUrgency] ?? urgencyStyles.medium

  return (
    <div className={`${style.bg} ${style.border} border rounded-lg p-4 mb-6`}>
      <div className="flex items-start gap-3">
        <div className={`${style.icon} mt-0.5 shrink-0`}>
          {migrationUrgency === 'critical' ? (
            <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
              <path fillRule="evenodd" d="M8.485 2.495c.673-1.167 2.357-1.167 3.03 0l6.28 10.875c.673 1.167-.17 2.625-1.516 2.625H3.72c-1.347 0-2.189-1.458-1.515-2.625L8.485 2.495zM10 6a.75.75 0 01.75.75v3.5a.75.75 0 01-1.5 0v-3.5A.75.75 0 0110 6zm0 9a1 1 0 100-2 1 1 0 000 2z" clipRule="evenodd" />
            </svg>
          ) : (
            <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
              <path fillRule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a.75.75 0 000 1.5h.253a.25.25 0 01.244.304l-.459 2.066A1.75 1.75 0 0010.747 15H11a.75.75 0 000-1.5h-.253a.25.25 0 01-.244-.304l.459-2.066A1.75 1.75 0 009.253 9H9z" clipRule="evenodd" />
            </svg>
          )}
        </div>
        <div className="flex-1">
          <div className="flex items-center justify-between">
            <h3 className={`text-sm font-semibold ${style.text}`}>
              FIPS 140-2 Sunset{' '}
              {daysUntil > 0 ? (
                <span>in {daysUntil} days</span>
              ) : (
                <span>(passed)</span>
              )}
            </h3>
            <span className={`text-xs font-mono ${style.text} opacity-70`}>
              {sunsetDate}
            </span>
          </div>
          <p className={`text-sm ${style.text} mt-1`}>
            Current module: FIPS {currentStandard}.{' '}
            {recommendedAction}
          </p>

          {/* Progress bar showing time until sunset */}
          {daysUntil > 0 && (
            <div className="mt-3">
              <div className="flex justify-between text-xs text-gray-500 mb-1">
                <span>Today</span>
                <span>Sept 21, 2026</span>
              </div>
              <div className="w-full bg-gray-200 rounded-full h-1.5">
                <div
                  className={`h-1.5 rounded-full ${
                    migrationUrgency === 'critical'
                      ? 'bg-red-500'
                      : migrationUrgency === 'high'
                      ? 'bg-orange-500'
                      : migrationUrgency === 'medium'
                      ? 'bg-yellow-500'
                      : 'bg-blue-500'
                  }`}
                  style={{
                    width: `${Math.min(100, Math.max(5, 100 - (daysUntil / 365) * 100))}%`,
                  }}
                />
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
