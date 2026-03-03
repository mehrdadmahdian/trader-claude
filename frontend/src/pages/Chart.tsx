import { useState, useMemo, useCallback, useEffect } from 'react'
import { RefreshCw, ChevronDown, Search, BarChart2, Newspaper } from 'lucide-react'
import { subDays, formatISO } from 'date-fns'
import { useMutation } from '@tanstack/react-query'
import { useMarkets, useSymbols, useCandles, useTimeframes } from '@/hooks/useMarketData'
import { CandlestickChart } from '@/components/chart/CandlestickChart'
import { IndicatorModal } from '@/components/chart/IndicatorModal'
import { IndicatorChips } from '@/components/chart/IndicatorChips'
import { PanelChart } from '@/components/chart/PanelChart'
import { NewsSidePanel } from '@/components/news/NewsSidePanel'
import { calculateIndicator } from '@/api/indicators'
import { useMarketStore, useThemeStore } from '@/stores'
import { useNewsBySymbol } from '@/hooks/useNews'
import type { ActiveIndicator, MarketSymbol, OHLCVCandle } from '@/types'

const TIMEFRAMES = ['1m', '5m', '15m', '30m', '1h', '4h', '1d', '1w']

function defaultDateRange(timeframe: string) {
  const to = new Date()
  const daysBack: Record<string, number> = {
    '1m': 1, '5m': 3, '15m': 7, '30m': 14,
    '1h': 30, '4h': 60, '1d': 365, '1w': 730,
  }
  const days = daysBack[timeframe] ?? 30
  return {
    from: formatISO(subDays(to, days)),
    to: formatISO(to),
  }
}

function storageKey(symbol: string | null, timeframe: string) {
  return `indicators:${symbol ?? ''}:${timeframe}`
}

function loadStoredIndicators(symbol: string | null, timeframe: string): ActiveIndicator[] {
  try {
    const stored = localStorage.getItem(storageKey(symbol, timeframe))
    return stored ? (JSON.parse(stored) as ActiveIndicator[]) : []
  } catch {
    return []
  }
}

