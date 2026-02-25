import { useEffect, useRef, useCallback } from 'react'
import { useBacktestStore } from '@/stores'
import type { ReplayControlMsg } from '@/types'

const WS_URL = (import.meta.env.VITE_WS_URL ?? 'ws://localhost:8080') as string

export function useReplayWS(replayId: string | null) {
  const wsRef = useRef<WebSocket | null>(null)
  const applyReplayMsg = useBacktestStore((s) => s.applyReplayMsg)

  useEffect(() => {
    if (!replayId) return

    const ws = new WebSocket(`${WS_URL}/ws/replay/${replayId}`)
    wsRef.current = ws

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data as string)
        applyReplayMsg(msg)
      } catch {
        // ignore malformed messages
      }
    }

    ws.onerror = (e) => {
      console.error('[useReplayWS] WebSocket error', e)
    }

    return () => {
      ws.close()
      wsRef.current = null
    }
  }, [replayId, applyReplayMsg])

  const sendControl = useCallback((msg: ReplayControlMsg) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(msg))
    }
  }, [])

  return { sendControl }
}
