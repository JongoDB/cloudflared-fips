export type Status = 'pass' | 'fail' | 'warning' | 'unknown'

export type Severity = 'critical' | 'high' | 'medium' | 'low' | 'info'

export type VerificationMethod = 'direct' | 'api' | 'probe' | 'inherited' | 'reported'

export const verificationMethodInfo: Record<VerificationMethod, { label: string; description: string }> = {
  direct: {
    label: 'Direct',
    description: 'Measured locally — self-test result, binary hash, OS FIPS mode check',
  },
  api: {
    label: 'API',
    description: 'Queried from Cloudflare API — Access policy, cipher config, tunnel health',
  },
  probe: {
    label: 'Probe',
    description: 'TLS handshake inspection — negotiated cipher suite, TLS version, certificate',
  },
  inherited: {
    label: 'Inherited',
    description: "Relies on provider's FedRAMP authorization — not independently verifiable",
  },
  reported: {
    label: 'Reported',
    description: 'Client-reported via WARP or device posture — trust depends on endpoint management',
  },
}

export interface ChecklistItem {
  id: string
  name: string
  status: Status
  severity: Severity
  verificationMethod: VerificationMethod
  what: string
  why: string
  remediation: string
  nistRef: string
}

export interface ChecklistSection {
  id: string
  name: string
  description: string
  items: ChecklistItem[]
}

export interface ComplianceSummary {
  total: number
  passed: number
  failed: number
  warnings: number
  unknown: number
}

export interface BuildManifest {
  version: string
  commit: string
  build_time: string
  cloudflared_upstream_version: string
  cloudflared_upstream_commit: string
  crypto_engine: string
  boringssl_version: string
  fips_certificates: FIPSCertificate[]
  target_platform: string
  package_format: string
  sbom_sha256: string
  binary_sha256: string
}

export interface FIPSCertificate {
  module: string
  certificate: string
  algorithms: string[]
}

export interface SSEStatus {
  connected: boolean
  lastUpdate: Date | null
  error: string | null
}
