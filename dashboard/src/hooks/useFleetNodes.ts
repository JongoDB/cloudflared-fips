import { useState, useEffect, useCallback } from 'react'
import type { FleetNode, FleetSummary, NodeFilter } from '../types/fleet'

interface UseFleetNodesResult {
  nodes: FleetNode[]
  summary: FleetSummary | null
  loading: boolean
  error: string | null
  refresh: () => void
  applyFilter: (filter: NodeFilter) => void
  filter: NodeFilter
}

export function useFleetNodes(): UseFleetNodesResult {
  const [nodes, setNodes] = useState<FleetNode[]>([])
  const [summary, setSummary] = useState<FleetSummary | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [filter, setFilter] = useState<NodeFilter>({})

  const fetchNodes = useCallback(async () => {
    try {
      setLoading(true)
      setError(null)

      const params = new URLSearchParams()
      if (filter.role) params.set('role', filter.role)
      if (filter.region) params.set('region', filter.region)
      if (filter.status) params.set('status', filter.status)

      const qs = params.toString()
      const url = '/api/v1/fleet/nodes' + (qs ? '?' + qs : '')

      const [nodesRes, summaryRes] = await Promise.all([
        fetch(url),
        fetch('/api/v1/fleet/summary'),
      ])

      if (nodesRes.ok) {
        const data = await nodesRes.json()
        setNodes(Array.isArray(data) ? data : [])
      } else {
        setError('Failed to fetch nodes')
      }

      if (summaryRes.ok) {
        const data = await summaryRes.json()
        setSummary(data)
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Network error')
    } finally {
      setLoading(false)
    }
  }, [filter])

  useEffect(() => {
    fetchNodes()
    // Poll every 30s
    const interval = setInterval(fetchNodes, 30000)
    return () => clearInterval(interval)
  }, [fetchNodes])

  const applyFilter = useCallback((f: NodeFilter) => {
    setFilter(f)
  }, [])

  return { nodes, summary, loading, error, refresh: fetchNodes, applyFilter, filter }
}
