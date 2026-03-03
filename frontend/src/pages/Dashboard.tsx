import { useMemo } from 'react'
import { CandlestickChart as ChartIcon } from 'lucide-react'
import { CandlestickChart } from '@/components/chart/CandlestickChart'
import { WatchlistPanel } from '@/components/dashboard/WatchlistPanel'
import { PortfolioSummaryPanel } from '@/components/dashboard/PortfolioSummaryPanel'
import { NewsFeedPanel } from '@/components/dashboard/NewsFeedPanel'
import { AlertsFeedPanel } from '@/components/dashboard/AlertsFeedPanel'
import { useCandles } from '@/hooks/useMarketData'
import { useMarketStore } from '@/stores'

// Rolling 7-day window for candle fetches
function useCandleWindow() {
  return useMemo(() => {
    const to = new Date()
    const from = new Date(to.getTime() - 7 * 24 * 60 * 60 * 1000)
    return {
      from: from.toISOString(),
      to: to.toISOString(),
    }
  }, [])
}

export function Dashboard() {
  const selectedSymbol = useMarketStore((s) => s.selectedSymbol)
  const selectedMarket = useMarketStore((s) => s.selectedMarket)
  const selectedTimeframe = useMarketStore((s) => s.selectedTimeframe)

  const { from, to } = useCandleWindow()

  // useCandles has an internal `enabled` guard: Boolean(adapter && symbol && timeframe && from && to)
  // Passing empty string for symbol when none is selected prevents the query from firing.
  const { data: candles = [], isFetching } = useCandles({
    adapter: 'binance',
    symbol: selectedSymbol ?? '',
    timeframe: selectedTimeframe,
    from,
    to,
    market: selectedMarket,
  })

  return (
    <div className="flex h-full gap-3 p-3 overflow-hidden">
      {/* ── Left rail ──────────────────────────────────────────── */}
      <div className="w-[220px] shrink-0 flex flex-col gap-3 overflow-hidden">
        <WatchlistPanel className="flex-1" />
        <PortfolioSummaryPanel className="shrink-0" />
      </div>

      {/* ── Center: chart ──────────────────────────────────────── */}
      <div className="flex-1 min-w-0 rounded-2xl bg-white shadow-sm border border-slate-100 h-full overflow-hidden">
        {selectedSymbol ? (
          <CandlestickChart
            candles={candles}
            isLoading={isFetching}
            className="w-full h-full"
          />
        ) : (
          <div className="flex flex-col items-center justify-center h-full gap-3 text-slate-300">
            <ChartIcon className="w-12 h-12" />
            <p className="text-sm font-medium text-slate-500">Select a symbol to view chart</p>
            <p className="text-xs text-slate-400">Use the symbol picker in the top bar</p>
          </div>
        )}
      </div>

      {/* ── Right rail ─────────────────────────────────────────── */}
      <div className="w-[280px] shrink-0 flex flex-col gap-3 overflow-hidden">
        <NewsFeedPanel className="flex-1" />
        <AlertsFeedPanel className="shrink-0 max-h-[280px]" />
      </div>
    </div>
  )
}
