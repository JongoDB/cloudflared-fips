export type NodeRole = 'controller' | 'server' | 'proxy' | 'client'
export type NodeStatus = 'online' | 'degraded' | 'offline'

export interface FleetNode {
  id: string
  name: string
  role: NodeRole
  region: string
  labels: Record<string, string>
  enrolled_at: string
  last_heartbeat: string
  status: NodeStatus
  version: string
  fips_backend: string
  compliance_pass: number
  compliance_fail: number
  compliance_warn: number
}

export interface FleetSummary {
  total_nodes: number
  online: number
  degraded: number
  offline: number
  by_role: Record<string, number>
  by_region: Record<string, number>
  fully_compliant: number
  updated_at: string
}

export interface EnrollmentToken {
  id: string
  token?: string
  role: NodeRole
  region: string
  expires_at: string
  max_uses: number
  used_count: number
  created_at: string
}

export interface FleetEvent {
  type: 'node_joined' | 'node_updated' | 'node_offline' | 'node_removed'
  node: FleetNode
  time: string
}

export interface NodeFilter {
  role?: NodeRole
  region?: string
  status?: NodeStatus
}
