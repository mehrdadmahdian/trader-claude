import { useEffect, useRef } from 'react'
import { useMonitorStore } from '@/stores'
import { wsBase } from '@/lib/utils'
import type { MonitorSignal } from '@/types'

// useMonitorSignalsWS connects to /ws/monitors/signals and subscribes to
// the given monitorIds. New signals are added to the Zustand pendingSignals
// queue for toast display.
export function useMonitorSignalsWS(monitorIds: number[]) {
  const addSignal = useMonitorStore((s) => s.addSignal)
  const wsRef = useRef<WebSocket | null>(null)

  useEffect(() => {
    if (monitorIds.length === 0) return

    const ws = new WebSocket(`${wsBase()}/ws/monitors/signals`)
    wsRef.current = ws

    ws.onopen = () => {
      ws.send(JSON.stringify({ action: 'subscribe', monitor_ids: monitorIds }))
    }

    ws.onmessage = (e: MessageEvent) => {
      try {
        const sig = JSON.parse(e.data as string) as MonitorSignal
        addSignal(sig)
      } catch {
        // ignore malformed messages
      }
    }

    ws.onerror = () => {
      // suppress console errors — server may not be running locally
    }

    return () => {
      ws.close()
    }
  }, [JSON.stringify(monitorIds), addSignal]) // eslint-disable-line react-hooks/exhaustive-deps
}
