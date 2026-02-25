import { useRef, useState } from 'react'
import { X, Loader2 } from 'lucide-react'
import { useBacktestStore } from '@/stores'
import { useCreateBookmark } from '@/hooks/useReplay'

interface BookmarkModalProps {
  open: boolean
  onClose: () => void
}

export function BookmarkModal({ open, onClose }: BookmarkModalProps) {
  const [label, setLabel] = useState('')
  const [note, setNote] = useState('')
  const chartCanvasRef = useRef<HTMLCanvasElement | null>(null)

  const activeBacktest = useBacktestStore((s) => s.activeBacktest)
  const replayIndex = useBacktestStore((s) => s.replayIndex)

  const createBookmark = useCreateBookmark(activeBacktest?.id ?? null)

  if (!open) return null

  function captureSnapshot(): string {
    // Find the lightweight-charts canvas in the replay overlay
    const canvas = document.querySelector<HTMLCanvasElement>(
      '.fixed.inset-0.z-50 canvas',
    )
    if (!canvas) return ''
    try {
      return canvas.toDataURL('image/png')
    } catch {
      return ''
    }
  }

  async function handleSave() {
    if (!activeBacktest) return
    const chartSnapshot = captureSnapshot()
    await createBookmark.mutateAsync({
      backtest_run_id: activeBacktest.id,
      candle_index: replayIndex,
      label: label.trim(),
      note: note.trim(),
      chart_snapshot: chartSnapshot,
    })
    setLabel('')
    setNote('')
    onClose()
  }

  const inputClass =
    'w-full bg-background border border-border rounded px-3 py-1.5 text-sm outline-none focus:ring-2 focus:ring-primary'

  return (
    <div className="fixed inset-0 z-[60] flex items-center justify-center bg-black/50 backdrop-blur-sm">
      <div className="bg-card border border-border rounded-xl shadow-xl w-full max-w-md mx-4 flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-border">
          <h3 className="text-sm font-semibold">Save Bookmark</h3>
          <button
            type="button"
            onClick={onClose}
            className="p-1.5 rounded-md hover:bg-accent transition-colors"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        {/* Body */}
        <div className="px-5 py-4 flex flex-col gap-3">
          <div>
            <label className="text-xs text-muted-foreground uppercase tracking-wide mb-1 block">
              Label
            </label>
            <input
              type="text"
              className={inputClass}
              placeholder="e.g. Double-top before reversal"
              value={label}
              onChange={(e) => setLabel(e.target.value)}
              autoFocus
            />
          </div>
          <div>
            <label className="text-xs text-muted-foreground uppercase tracking-wide mb-1 block">
              Note
            </label>
            <textarea
              className={`${inputClass} resize-none h-24`}
              placeholder="Observations, context, ideas…"
              value={note}
              onChange={(e) => setNote(e.target.value)}
            />
          </div>
          <p className="text-xs text-muted-foreground">
            Candle index: <span className="tabular-nums font-medium">{replayIndex}</span>
            {' · '}
            Chart snapshot will be captured automatically.
          </p>
          <canvas ref={chartCanvasRef} className="hidden" />
        </div>

        {/* Footer */}
        <div className="flex justify-end gap-2 px-5 py-4 border-t border-border">
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-1.5 text-sm rounded-md border border-border hover:bg-accent transition-colors"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={handleSave}
            disabled={createBookmark.isPending || !label.trim()}
            className="px-4 py-1.5 text-sm rounded-md bg-primary text-primary-foreground hover:bg-primary/90 transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
          >
            {createBookmark.isPending && <Loader2 className="h-3 w-3 animate-spin" />}
            Save
          </button>
        </div>
      </div>
    </div>
  )
}
