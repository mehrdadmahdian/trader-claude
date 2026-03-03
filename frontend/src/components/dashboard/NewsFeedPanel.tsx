import { Newspaper } from 'lucide-react'
import { cn } from '@/lib/utils'
import { formatDistanceToNow } from 'date-fns'
import { useNewsBySymbol } from '@/hooks/useNews'
import { useMarketStore } from '@/stores'
import type { NewsItem } from '@/types'

// ── Sentiment chip ──────────────────────────────────────────────────────────

function SentimentChip({ value }: { value: number }) {
  if (value === 0) return null

  const isPositive = value > 0
  const isNegative = value < 0

  const label = isPositive ? 'Bullish' : 'Bearish'
  const classes = isPositive
    ? 'bg-emerald-50 text-emerald-600 border border-emerald-200'
    : isNegative
    ? 'bg-rose-50 text-rose-600 border border-rose-200'
    : 'bg-slate-100 text-slate-500 border border-slate-200'

  return (
    <span
      className={`inline-flex items-center rounded-full px-1.5 py-0.5 text-[9px] font-semibold leading-none ${classes}`}
    >
      {label}
    </span>
  )
}

// ── News item row ───────────────────────────────────────────────────────────

function NewsRow({ item }: { item: NewsItem }) {
  const timeAgo = (() => {
    try {
      return formatDistanceToNow(new Date(item.published_at), { addSuffix: true })
    } catch {
      return ''
    }
  })()

  const content = (
    <div className="flex flex-col gap-0.5 px-3 py-2.5 hover:bg-slate-50 hover:translate-x-0.5 transition-all duration-150 cursor-pointer">
      {/* Source + time */}
      <div className="flex items-center justify-between gap-2">
        <span className="text-[10px] text-slate-400 font-medium truncate">
          {item.source}
        </span>
        <span className="text-[10px] text-slate-400 shrink-0">{timeAgo}</span>
      </div>

      {/* Headline */}
      <p className="text-xs font-medium text-slate-700 leading-snug line-clamp-2">
        {item.title}
      </p>

      {/* Sentiment chip */}
      {item.sentiment !== undefined && item.sentiment !== 0 && (
        <div className="mt-0.5">
          <SentimentChip value={item.sentiment} />
        </div>
      )}
    </div>
  )

  if (item.url) {
    return (
      <a
        href={item.url}
        target="_blank"
        rel="noopener noreferrer"
        className="block"
      >
        {content}
      </a>
    )
  }

  return <div>{content}</div>
}

// ── Empty state ─────────────────────────────────────────────────────────────

function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center gap-2 py-10 text-slate-400">
      <Newspaper className="h-7 w-7 opacity-40" />
      <p className="text-xs">No news available</p>
    </div>
  )
}

// ── Main panel ──────────────────────────────────────────────────────────────

export function NewsFeedPanel({ className }: { className?: string }) {
  const selectedSymbol = useMarketStore((s) => s.selectedSymbol)
  const { data, isFetching } = useNewsBySymbol(selectedSymbol, 20)

  const items: NewsItem[] = data?.data ?? []

  return (
    <div className={cn("flex flex-col h-full bg-white rounded-xl border border-slate-200 shadow-sm overflow-hidden", className)}>
      {/* Header */}
      <div className="flex items-center justify-between px-3 py-2.5 border-b border-slate-100 shrink-0">
        <div className="flex items-center gap-1.5">
          <Newspaper className="h-3.5 w-3.5 text-slate-400" />
          <span className="text-[11px] font-semibold text-slate-500 tracking-wider uppercase">
            News
          </span>
          {selectedSymbol && (
            <span className="inline-flex items-center rounded-full bg-indigo-50 px-2 py-0.5 text-[10px] font-semibold text-indigo-600 border border-indigo-100">
              {selectedSymbol}
            </span>
          )}
        </div>

        {/* Live fetch indicator */}
        {isFetching && (
          <span className="relative flex h-2 w-2">
            <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-indigo-400 opacity-75" />
            <span className="relative inline-flex rounded-full h-2 w-2 bg-indigo-500" />
          </span>
        )}
      </div>

      {/* Scrollable news list */}
      <div className="flex-1 overflow-y-auto min-h-0">
        {items.length === 0 && !isFetching ? (
          <EmptyState />
        ) : (
          <ul className="divide-y divide-slate-100">
            {items.map((item) => (
              <li key={item.id}>
                <NewsRow item={item} />
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}
