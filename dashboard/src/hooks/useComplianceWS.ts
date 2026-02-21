import { useCallback, useEffect, useRef, useState } from 'react'
import type { ChecklistSection } from '../types/compliance'

interface UseComplianceWSOptions {
  /** Whether the connection is enabled */
  enabled: boolean
  /** Fallback sections when not connected */
  fallbackSections: ChecklistSection[]
  /** WebSocket URL (default: ws://localhost:8080/api/v1/ws) */
  wsUrl?: string
  /** Reconnect delay in ms (default: 3000) */
  reconnectDelay?: number
  /** Maximum reconnect attempts (default: 5) */
  maxReconnectAttempts?: number
}

interface WSStatus {
  connected: boolean
  transport: 'websocket' | 'sse' | 'none'
  lastUpdate: Date | null
  error: string | null
  reconnectAttempts: number
}

/**
 * Hook for real-time compliance updates via WebSocket with SSE fallback.
 *
 * Attempts WebSocket first. If the server doesn't support it (returns 501),
 * falls back to the existing SSE endpoint (/api/v1/events).
 */
export function useComplianceWS({
  enabled,
  fallbackSections,
  wsUrl,
  reconnectDelay = 3000,
  maxReconnectAttempts = 5,
}: UseComplianceWSOptions) {
  const [sections, setSections] = useState<ChecklistSection[]>(fallbackSections)
  const [status, setStatus] = useState<WSStatus>({
    connected: false,
    transport: 'none',
    lastUpdate: null,
    error: null,
    reconnectAttempts: 0,
  })

  const wsRef = useRef<WebSocket | null>(null)
  const sseRef = useRef<EventSource | null>(null)
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const attemptsRef = useRef(0)

  const defaultWsUrl = wsUrl ?? `ws://${window.location.host}/api/v1/ws`
  const sseUrl = `/api/v1/events`

  const connectSSE = useCallback(() => {
    if (sseRef.current) return

    const source = new EventSource(sseUrl)
    sseRef.current = source

    source.addEventListener('compliance', (event) => {
      try {
        const data = JSON.parse(event.data)
        if (data.sections) {
          setSections(data.sections)
        }
        setStatus((prev) => ({
          ...prev,
          connected: true,
          transport: 'sse',
          lastUpdate: new Date(),
          error: null,
        }))
      } catch {
        // Ignore parse errors
      }
    })

    source.onerror = () => {
      setStatus((prev) => ({
        ...prev,
        connected: false,
        error: 'SSE connection lost',
      }))
    }

    source.onopen = () => {
      setStatus((prev) => ({
        ...prev,
        connected: true,
        transport: 'sse',
        error: null,
      }))
    }
  }, [sseUrl])

  const connect = useCallback(() => {
    if (!enabled) return

    // Try WebSocket first
    try {
      const ws = new WebSocket(defaultWsUrl)
      wsRef.current = ws

      ws.onopen = () => {
        attemptsRef.current = 0
        setStatus({
          connected: true,
          transport: 'websocket',
          lastUpdate: new Date(),
          error: null,
          reconnectAttempts: 0,
        })
      }

      ws.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data)
          if (data.type === 'compliance' && data.data?.sections) {
            setSections(data.data.sections)
          } else if (data.sections) {
            setSections(data.sections)
          }
          setStatus((prev) => ({
            ...prev,
            lastUpdate: new Date(),
          }))
        } catch {
          // Ignore parse errors
        }
      }

      ws.onerror = () => {
        // WebSocket failed — fall back to SSE
        ws.close()
        wsRef.current = null
        connectSSE()
      }

      ws.onclose = (event) => {
        wsRef.current = null
        if (event.code !== 1000 && enabled) {
          // Abnormal close — try reconnecting
          attemptsRef.current++
          if (attemptsRef.current <= maxReconnectAttempts) {
            setStatus((prev) => ({
              ...prev,
              connected: false,
              error: `Reconnecting (${attemptsRef.current}/${maxReconnectAttempts})`,
              reconnectAttempts: attemptsRef.current,
            }))
            reconnectTimerRef.current = setTimeout(connect, reconnectDelay)
          } else {
            // Max attempts reached — fall back to SSE
            connectSSE()
          }
        }
      }
    } catch {
      // WebSocket not available — fall back to SSE
      connectSSE()
    }
  }, [enabled, defaultWsUrl, connectSSE, reconnectDelay, maxReconnectAttempts])

  useEffect(() => {
    if (enabled) {
      connect()
    }

    return () => {
      if (wsRef.current) {
        wsRef.current.close(1000)
        wsRef.current = null
      }
      if (sseRef.current) {
        sseRef.current.close()
        sseRef.current = null
      }
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current)
        reconnectTimerRef.current = null
      }
      setStatus({
        connected: false,
        transport: 'none',
        lastUpdate: null,
        error: null,
        reconnectAttempts: 0,
      })
    }
  }, [enabled, connect])

  return { sections, wsStatus: status }
}
