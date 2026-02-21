import { useState, useEffect, useCallback, useRef } from 'react'
import type { ChecklistSection, SSEStatus } from '../types/compliance'

interface SSEMessage {
  type: 'full' | 'patch'
  sections?: ChecklistSection[]
  updates?: Array<{
    sectionId: string
    itemId: string
    status: 'pass' | 'fail' | 'warning' | 'unknown'
  }>
  timestamp: string
}

interface UseComplianceSSEOptions {
  url?: string
  enabled?: boolean
  fallbackSections: ChecklistSection[]
}

export function useComplianceSSE({
  url = '/api/v1/compliance/stream',
  enabled = false,
  fallbackSections,
}: UseComplianceSSEOptions) {
  const [sections, setSections] = useState<ChecklistSection[]>(fallbackSections)
  const [sseStatus, setSSEStatus] = useState<SSEStatus>({
    connected: false,
    lastUpdate: null,
    error: null,
  })
  const eventSourceRef = useRef<EventSource | null>(null)
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const connect = useCallback(() => {
    if (!enabled) return

    try {
      const es = new EventSource(url)
      eventSourceRef.current = es

      es.onopen = () => {
        setSSEStatus({ connected: true, lastUpdate: new Date(), error: null })
      }

      es.onmessage = (event) => {
        try {
          const message: SSEMessage = JSON.parse(event.data)

          if (message.type === 'full' && message.sections) {
            setSections(message.sections)
          } else if (message.type === 'patch' && message.updates) {
            setSections((prev) =>
              prev.map((section) => ({
                ...section,
                items: section.items.map((item) => {
                  const update = message.updates?.find(
                    (u) => u.sectionId === section.id && u.itemId === item.id
                  )
                  return update ? { ...item, status: update.status } : item
                }),
              }))
            )
          }

          setSSEStatus((prev) => ({
            ...prev,
            connected: true,
            lastUpdate: new Date(),
          }))
        } catch {
          // Ignore malformed messages
        }
      }

      es.onerror = () => {
        es.close()
        setSSEStatus((prev) => ({
          ...prev,
          connected: false,
          error: 'Connection lost. Retrying...',
        }))

        // Reconnect after 5 seconds
        reconnectTimeoutRef.current = setTimeout(() => {
          connect()
        }, 5000)
      }
    } catch (err) {
      setSSEStatus({
        connected: false,
        lastUpdate: null,
        error: err instanceof Error ? err.message : 'Failed to connect',
      })
    }
  }, [url, enabled])

  useEffect(() => {
    connect()

    return () => {
      if (eventSourceRef.current) {
        eventSourceRef.current.close()
      }
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current)
      }
    }
  }, [connect])

  // Reset to fallback when SSE is disabled
  useEffect(() => {
    if (!enabled) {
      setSections(fallbackSections)
      setSSEStatus({ connected: false, lastUpdate: null, error: null })
    }
  }, [enabled, fallbackSections])

  return { sections, sseStatus }
}
