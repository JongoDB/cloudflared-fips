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
  schema_version: string
  build_id: string
  timestamp: string
  source: {
    repository: string
    branch: string
    commit: string
    tag: string
  }
  platform: {
    build_os: string
    build_arch: string
    target_os: string
    target_arch: string
    builder: string
  }
  go_version: string
  go_experiment: string
  cgo_enabled: boolean
  fips_certificates: FIPSCertificate[]
  binary: {
    name: string
    path: string
    sha256: string
    size: number
    stripped: boolean
  }
  sbom: {
    format: string
    path: string
    sha256: string
  }
  verification: {
    boring_crypto_symbols: boolean
    self_test_passed: boolean
    banned_ciphers: string[]
    static_linked: boolean
  }
}

export interface FIPSCertificate {
  module: string
  cert_number: string
  level: string
  validated_on: string
  expires_on: string
  cmvp_url: string
}
