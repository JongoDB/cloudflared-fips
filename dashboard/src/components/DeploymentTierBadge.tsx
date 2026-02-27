interface DeploymentTierBadgeProps {
  tier: string // "standard" | "regional_keyless" | "self_hosted"
}

const tierInfo: Record<string, { label: string; description: string; color: string }> = {
  standard: {
    label: 'Tier 1 — Standard Tunnel',
    description: 'Edge crypto inherited from Cloudflare FedRAMP authorization',
    color: 'bg-blue-100 text-blue-800 border-blue-200',
  },
  regional_keyless: {
    label: 'Tier 2 — FIPS 140 L3 (Keyless SSL + HSM)',
    description: "Cloudflare's official FIPS 140 Level 3 architecture. Key ops flow through tunnel to customer HSM via PKCS#11.",
    color: 'bg-purple-100 text-purple-800 border-purple-200',
  },
  self_hosted: {
    label: 'Tier 3 — Self-Hosted FIPS Proxy',
    description: 'Full TLS control in GovCloud, no inherited crypto gaps',
    color: 'bg-green-100 text-green-800 border-green-200',
  },
}

export default function DeploymentTierBadge({ tier }: DeploymentTierBadgeProps) {
  const info = tierInfo[tier] ?? tierInfo.standard

  return (
    <div className={`inline-flex items-center gap-1.5 sm:gap-2 px-2 sm:px-3 py-1 sm:py-1.5 rounded-lg border text-xs sm:text-sm font-medium ${info.color}`}
      title={info.description}
    >
      <svg className="w-3.5 h-3.5 sm:w-4 sm:h-4 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
          d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"
        />
      </svg>
      <span className="truncate">{info.label}</span>
    </div>
  )
}
