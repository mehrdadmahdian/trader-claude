import { useRef, useEffect, useState } from 'react'
import { Star, TrendingUp } from 'lucide-react'
import { cn } from '@/lib/utils'
import { useMarketStore } from '@/stores'

const DEFAULT_WATCH = [
  'BTCUSDT', 'ETHUSDT', 'SOLUSDT', 'BNBUSDT',
  'XRPUSDT', 'ADAUSDT', 'DOGEUSDT', 'AVAXUSDT',
]

function PriceRow({
  symbol,
  price,
  isActive,
  onClick,
}: {
  symbol: string
  price: number | null
  isActive: boolean
  onClick: () => void
}) {
  const [flashClass, setFlashClass] = useState('')
  const prevPriceRef = useRef<number | null>(null)

  useEffect(() => {
    if (price === null) {
      prevPriceRef.current = null
      return
    }
    if (prevPriceRef.current !== null) {
      if (price > prevPriceRef.current) {
        setFlashClass('flash-up')
      } else if (price < prevPriceRef.current) {
        setFlashClass('flash-down')
      }
    }
    prevPriceRef.current = price
    const t = setTimeout(() => setFlashClass(''), 400)
    return () => clearTimeout(t)
  }, [price])

  return (
    <button
      onClick={onClick}
      className={cn(
        'w-full flex items-center gap-2 px-3 py-2 rounded-lg text-left transition-all duration-150',
        'hover:bg-slate-50',
        isActive ? 'border-l-2 border-primary bg-primary/5' : 'border-l-2 border-transparent',
        flashClass,
      )}
    >
      <span className={cn(
        'font-mono text-xs font-semibold truncate flex-1',
        isActive ? 'text-primary' : 'text-slate-800',
      )}>
        {symbol}
      </span>
      {price !== null ? (
        <span className="font-mono text-xs text-slate-600 shrink-0">
          {price.toLocaleString(undefined, {
            minimumFractionDigits: 2,
            maximumFractionDigits: price > 100 ? 2 : 5,
          })}
        </span>
      ) : (
        <span className="text-xs text-slate-300">—</span>
      )}
    </button>
  )
}

export function WatchlistPanel({ className }: { className?: string }) {
  const ticks = useMarketStore((s) => s.ticks)
  const selectedSymbol = useMarketStore((s) => s.selectedSymbol)
  const setSelectedSymbol = useMarketStore((s) => s.setSelectedSymbol)

  const tickingSymbols = Object.keys(ticks).map((key) => key.split(':')[0])
  const displaySymbols = tickingSymbols.length > 0
    ? Array.from(new Set([...tickingSymbols, ...DEFAULT_WATCH])).slice(0, 20)
    : DEFAULT_WATCH

  return (
    <div className={cn("flex flex-col rounded-2xl bg-white shadow-sm border border-slate-100 overflow-hidden h-full", className)}>
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-slate-100 shrink-0">
        <div className="flex items-center gap-2">
          <Star className="w-3.5 h-3.5 text-slate-400" />
          <span className="text-xs font-semibold uppercase tracking-wider text-slate-400">Watchlist</span>
        </div>
        <TrendingUp className="w-3.5 h-3.5 text-slate-300" />
      </div>

      {/* Rows */}
      <div className="flex-1 overflow-y-auto p-2 space-y-0.5">
        {displaySymbols.map((sym) => {
          const tickKey = Object.keys(ticks).find((k) => k.startsWith(sym + ':'))
          const tick = tickKey ? ticks[tickKey] : null
          return (
            <PriceRow
              key={sym}
              symbol={sym}
              price={tick?.price ?? null}
              isActive={selectedSymbol === sym}
              onClick={() => setSelectedSymbol(sym)}
            />
          )
        })}
      </div>
    </div>
  )
}
