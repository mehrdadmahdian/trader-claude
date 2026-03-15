import { useEffect, useRef } from 'react'
import { usePortfolioStore } from '@/stores'
import { wsBase } from '@/lib/utils'
import type { PortfolioUpdateMsg } from '@/types'

export function usePortfolioLive(portfolioId: number | null) {
  const { applyLiveUpdate } = usePortfolioStore()
  const wsRef = useRef<WebSocket | null>(null)

  useEffect(() => {
    if (!portfolioId) return

    const ws = new WebSocket(`${wsBase()}/ws/portfolio/${portfolioId}/live`)
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
