export type Status = 'pass' | 'fail' | 'warning' | 'unknown'

export type Severity = 'critical' | 'high' | 'medium' | 'low' | 'info'

export interface ChecklistItem {
  id: string
  name: string
  status: Status
  severity: Severity
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
