import { useState, useEffect, useCallback, useRef } from 'react'
import type { FleetNode, FleetSummary, FleetEvent } from '../types/fleet'

interface UseFleetSSEOptions {
  enabled: boolean
}

interface UseFleetSSEResult {
  nodes: FleetNode[]
  summary: FleetSummary | null
  connected: boolean
  lastUpdate: Date | null
}

export function useFleetSSE({ enabled }: UseFleetSSEOptions): UseFleetSSEResult {
  const [nodes, setNodes] = useState<FleetNode[]>([])
  const [summary, setSummary] = useState<FleetSummary | null>(null)
  const [connected, setConnected] = useState(false)
  const [lastUpdate, setLastUpdate] = useState<Date | null>(null)
  const esRef = useRef<EventSource | null>(null)

  const connect = useCallback(() => {
    if (esRef.current) {
      esRef.current.close()
    }

    const es = new EventSource('/api/v1/fleet/events')
    esRef.current = es

    es.addEventListener('fleet_nodes', (e) => {
      try {
        const data = JSON.parse(e.data) as FleetNode[]
        setNodes(data)
        setLastUpdate(new Date())
      } catch { /* ignore parse errors */ }
    })

    es.addEventListener('fleet_summary', (e) => {
      try {
        const data = JSON.parse(e.data) as FleetSummary
        setSummary(data)
        setLastUpdate(new Date())
      } catch { /* ignore parse errors */ }
    })

    es.addEventListener('fleet_event', (e) => {
      try {
        const event = JSON.parse(e.data) as FleetEvent
        setNodes((prev) => {
          const idx = prev.findIndex((n) => n.id === event.node.id)
          if (event.type === 'node_removed') {
            return prev.filter((n) => n.id !== event.node.id)
          }
          if (event.type === 'node_joined' && idx === -1) {
            return [event.node, ...prev]
          }
          if (idx >= 0) {
            const updated = [...prev]
            updated[idx] = event.node
            return updated
          }
          return prev
        })
        setLastUpdate(new Date())
      } catch { /* ignore parse errors */ }
    })

    es.onopen = () => setConnected(true)
    es.onerror = () => {
      setConnected(false)
      es.close()
      // Reconnect after 5s
      setTimeout(() => {
        if (enabled) connect()
      }, 5000)
    }
  }, [enabled])

  useEffect(() => {
    if (enabled) {
      connect()
    }
    return () => {
      esRef.current?.close()
      esRef.current = null
      setConnected(false)
    }
  }, [enabled, connect])

  return { nodes, summary, connected, lastUpdate }
}
