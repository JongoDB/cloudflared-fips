import type { VerificationMethod } from '../types/compliance'
import { verificationMethodInfo } from '../types/compliance'

interface VerificationBadgeProps {
  method: VerificationMethod
}

const methodStyles: Record<VerificationMethod, { bg: string; text: string; border: string }> = {
  direct: { bg: 'bg-emerald-50', text: 'text-emerald-700', border: 'border-emerald-200' },
  api: { bg: 'bg-blue-50', text: 'text-blue-700', border: 'border-blue-200' },
  probe: { bg: 'bg-violet-50', text: 'text-violet-700', border: 'border-violet-200' },
  inherited: { bg: 'bg-amber-50', text: 'text-amber-700', border: 'border-amber-200' },
  reported: { bg: 'bg-orange-50', text: 'text-orange-700', border: 'border-orange-200' },
}

const methodIcons: Record<VerificationMethod, string> = {
  direct: '\u2713',    // checkmark
  api: '\u21c4',       // left right arrow
  probe: '\u2702',     // scissors (probe/cut)
  inherited: '\u21b3', // downwards arrow with tip rightwards
  reported: '\u2709',  // envelope
}

export default function VerificationBadge({ method }: VerificationBadgeProps) {
  const styles = methodStyles[method]
  const info = verificationMethodInfo[method]
  const icon = methodIcons[method]

  return (
    <span
      className={`inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-xs font-medium border ${styles.bg} ${styles.text} ${styles.border}`}
      title={info.description}
    >
      <span aria-hidden="true">{icon}</span>
      {info.label}
    </span>
  )
}
