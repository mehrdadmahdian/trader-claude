import { useMemo } from 'react'
import { CandlestickChart } from '@/components/chart/CandlestickChart'
import { useCandles } from '@/hooks/useMarketData'
import type { WidgetProps } from '@/types/terminal'

function defaultDateRange(timeframe: string): { from: string; to: string } {
  const daysBack: Record<string, number> = {
    '1m': 1, '5m': 2, '15m': 3, '30m': 5,
    '1h': 7, '4h': 14, '1d': 90, '1w': 365,
  }
  const days = daysBack[timeframe] ?? 30
  const to = new Date()
  const from = new Date(to.getTime() - days * 24 * 60 * 60 * 1000)
  return {
    from: from.toISOString(),
    to: to.toISOString(),
  }
}

export function ChartWidget({ ticker, market = 'binance', timeframe = '1h' }: WidgetProps) {
  const { from, to } = useMemo(() => defaultDateRange(timeframe), [timeframe])

  const { data: candles = [], isFetching } = useCandles({
    adapter: market,
    symbol: ticker,
    timeframe,
    from,
    to,
    market,
  })

  if (!ticker) {
    return (
      <div className="flex items-center justify-center h-full text-muted-foreground text-sm">
        Type a ticker in the command bar (e.g. BTC GP)
      </div>
    )
  }

  return (
    <CandlestickChart
      candles={candles ?? []}
      isLoading={isFetching}
      className="h-full"
    />
  )
}
