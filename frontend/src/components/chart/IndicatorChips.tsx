import { X, Settings2 } from 'lucide-react'
import type { ActiveIndicator } from '../../types'

interface Props {
  indicators: ActiveIndicator[]
  onRemove: (id: string) => void
  onEdit: (indicator: ActiveIndicator) => void
}

export function IndicatorChips({ indicators, onRemove, onEdit }: Props) {
  if (indicators.length === 0) return null
  return (
    <div className="flex flex-wrap items-center gap-1">
      {indicators.map((ind) => (
        <div
          key={ind.meta.id}
          className="flex items-center gap-1 bg-secondary text-secondary-foreground rounded px-2 py-0.5 text-xs border"
        >
          <span
            className="w-2 h-2 rounded-full shrink-0"
            style={{ background: ind.meta.outputs[0]?.color ?? '#888' }}
          />
          <span className="font-medium">{ind.meta.name}</span>
          <button
            onClick={() => onEdit(ind)}
            className="ml-0.5 text-muted-foreground hover:text-foreground transition-colors"
            title="Edit parameters"
          >
            <Settings2 className="w-3 h-3" />
          </button>
          <button
            onClick={() => onRemove(ind.meta.id)}
            className="text-muted-foreground hover:text-destructive transition-colors"
            title="Remove indicator"
          >
            <X className="w-3 h-3" />
          </button>
        </div>
      ))}
    </div>
  )
}
