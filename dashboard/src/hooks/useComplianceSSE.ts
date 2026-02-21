import { useState, useEffect } from 'react'
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
  const [liveSections, setLiveSections] = useState<ChecklistSection[] | null>(null)
  const [sseStatus, setSSEStatus] = useState<SSEStatus>({
    connected: false,
    lastUpdate: null,
    error: null,
  })

  useEffect(() => {
    if (!enabled) return

    let es: EventSource | null = null
    let reconnectTimeout: ReturnType<typeof setTimeout> | null = null
    let cancelled = false

    function attemptConnect() {
      if (cancelled) return

      try {
        es = new EventSource(url)
      } catch {
        // EventSource constructor failed — retry after delay
        reconnectTimeout = setTimeout(attemptConnect, 5000)
        return
      }

      es.onopen = () => {
        if (!cancelled) {
          setSSEStatus({ connected: true, lastUpdate: new Date(), error: null })
        }
      }

      es.onmessage = (event) => {
        if (cancelled) return
        try {
          const message: SSEMessage = JSON.parse(event.data)

          if (message.type === 'full' && message.sections) {
            setLiveSections(message.sections)
          } else if (message.type === 'patch' && message.updates) {
            setLiveSections((prev) => {
              if (!prev) return prev
              return prev.map((section) => ({
                ...section,
                items: section.items.map((item) => {
                  const update = message.updates?.find(
                    (u) => u.sectionId === section.id && u.itemId === item.id
                  )
                  return update ? { ...item, status: update.status } : item
                }),
              }))
            })
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
        if (cancelled) return
        es?.close()
        setSSEStatus((prev) => ({
          ...prev,
          connected: false,
          error: 'Connection lost. Retrying...',
        }))
        reconnectTimeout = setTimeout(attemptConnect, 5000)
      }
    }

    attemptConnect()

    return () => {
      cancelled = true
      es?.close()
      if (reconnectTimeout) clearTimeout(reconnectTimeout)
    }
  }, [url, enabled])

  // Derive sections: use live data when enabled and available, otherwise fallback
  const sections = enabled && liveSections ? liveSections : fallbackSections

  // Derive status: reset when disabled (no effect needed)
  const effectiveStatus = enabled
    ? sseStatus
    : { connected: false, lastUpdate: null, error: null }

  return { sections, sseStatus: effectiveStatus }
}