export function Chart() {
  const [searchQuery, setSearchQuery] = useState('')
  const [showSearch, setShowSearch] = useState(false)

  const selectedSymbol = useMarketStore((s) => s.selectedSymbol)
  const selectedMarket = useMarketStore((s) => s.selectedMarket)
  const selectedTimeframe = useMarketStore((s) => s.selectedTimeframe)
  const setSelectedSymbol = useMarketStore((s) => s.setSelectedSymbol)
  const setSelectedMarket = useMarketStore((s) => s.setSelectedMarket)
  const setSelectedTimeframe = useMarketStore((s) => s.setSelectedTimeframe)

  const theme = useThemeStore((s) => s.theme)
  const isDark = theme === 'dark'

  const [selectedAdapter, setSelectedAdapter] = useState('binance')
  const [newsOpen, setNewsOpen] = useState(false)

  const { data: adapters } = useMarkets()
  const { data: symbols } = useSymbols(selectedAdapter, selectedMarket)
  const { data: serverTimeframes } = useTimeframes()

  const timeframes = serverTimeframes ?? TIMEFRAMES

  const { from, to } = useMemo(
    () => defaultDateRange(selectedTimeframe),
    [selectedTimeframe],
  )

  const {
    data: candles,
    isFetching,
    isError,
    error,
    refetch,
  } = useCandles({
    adapter: selectedAdapter,
    symbol: selectedSymbol ?? '',
    timeframe: selectedTimeframe,
    from,
    to,
    market: selectedMarket,
  })

  const { data: newsData, isFetching: newsFetching } = useNewsBySymbol(
    newsOpen ? selectedSymbol : null,
    20,
  )
  const newsItems = newsData?.data ?? []

  // ── Indicators state ───────────────────────────────────────────────────────

  const [indicatorModalOpen, setIndicatorModalOpen] = useState(false)
  const [activeIndicators, setActiveIndicators] = useState<ActiveIndicator[]>(() =>
    loadStoredIndicators(selectedSymbol, selectedTimeframe),
  )

  // Persist {meta, params} subset to localStorage on change
  useEffect(() => {
    const persisted = activeIndicators.map(({ meta, params }) => ({ meta, params, result: undefined }))
    localStorage.setItem(storageKey(selectedSymbol, selectedTimeframe), JSON.stringify(persisted))
  }, [activeIndicators, selectedSymbol, selectedTimeframe])

  // Reload stored indicators when symbol or timeframe changes
  useEffect(() => {
    setActiveIndicators(loadStoredIndicators(selectedSymbol, selectedTimeframe))
  }, [selectedSymbol, selectedTimeframe])

  const { mutateAsync: calcIndicator } = useMutation({ mutationFn: calculateIndicator })

  // Build the candle payload once so multiple effects can share it
  const candlePayload = useMemo<OHLCVCandle[]>(
    () => candles ?? [],
    [candles],
  )

  function toCandleRequest(cs: OHLCVCandle[]) {
    return cs.map((c) => ({
      timestamp: c.timestamp,
      open: c.open,
      high: c.high,
      low: c.low,
      close: c.close,
      volume: c.volume,
    }))
  }

  // Re-calculate all active indicators when candles change
  useEffect(() => {
    if (candlePayload.length === 0 || activeIndicators.length === 0) return
    const payload = toCandleRequest(candlePayload)
    activeIndicators.forEach((ind, idx) => {
      calcIndicator({ indicator_id: ind.meta.id, params: ind.params, candles: payload })
        .then((result) => {
          setActiveIndicators((prev) =>
            prev.map((a, i) => (i === idx ? { ...a, result } : a)),
          )
        })
        .catch(() => { /* chart still renders without indicator */ })
    })
  }, [candlePayload]) // eslint-disable-line react-hooks/exhaustive-deps

  const handleAddIndicator = useCallback(
    async (ind: ActiveIndicator) => {
      if (candlePayload.length === 0) {
        setActiveIndicators((prev) => [...prev, ind])
        return
      }
      try {
        const result = await calcIndicator({
          indicator_id: ind.meta.id,
          params: ind.params,
          candles: toCandleRequest(candlePayload),
        })
        setActiveIndicators((prev) => [...prev, { ...ind, result }])
      } catch {
        setActiveIndicators((prev) => [...prev, ind])
      }
    },
    [candlePayload, calcIndicator],
  )

  const handleRemoveIndicator = useCallback((id: string) => {
    setActiveIndicators((prev) => prev.filter((a) => a.meta.id !== id))
  }, [])

  // ── Symbol search ──────────────────────────────────────────────────────────

  const filteredSymbols = useMemo(() => {
    if (!symbols) return []
    if (!searchQuery.trim()) return symbols.slice(0, 50)
    const q = searchQuery.toLowerCase()
    return symbols.filter(
      (s) => s.id.toLowerCase().includes(q) || s.description?.toLowerCase().includes(q),
    )
  }, [symbols, searchQuery])

  const handleAdapterChange = useCallback(
    (e: React.ChangeEvent<HTMLSelectElement>) => {
      const id = e.target.value
      setSelectedAdapter(id)
      const adapter = adapters?.find((a) => a.id === id)
      if (adapter && adapter.markets.length > 0) {
        setSelectedMarket(adapter.markets[0])
      }
      setSelectedSymbol(null)
    },
    [adapters, setSelectedMarket, setSelectedSymbol],
  )

  const handleSymbolSelect = useCallback(
    (sym: MarketSymbol) => {
      setSelectedSymbol(sym.id)
      setShowSearch(false)
      setSearchQuery('')
    },
    [setSelectedSymbol],
  )

  // ── Render ────────────────────────────────────────────────────────────────

  const overlayIndicators = activeIndicators.filter((ind) => ind.meta.type === 'overlay')
  const panelIndicators = activeIndicators.filter((ind) => ind.meta.type === 'panel')

  return (
    <div className="flex flex-col h-full gap-3 px-4 py-3">
      {/* ── Toolbar ── */}
      <div className="flex flex-wrap items-center gap-2">
        {/* Adapter selector */}
        <div className="relative">
          <select
            className="appearance-none h-8 rounded-lg border border-slate-200 bg-white pl-3 pr-7 text-sm shadow-sm text-slate-700 hover:border-slate-300 focus:outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary transition cursor-pointer"
            value={selectedAdapter}
            onChange={handleAdapterChange}
            aria-label="Select adapter"
          >
            {(adapters ?? [{ id: 'binance', markets: ['crypto'] }]).map((a) => (
              <option key={a.id} value={a.id}>
                {a.id.charAt(0).toUpperCase() + a.id.slice(1)}
              </option>
            ))}
          </select>
          <ChevronDown className="pointer-events-none absolute right-2 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
        </div>

        {/* Symbol search */}
        <div className="relative">
          <button
            className="flex items-center gap-1.5 h-8 bg-white border border-slate-200 rounded-lg px-3 text-sm min-w-[140px] hover:border-slate-300 hover:shadow-md transition-all duration-150 shadow-sm"
            onClick={() => setShowSearch((v) => !v)}
            aria-expanded={showSearch}
            aria-haspopup="listbox"
          >
            <Search className="h-4 w-4 text-muted-foreground flex-shrink-0" />
            <span className={selectedSymbol ? 'font-medium' : 'text-muted-foreground'}>
              {selectedSymbol ?? 'Search symbol…'}
            </span>
            <ChevronDown className="h-4 w-4 text-muted-foreground ml-auto" />
          </button>

          {showSearch && (
            <div className="absolute z-50 top-full mt-1 w-72 bg-white border border-slate-200 rounded-xl shadow-xl">
              <div className="p-2 border-b border-border">
                <input
                  autoFocus
                  type="text"
                  placeholder="Search…"
                  className="w-full rounded-lg border border-slate-200 px-3 py-1.5 text-sm outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary transition"
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  aria-label="Symbol search"
                />
              </div>
              <ul
                role="listbox"
                className="max-h-64 overflow-y-auto divide-y divide-border"
              >
                {filteredSymbols.length === 0 ? (
                  <li className="px-3 py-4 text-sm text-muted-foreground text-center">
                    No symbols found
                  </li>
                ) : (
                  filteredSymbols.map((sym) => (
                    <li key={sym.id}>
                      <button
                        role="option"
                        aria-selected={sym.id === selectedSymbol}
                        className="w-full px-3 py-2.5 text-left text-sm hover:bg-slate-50 transition-colors flex items-center justify-between"
                        onClick={() => handleSymbolSelect(sym)}
                      >
                        <span className="font-medium">{sym.id}</span>
                        {sym.description && (
                          <span className="text-muted-foreground text-xs ml-2 truncate max-w-[100px]">
                            {sym.description}
                          </span>
                        )}
                      </button>
                    </li>
                  ))
                )}
              </ul>
            </div>
          )}
        </div>

        {/* Timeframe buttons */}
        <div className="flex items-center gap-0.5 bg-slate-100 rounded-lg p-0.5">
          {timeframes.map((tf) => (
            <button
              key={tf}
              onClick={() => setSelectedTimeframe(tf)}
              className={`px-2.5 py-1 text-xs font-medium rounded transition-colors ${
                selectedTimeframe === tf
                  ? 'bg-white text-slate-900 shadow-sm'
                  : 'text-slate-500 hover:text-slate-700'
              }`}
            >
              {tf}
            </button>
          ))}
        </div>

        {/* Indicators button */}
        <button
          onClick={() => setIndicatorModalOpen(true)}
          className="flex items-center gap-1.5 h-8 px-3 text-sm bg-white border border-slate-200 rounded-lg shadow-sm hover:border-slate-300 hover:shadow-md transition-all duration-150 text-slate-700"
          aria-label="Open indicators"
        >
          <BarChart2 className="h-4 w-4" />
          Indicators
        </button>

        {/* News toggle button */}
        <button
          onClick={() => setNewsOpen((v) => !v)}
          className={`flex items-center gap-1.5 px-3 py-2 text-sm border rounded-md transition-colors ${
            newsOpen
              ? 'bg-primary text-white border-primary'
              : 'bg-white border-slate-200 text-slate-700 hover:border-slate-300 hover:shadow-md'
          }`}
          aria-label="Toggle news panel"
          aria-pressed={newsOpen}
        >
          <Newspaper className="h-4 w-4" />
          News
        </button>

        {/* Active indicator chips */}
        {activeIndicators.length > 0 && (
          <IndicatorChips
            indicators={activeIndicators}
            onRemove={handleRemoveIndicator}
            onEdit={() => setIndicatorModalOpen(true)}
          />
        )}

        {/* Refresh button */}
        <button
          onClick={() => refetch()}
          disabled={isFetching}
          className="ml-auto flex items-center gap-1.5 h-8 px-3 text-sm bg-white border border-slate-200 rounded-lg shadow-sm hover:border-slate-300 hover:shadow-md transition-all duration-150 disabled:opacity-50 text-slate-700"
          aria-label="Refresh data"
        >
          <RefreshCw className={`h-4 w-4 ${isFetching ? 'animate-spin' : ''}`} />
          Refresh
        </button>
      </div>

      {/* ── Chart area + News panel row ── */}
      <div className="flex flex-1 gap-0 min-h-0 overflow-hidden">
        {/* Chart column */}
        <div className="flex flex-col flex-1 gap-4 min-h-0 min-w-0 overflow-hidden">
          <div className="flex-1 bg-white border border-slate-100 rounded-2xl shadow-sm overflow-hidden min-h-0">
            {!selectedSymbol ? (
              /* Empty state */
              <div className="flex flex-col items-center justify-center h-full gap-3 text-muted-foreground">
                <Search className="h-12 w-12 opacity-30" />
                <p className="text-lg font-medium">Select a symbol to view chart</p>
                <p className="text-sm">Choose an adapter and search for a symbol above</p>
              </div>
            ) : isError ? (
              /* Error state */
              <div className="flex flex-col items-center justify-center h-full gap-3">
                <p className="text-destructive font-medium">Failed to load candles</p>
                <p className="text-sm text-muted-foreground">
                  {error instanceof Error ? error.message : 'Unknown error'}
                </p>
                <button
                  onClick={() => refetch()}
                  className="mt-2 px-4 py-2 bg-primary text-primary-foreground rounded-md text-sm hover:bg-primary/90 transition-colors"
                >
                  Retry
                </button>
              </div>
            ) : (
              /* Chart with loading overlay */
              <CandlestickChart
                candles={candles ?? []}
                overlayIndicators={overlayIndicators}
                isLoading={isFetching}
                className="h-full"
              />
            )}
          </div>

          {/* ── Panel indicators ── */}
          {panelIndicators.map((ind) => (
            <PanelChart
              key={ind.meta.id}
              indicator={ind}
              onClose={() => handleRemoveIndicator(ind.meta.id)}
              isDark={isDark}
            />
          ))}
        </div>

        {/* News side panel (conditional) */}
        {newsOpen && (
          <NewsSidePanel
            items={newsItems}
            isLoading={newsFetching}
            onClose={() => setNewsOpen(false)}
          />
        )}
      </div>
      {/* ── Indicator modal ── */}
      <IndicatorModal
        open={indicatorModalOpen}
        onClose={() => setIndicatorModalOpen(false)}
        active={activeIndicators}
        onAdd={handleAddIndicator}
      />
    </div>
  )
}
