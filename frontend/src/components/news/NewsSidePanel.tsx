import { X, ExternalLink } from 'lucide-react'
import { formatDistanceToNow } from 'date-fns'
import { cn } from '@/lib/utils'
import type { NewsItem } from '@/types'

interface SentimentDotProps {
  sentiment: number
}

function SentimentDot({ sentiment }: SentimentDotProps) {
  const color =
    sentiment > 0.2
      ? 'bg-green-500'
      : sentiment < -0.2
        ? 'bg-red-500'
        : 'bg-yellow-400'
  const label =
    sentiment > 0.2 ? 'Positive' : sentiment < -0.2 ? 'Negative' : 'Neutral'
  return (
    <span
      className={cn('mt-1 inline-block w-2 h-2 rounded-full shrink-0', color)}
      title={label}
      aria-label={label}
    />
  )
}

interface NewsSidePanelProps {
  items: NewsItem[]
  isLoading: boolean
  onClose: () => void
}

export function NewsSidePanel({ items, isLoading, onClose }: NewsSidePanelProps) {
  return (
    <div className="w-80 flex flex-col border-l border-border bg-card shrink-0 h-full overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-border shrink-0">
        <h3 className="font-semibold text-sm">News</h3>
        <button
          onClick={onClose}
          className="p-1 rounded text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
          aria-label="Close news panel"
        >
          <X className="w-4 h-4" />
        </button>
      </div>

      {/* Feed */}
      <div className="flex-1 overflow-y-auto">
        {isLoading && (
          <p className="p-4 text-sm text-muted-foreground">Loading news…</p>
        )}
        {!isLoading && items.length === 0 && (
          <p className="p-4 text-sm text-muted-foreground">No news found for this symbol.</p>
        )}
        {items.map((item) => (
          <a
            key={item.id}
            href={item.url}
            target="_blank"
            rel="noopener noreferrer"
            className="flex gap-2 px-4 py-3 border-b border-border hover:bg-accent/50 transition-colors group"
          >
            <SentimentDot sentiment={item.sentiment} />
            <div className="min-w-0 flex-1">
              <p className="text-xs font-medium leading-snug line-clamp-3 text-foreground group-hover:text-primary transition-colors">
                {item.title}
              </p>
              <div className="flex items-center gap-2 mt-1.5">
                <span className="text-[10px] font-semibold text-muted-foreground uppercase tracking-wide">
                  {item.source}
                </span>
                <span className="text-[10px] text-muted-foreground">
                  {formatDistanceToNow(new Date(item.published_at), { addSuffix: true })}
                </span>
                <ExternalLink className="w-3 h-3 text-muted-foreground ml-auto opacity-0 group-hover:opacity-100 transition-opacity shrink-0" />
              </div>
            </div>
          </a>
        ))}
      </div>
    </div>
  )
}
