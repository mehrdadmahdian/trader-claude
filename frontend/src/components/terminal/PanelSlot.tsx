import { useState } from 'react'
import { X, Maximize2, Minimize2 } from 'lucide-react'
import { cn } from '@/lib/utils'
import { FUNCTION_META } from '@/types/terminal'
import { getWidget } from './WidgetRegistry'
import type { PanelConfig, LinkGroup } from '@/types/terminal'

const LINK_COLORS: Record<NonNullable<LinkGroup>, string> = {
  red:    'bg-red-500',
  blue:   'bg-blue-500',
  green:  'bg-green-500',
  yellow: 'bg-yellow-500',
}

interface PanelSlotProps {
  config: PanelConfig
  isActive: boolean
  onFocus: () => void
  onClose: () => void
  onUpdate: (update: Partial<PanelConfig>) => void
}

export function PanelSlot({ config, isActive, onFocus, onClose, onUpdate: _ }: PanelSlotProps) {
  const [maximized, setMaximized] = useState(false)
  const meta = FUNCTION_META[config.functionCode]
  const Widget = getWidget(config.functionCode)

  return (
    <div
      className={cn(
        'flex flex-col h-full rounded border bg-background overflow-hidden',
        isActive ? 'border-blue-500/60' : 'border-border',
        maximized && 'fixed inset-2 z-50 shadow-2xl',
      )}
      onClick={onFocus}
    >
      {/* Header — also serves as drag handle */}
      <div className="drag-handle flex items-center gap-1.5 px-2 py-1 border-b border-border bg-muted/40 shrink-0 cursor-grab active:cursor-grabbing">
        {/* Link group indicator */}
        {config.linkGroup && (
          <span className={cn('w-2 h-2 rounded-full shrink-0', LINK_COLORS[config.linkGroup])} />
        )}

        {/* Ticker badge */}
        {config.ticker && (
          <span className="text-xs font-mono font-semibold text-foreground bg-muted px-1.5 py-0.5 rounded">
            {config.ticker}
          </span>
        )}

        {/* Function label */}
        <span className="text-xs text-muted-foreground">{meta.label}</span>

        <div className="ml-auto flex items-center gap-1">
          <button
            className="text-muted-foreground hover:text-foreground p-0.5 rounded"
            onClick={(e) => { e.stopPropagation(); setMaximized((m) => !m) }}
            title={maximized ? 'Restore' : 'Maximize'}
          >
            {maximized ? <Minimize2 size={12} /> : <Maximize2 size={12} />}
          </button>
          <button
            className="text-muted-foreground hover:text-red-500 p-0.5 rounded"
            onClick={(e) => { e.stopPropagation(); onClose() }}
            title="Close panel"
          >
            <X size={12} />
          </button>
        </div>
      </div>

      {/* Widget content */}
      <div className="flex-1 overflow-hidden">
        <Widget
          ticker={config.ticker}
          market={config.market}
          timeframe={config.timeframe}
          params={config.params}
        />
      </div>
    </div>
  )
}
