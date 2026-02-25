import { X } from 'lucide-react'
import { useBacktestStore } from '@/stores'
import { useReplayWS } from '@/hooks/useReplayWS'
import { ReplayChart } from './ReplayChart'
import { ReplayControlBar } from './ReplayControlBar'
import { EquityMiniChart } from './EquityMiniChart'
import { SignalToast } from './SignalToast'

interface ReplayOverlayProps {
  onSaveBookmark: () => void
}

export function ReplayOverlay({ onSaveBookmark }: ReplayOverlayProps) {
  const replayOpen = useBacktestStore((s) => s.replayOpen)
  const replayId = useBacktestStore((s) => s.replayId)
  const activeBacktest = useBacktestStore((s) => s.activeBacktest)
  const resetReplay = useBacktestStore((s) => s.resetReplay)

  const { sendControl } = useReplayWS(replayOpen ? replayId : null)

  if (!replayOpen) return null

  return (
    <div className="fixed inset-0 z-50 flex flex-col bg-background">
      {/* Header */}
      <div className="flex items-center justify-between px-6 py-3 border-b border-border flex-shrink-0">
        <div className="flex items-center gap-4">
          <h2 className="text-base font-semibold">
            Replay — {activeBacktest?.strategy_name ?? '—'}
          </h2>
          <span className="text-sm text-muted-foreground">
            {activeBacktest?.symbol} · {activeBacktest?.timeframe}
          </span>
        </div>

        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={onSaveBookmark}
            className="px-3 py-1.5 text-sm rounded-md border border-border hover:bg-accent transition-colors"
          >
            Save Bookmark
          </button>
          <button
            type="button"
            onClick={resetReplay}
            className="p-2 rounded-md hover:bg-accent transition-colors"
            aria-label="Close replay"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
      </div>

      {/* Chart area */}
      <div className="relative flex-1 overflow-hidden">
        <ReplayChart />
        <EquityMiniChart />
        <SignalToast />
      </div>

      {/* Control bar */}
      <ReplayControlBar sendControl={sendControl} />
    </div>
  )
}
