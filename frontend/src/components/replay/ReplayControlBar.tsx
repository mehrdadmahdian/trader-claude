import type { ReplayControlMsg } from '@/types'

interface ReplayControlBarProps {
  sendControl: (msg: ReplayControlMsg) => void
}

// Placeholder — fully implemented in Task 7
export function ReplayControlBar(_props: ReplayControlBarProps) {
  return (
    <div className="flex items-center justify-center px-6 py-3 border-t border-border bg-background flex-shrink-0 h-14">
      <span className="text-xs text-muted-foreground">Controls loading…</span>
    </div>
  )
}
