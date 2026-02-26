import { useEffect, useRef } from 'react'
import { usePortfolioStore } from '@/stores'
import type { PortfolioUpdateMsg } from '@/types'

const WS_URL =
  (import.meta.env.VITE_WS_URL as string | undefined) ?? 'ws://localhost:8080'

export function usePortfolioLive(portfolioId: number | null) {
  const { applyLiveUpdate } = usePortfolioStore()
  const wsRef = useRef<WebSocket | null>(null)

  useEffect(() => {
    if (!portfolioId) return

    const ws = new WebSocket(`${WS_URL}/ws/portfolio/${portfolioId}/live`)
    wsRef.current = ws

    ws.onmessage = (event) => {
      try {
        const msg: PortfolioUpdateMsg = JSON.parse(event.data as string)
        if (msg.type === 'portfolio_update') {
          applyLiveUpdate(msg)
        }
      } catch {
        // ignore parse errors
      }
    }

    ws.onerror = () => {}

    return () => {
      ws.close()
      wsRef.current = null
    }
  }, [portfolioId, applyLiveUpdate])
}
