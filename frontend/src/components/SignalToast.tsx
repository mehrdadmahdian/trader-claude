import { useEffect } from 'react'
import { X, TrendingUp, TrendingDown } from 'lucide-react'
import { useMonitorStore } from '@/stores'
import type { MonitorSignal } from '@/types'

const TOAST_DURATION_MS = 8000

interface SignalToastItemProps {
  signal: MonitorSignal
}

function SignalToastItem({ signal }: SignalToastItemProps) {
  const clearSignal = useMonitorStore((s) => s.clearSignal)

  useEffect(() => {
    const t = setTimeout(() => clearSignal(signal.id), TOAST_DURATION_MS)
    return () => clearTimeout(t)
  }, [signal.id, clearSignal])

  const isLong = signal.direction === 'long'
  const bg = isLong
    ? 'bg-green-900/90 border-green-600'
    : 'bg-red-900/90 border-red-600'
  const text = isLong ? 'text-green-100' : 'text-red-100'
  const Icon = isLong ? TrendingUp : TrendingDown
  const label = isLong ? 'LONG' : 'SHORT'

  return (
    <div
      className={`flex items-start gap-3 p-4 rounded-lg border shadow-lg w-80 animate-slide-in-right ${bg}`}
    >
      <Icon className={`mt-0.5 h-5 w-5 flex-shrink-0 ${text}`} />
      <div className="flex-1 min-w-0">
        <p className={`text-sm font-semibold ${text}`}>
          {label} Signal
        </p>
        <p className={`text-xs mt-0.5 ${text} opacity-80 truncate`}>
          ${signal.price.toLocaleString(undefined, { maximumFractionDigits: 2 })}
          {' · '}strength {(signal.strength * 100).toFixed(0)}%
        </p>
      </div>
      <button
        onClick={() => clearSignal(signal.id)}
        className={`flex-shrink-0 ${text} opacity-70 hover:opacity-100`}
      >
        <X className="h-4 w-4" />
      </button>
    </div>
  )
}

// SignalToast renders stacked toasts (max 3) in the bottom-right corner.
// Include this once in Layout.tsx.
export function SignalToast() {
  const pendingSignals = useMonitorStore((s) => s.pendingSignals)

  if (pendingSignals.length === 0) return null

  // Show at most 3 toasts
  const visible = pendingSignals.slice(-3)

  return (
    <div className="fixed bottom-6 right-6 z-50 flex flex-col gap-2 items-end">
      {visible.map((sig) => (
        <SignalToastItem key={sig.id} signal={sig} />
      ))}
    </div>
  )
}
