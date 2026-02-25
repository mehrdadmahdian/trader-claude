import { SkipBack, SkipForward, Play, Pause, ChevronRight } from 'lucide-react'
import { useBacktestStore } from '@/stores'
import type { ReplayControlMsg } from '@/types'
import { format } from 'date-fns'

const SPEED_CHIPS = [0.25, 0.5, 1, 2, 4, 10]

interface ReplayControlBarProps {
  sendControl: (msg: ReplayControlMsg) => void
}

export function ReplayControlBar({ sendControl }: ReplayControlBarProps) {
  const replayState = useBacktestStore((s) => s.replayState)
  const replayIndex = useBacktestStore((s) => s.replayIndex)
  const replayTotal = useBacktestStore((s) => s.replayTotal)
  const replaySpeed = useBacktestStore((s) => s.replaySpeed)
  const replayCandles = useBacktestStore((s) => s.replayCandles)

  const isPlaying = replayState === 'playing'
  const isComplete = replayState === 'complete'

  const currentCandle = replayCandles[replayIndex - 1] ?? replayCandles[replayCandles.length - 1]
  const timestamp = currentCandle
    ? (() => {
        try {
          return format(new Date(currentCandle.timestamp), 'MMM d, yyyy HH:mm')
        } catch {
          return currentCandle.timestamp
        }
      })()
    : '—'

  function handlePlayPause() {
    if (isPlaying) {
      sendControl({ type: 'pause' })
    } else if (isComplete) {
      sendControl({ type: 'seek', index: 0 })
      sendControl({ type: 'start' })
    } else {
      sendControl({ type: 'start' })
    }
  }

  function handleStep() {
    sendControl({ type: 'step' })
  }

  function handleRestart() {
    sendControl({ type: 'pause' })
    sendControl({ type: 'seek', index: 0 })
  }

  function handleSeek(e: React.ChangeEvent<HTMLInputElement>) {
    const idx = parseInt(e.target.value, 10)
    sendControl({ type: 'seek', index: idx })
  }

  function handleSpeed(speed: number) {
    sendControl({ type: 'set_speed', speed })
  }

  return (
    <div className="flex flex-col gap-2 px-6 py-3 border-t border-border bg-background flex-shrink-0">
      {/* Seek slider */}
      <div className="flex items-center gap-3">
        <span className="text-xs text-muted-foreground tabular-nums w-8 text-right">
          {replayIndex}
        </span>
        <input
          type="range"
          min={0}
          max={Math.max(replayTotal, 1)}
          value={replayIndex}
          onChange={handleSeek}
          className="flex-1 h-1.5 bg-muted rounded-full appearance-none cursor-pointer accent-primary"
        />
        <span className="text-xs text-muted-foreground tabular-nums w-8">
          {replayTotal}
        </span>
      </div>

      {/* Controls row */}
      <div className="flex items-center justify-between">
        {/* Playback buttons */}
        <div className="flex items-center gap-1">
          <button
            type="button"
            onClick={handleRestart}
            className="p-2 rounded-md hover:bg-accent transition-colors text-muted-foreground hover:text-foreground"
            title="Restart"
          >
            <SkipBack className="h-4 w-4" />
          </button>

          <button
            type="button"
            onClick={handlePlayPause}
            className="p-2 rounded-md bg-primary text-primary-foreground hover:bg-primary/90 transition-colors"
            title={isPlaying ? 'Pause' : 'Play'}
          >
            {isPlaying ? <Pause className="h-4 w-4" /> : <Play className="h-4 w-4" />}
          </button>

          <button
            type="button"
            onClick={handleStep}
            disabled={isComplete}
            className="p-2 rounded-md hover:bg-accent transition-colors text-muted-foreground hover:text-foreground disabled:opacity-40"
            title="Step forward one candle"
          >
            <ChevronRight className="h-4 w-4" />
          </button>

          <button
            type="button"
            disabled
            className="p-2 rounded-md text-muted-foreground opacity-40 cursor-not-allowed"
            title="Skip to end"
          >
            <SkipForward className="h-4 w-4" />
          </button>
        </div>

        {/* Timestamp */}
        <span className="text-xs text-muted-foreground tabular-nums">{timestamp}</span>

        {/* Speed chips */}
        <div className="flex items-center gap-1">
          {SPEED_CHIPS.map((s) => (
            <button
              key={s}
              type="button"
              onClick={() => handleSpeed(s)}
              className={`px-2 py-0.5 text-xs rounded border transition-colors ${
                replaySpeed === s
                  ? 'bg-primary text-primary-foreground border-primary'
                  : 'border-border text-muted-foreground hover:text-foreground hover:border-primary/50'
              }`}
            >
              {s}×
            </button>
          ))}
        </div>
      </div>
    </div>
  )
}
