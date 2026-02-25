import { useEffect, useRef, useState } from 'react'
import { TrendingUp, TrendingDown, X } from 'lucide-react'
import { useBacktestStore } from '@/stores'
import type { Trade } from '@/types'

interface Toast {
  id: string
  trade: Trade
  eventType: 'open' | 'close'
  addedAt: number
}

const TOAST_DURATION_MS = 8000

export function SignalToast() {
  const [toasts, setToasts] = useState<Toast[]>([])
  const replayTrades = useBacktestStore((s) => s.replayTrades)
  const prevTradeIds = useRef<Set<number>>(new Set())

  // Detect newly added trades and create toasts
  useEffect(() => {
    const newToasts: Toast[] = []
    for (const trade of replayTrades) {
      if (!prevTradeIds.current.has(trade.id)) {
        prevTradeIds.current.add(trade.id)
        newToasts.push({
          id: `${trade.id}-open-${Date.now()}`,
          trade,
          eventType: 'open',
          addedAt: Date.now(),
        })
      }
    }
    if (newToasts.length > 0) {
      setToasts((prev) => [...prev, ...newToasts].slice(-5))
    }
  }, [replayTrades])

  // Auto-dismiss after TOAST_DURATION_MS
  useEffect(() => {
    if (toasts.length === 0) return
    const timer = setTimeout(() => {
      const now = Date.now()
      setToasts((prev) => prev.filter((t) => now - t.addedAt < TOAST_DURATION_MS))
    }, 500)
    return () => clearTimeout(timer)
  }, [toasts])

  function dismiss(id: string) {
    setToasts((prev) => prev.filter((t) => t.id !== id))
  }

  if (toasts.length === 0) return null

  return (
    <div className="absolute top-4 right-4 flex flex-col gap-2 z-10 pointer-events-auto">
      {toasts.map((toast) => {
        const isLong = toast.trade.direction === 'long'
        return (
          <div
            key={toast.id}
            className={`flex items-center gap-3 px-3 py-2 rounded-lg border shadow-lg bg-background/95 backdrop-blur-sm min-w-[180px] ${
              isLong ? 'border-green-500/40' : 'border-red-500/40'
            }`}
          >
            <span
              className={`flex items-center gap-1 text-sm font-medium ${
                isLong ? 'text-green-500' : 'text-red-500'
              }`}
            >
              {isLong ? <TrendingUp className="h-4 w-4" /> : <TrendingDown className="h-4 w-4" />}
              {isLong ? 'BUY' : 'SELL'}
            </span>
            <div className="flex-1 min-w-0">
              <p className="text-xs font-medium truncate">{toast.trade.symbol}</p>
              <p className="text-xs text-muted-foreground tabular-nums">
                @ {toast.trade.entry_price.toFixed(4)}
              </p>
            </div>
            <button
              type="button"
              onClick={() => dismiss(toast.id)}
              className="text-muted-foreground hover:text-foreground transition-colors"
            >
              <X className="h-3 w-3" />
            </button>
          </div>
        )
      })}
    </div>
  )
}
