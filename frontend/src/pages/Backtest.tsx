import { useState, useCallback, useEffect, useMemo } from 'react'
import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip as RechartsTooltip,
  ResponsiveContainer,
} from 'recharts'
import { format, parseISO } from 'date-fns'
import {
  Play,
  Clock,
  CheckCircle,
  XCircle,
  Loader2,
  BarChart2,
  TrendingUp,
  TrendingDown,
  AlertCircle,
} from 'lucide-react'
import {
  useStrategies,
  useBacktestRuns,
  useBacktestRun,
  useRunBacktest,
  useBacktestProgress,
} from '@/hooks/useBacktest'
import { useMarkets, useCandles } from '@/hooks/useMarketData'
import { CandlestickChart } from '@/components/chart/CandlestickChart'
import { useBacktestStore } from '@/stores'
import type {
  BacktestMetrics,
  BacktestStatus,
  ParamDefinition,
  StrategyInfo,
  Trade,
} from '@/types'

// ── Constants ─────────────────────────────────────────────────────────────────

const TIMEFRAMES = ['1m', '5m', '15m', '30m', '1h', '4h', '1d', '1w']
const MARKETS = ['crypto', 'stock', 'etf', 'forex']

// ── Helper: status badge ──────────────────────────────────────────────────────

function StatusBadge({ status }: { status: BacktestStatus }) {
  const map: Record<BacktestStatus, { label: string; classes: string; icon: React.ReactNode }> = {
    pending: {
      label: 'Pending',
      classes: 'bg-yellow-500/10 text-yellow-500 border-yellow-500/20',
      icon: <Clock className="h-3 w-3" />,
    },
    running: {
      label: 'Running',
      classes: 'bg-blue-500/10 text-blue-500 border-blue-500/20',
      icon: <Loader2 className="h-3 w-3 animate-spin" />,
    },
    completed: {
      label: 'Completed',
      classes: 'bg-green-500/10 text-green-500 border-green-500/20',
      icon: <CheckCircle className="h-3 w-3" />,
    },
    failed: {
      label: 'Failed',
      classes: 'bg-red-500/10 text-red-500 border-red-500/20',
      icon: <XCircle className="h-3 w-3" />,
    },
    cancelled: {
      label: 'Cancelled',
      classes: 'bg-gray-500/10 text-gray-500 border-gray-500/20',
      icon: <XCircle className="h-3 w-3" />,
    },
  }
  const { label, classes, icon } = map[status] ?? map.pending
  return (
    <span
      className={`inline-flex items-center gap-1 px-2 py-0.5 rounded border text-xs font-medium ${classes}`}
    >
      {icon}
      {label}
    </span>
  )
}

// ── Helper: metric card ───────────────────────────────────────────────────────

function MetricCard({
  label,
  value,
  isPercent = false,
  colorize = false,
}: {
  label: string
  value: number | undefined
  isPercent?: boolean
  colorize?: boolean
}) {
  const formatted =
    value == null
      ? '—'
      : isPercent
        ? `${(value * 100).toFixed(2)}%`
        : value.toFixed(4)

  const colorClass =
    colorize && value != null
      ? value >= 0
        ? 'text-green-500'
        : 'text-red-500'
      : 'text-foreground'

  return (
    <div className="bg-card border border-border rounded-lg p-4 flex flex-col gap-1">
      <span className="text-xs text-muted-foreground uppercase tracking-wide">{label}</span>
      <span className={`text-xl font-semibold tabular-nums ${colorClass}`}>{formatted}</span>
    </div>
  )
}

// ── ParamField ────────────────────────────────────────────────────────────────

function ParamField({
  param,
  value,
  onChange,
}: {
  param: ParamDefinition
  value: unknown
  onChange: (v: unknown) => void
}) {
  const inputClass =
    'w-full bg-background border border-border rounded px-3 py-1.5 text-sm outline-none focus:ring-2 focus:ring-primary'

  if (param.type === 'bool') {
    return (
      <div className="flex items-center justify-between">
        <div>
          <label className="text-sm font-medium">{param.name}</label>
          {param.description && (
            <p className="text-xs text-muted-foreground">{param.description}</p>
          )}
        </div>
        <button
          type="button"
          onClick={() => onChange(!value)}
          className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-primary focus:ring-offset-2 ${
            value ? 'bg-primary' : 'bg-muted'
          }`}
          aria-checked={Boolean(value)}
          role="switch"
        >
          <span
            className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
              value ? 'translate-x-6' : 'translate-x-1'
            }`}
          />
        </button>
      </div>
    )
  }

  if (param.type === 'select' && param.options) {
    return (
      <div>
        <label className="text-sm font-medium block mb-1">{param.name}</label>
        {param.description && (
          <p className="text-xs text-muted-foreground mb-1">{param.description}</p>
        )}
        <select
          className={inputClass}
          value={String(value ?? param.default ?? '')}
          onChange={(e) => onChange(e.target.value)}
        >
          {param.options.map((opt) => (
            <option key={opt} value={opt}>
              {opt}
            </option>
          ))}
        </select>
      </div>
    )
  }

  if (param.type === 'int' || param.type === 'float') {
    return (
      <div>
        <div className="flex items-center justify-between mb-1">
          <label className="text-sm font-medium">{param.name}</label>
          <span className="text-xs text-muted-foreground tabular-nums">{String(value ?? '')}</span>
        </div>
        {param.description && (
          <p className="text-xs text-muted-foreground mb-1">{param.description}</p>
        )}
        <input
          type="number"
          className={inputClass}
          value={String(value ?? param.default ?? '')}
          min={param.min}
          max={param.max}
          step={param.type === 'float' ? 'any' : '1'}
          onChange={(e) => {
            const raw = e.target.value
            const parsed = param.type === 'float' ? parseFloat(raw) : parseInt(raw, 10)
            onChange(isNaN(parsed) ? raw : parsed)
          }}
        />
      </div>
    )
  }

  // string fallback
  return (
    <div>
      <label className="text-sm font-medium block mb-1">{param.name}</label>
      {param.description && (
        <p className="text-xs text-muted-foreground mb-1">{param.description}</p>
      )}
      <input
        type="text"
        className={inputClass}
        value={String(value ?? param.default ?? '')}
        onChange={(e) => onChange(e.target.value)}
      />
    </div>
  )
}

// ── OverviewTab ───────────────────────────────────────────────────────────────

function OverviewTab({
  metrics,
  equityCurve,
}: {
  metrics: BacktestMetrics | undefined
  equityCurve: { timestamp: string; value: number }[]
}) {
  const chartData = useMemo(
    () =>
      equityCurve.map((pt) => ({
        time: pt.timestamp,
        value: pt.value,
      })),
    [equityCurve],
  )

  const formatXAxis = useCallback((tick: string) => {
    try {
      return format(parseISO(tick), 'MMM d')
    } catch {
      return tick
    }
  }, [])

  const formatTooltipDate = useCallback((label: string) => {
    try {
      return format(parseISO(label), 'MMM d, yyyy HH:mm')
    } catch {
      return label
    }
  }, [])

  return (
    <div className="flex flex-col gap-6">
      {/* Metric cards */}
      <div className="grid grid-cols-3 gap-3">
        <MetricCard label="Total Return" value={metrics?.total_return} isPercent colorize />
        <MetricCard
          label="Annualized Return"
          value={metrics?.annualized_return}
          isPercent
          colorize
        />
        <MetricCard label="Sharpe Ratio" value={metrics?.sharpe_ratio} colorize />
        <MetricCard label="Sortino Ratio" value={metrics?.sortino_ratio} colorize />
        <MetricCard label="Max Drawdown" value={metrics?.max_drawdown} isPercent />
        <MetricCard label="Win Rate" value={metrics?.win_rate} isPercent />
        <MetricCard label="Profit Factor" value={metrics?.profit_factor} />
        <MetricCard label="Avg Win" value={metrics?.avg_win} isPercent colorize />
        <MetricCard label="Avg Loss" value={metrics?.avg_loss} isPercent colorize />
        <MetricCard label="Total Trades" value={metrics?.total_trades} />
        <MetricCard label="Largest Win" value={metrics?.largest_win} isPercent colorize />
        <MetricCard label="Largest Loss" value={metrics?.largest_loss} isPercent colorize />
      </div>

      {/* Equity curve */}
      {chartData.length > 0 && (
        <div>
          <h3 className="text-sm font-medium text-muted-foreground mb-3 uppercase tracking-wide">
            Equity Curve
          </h3>
          <div className="h-64 bg-card border border-border rounded-lg p-4">
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={chartData} margin={{ top: 4, right: 8, left: 8, bottom: 0 }}>
                <defs>
                  <linearGradient id="equityGrad" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#22c55e" stopOpacity={0.3} />
                    <stop offset="95%" stopColor="#22c55e" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" stroke="rgba(255,255,255,0.05)" />
                <XAxis
                  dataKey="time"
                  tickFormatter={formatXAxis}
                  tick={{ fontSize: 11 }}
                  tickLine={false}
                  axisLine={false}
                  interval="preserveStartEnd"
                />
                <YAxis
                  tick={{ fontSize: 11 }}
                  tickLine={false}
                  axisLine={false}
                  width={72}
                  tickFormatter={(v: number) => `$${v.toLocaleString()}`}
                />
                <RechartsTooltip
                  formatter={(v: number) => [`$${v.toLocaleString()}`, 'Portfolio']}
                  labelFormatter={formatTooltipDate}
                  contentStyle={{
                    background: 'hsl(var(--card))',
                    border: '1px solid hsl(var(--border))',
                    borderRadius: 6,
                    fontSize: 12,
                  }}
                />
                <Area
                  type="monotone"
                  dataKey="value"
                  stroke="#22c55e"
                  strokeWidth={2}
                  fill="url(#equityGrad)"
                  dot={false}
                  activeDot={{ r: 4 }}
                />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        </div>
      )}
    </div>
  )
}

// ── TradesTab ─────────────────────────────────────────────────────────────────

function TradesTab({ trades }: { trades: Trade[] }) {
  const sorted = useMemo(
    () => [...trades].sort((a, b) => a.entry_time.localeCompare(b.entry_time)),
    [trades],
  )

  const formatPrice = (v: number | undefined) => (v == null ? '—' : v.toFixed(4))
  const formatDate = (iso: string) => {
    try {
      return format(parseISO(iso), 'MMM d HH:mm')
    } catch {
      return iso
    }
  }

  if (sorted.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-16 text-muted-foreground gap-2">
        <BarChart2 className="h-10 w-10 opacity-30" />
        <p className="text-sm">(no trades)</p>
      </div>
    )
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm border-collapse">
        <thead>
          <tr className="border-b border-border text-muted-foreground text-xs uppercase tracking-wide">
            <th className="py-2 px-3 text-left">#</th>
            <th className="py-2 px-3 text-left">Dir</th>
            <th className="py-2 px-3 text-left">Entry Time</th>
            <th className="py-2 px-3 text-right">Entry Price</th>
            <th className="py-2 px-3 text-left">Exit Time</th>
            <th className="py-2 px-3 text-right">Exit Price</th>
            <th className="py-2 px-3 text-right">PnL</th>
            <th className="py-2 px-3 text-right">PnL%</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-border">
          {sorted.map((trade, i) => {
            const pnlPositive = (trade.pnl ?? 0) >= 0
            const pnlClass = pnlPositive ? 'text-green-500' : 'text-red-500'
            return (
              <tr key={trade.id} className="hover:bg-accent/30 transition-colors">
                <td className="py-2 px-3 text-muted-foreground">{i + 1}</td>
                <td className="py-2 px-3">
                  <span
                    className={`flex items-center gap-1 font-medium ${
                      trade.direction === 'long' ? 'text-green-500' : 'text-red-500'
                    }`}
                  >
                    {trade.direction === 'long' ? (
                      <TrendingUp className="h-3.5 w-3.5" />
                    ) : (
                      <TrendingDown className="h-3.5 w-3.5" />
                    )}
                    {trade.direction}
                  </span>
                </td>
                <td className="py-2 px-3 text-muted-foreground">{formatDate(trade.entry_time)}</td>
                <td className="py-2 px-3 text-right tabular-nums">
                  {formatPrice(trade.entry_price)}
                </td>
                <td className="py-2 px-3 text-muted-foreground">
                  {trade.exit_time ? formatDate(trade.exit_time) : '—'}
                </td>
                <td className="py-2 px-3 text-right tabular-nums">
                  {formatPrice(trade.exit_price)}
                </td>
                <td className={`py-2 px-3 text-right tabular-nums font-medium ${pnlClass}`}>
                  {trade.pnl == null ? '—' : trade.pnl.toFixed(4)}
                </td>
                <td className={`py-2 px-3 text-right tabular-nums ${pnlClass}`}>
                  {trade.pnl_percent == null
                    ? '—'
                    : `${(trade.pnl_percent * 100).toFixed(2)}%`}
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}

// ── ChartTab ──────────────────────────────────────────────────────────────────

function ChartTab({
  adapter,
  symbol,
  market,
  timeframe,
  startDate,
  endDate,
}: {
  adapter: string
  symbol: string
  market: string
  timeframe: string
  startDate: string
  endDate: string
}) {
  const { data: candles, isFetching } = useCandles({
    adapter,
    symbol,
    timeframe,
    from: startDate,
    to: endDate,
    market,
  })

  return (
    <div className="h-[400px]">
      <CandlestickChart candles={candles ?? []} isLoading={isFetching} className="h-full" />
    </div>
  )
}

// ── StrategyCard ──────────────────────────────────────────────────────────────

function StrategyCard({
  strategy,
  selected,
  onSelect,
}: {
  strategy: StrategyInfo
  selected: boolean
  onSelect: () => void
}) {
  return (
    <button
      type="button"
      onClick={onSelect}
      className={`text-left p-3 rounded-lg border transition-all ${
        selected
          ? 'border-primary bg-primary/10 ring-1 ring-primary'
          : 'border-border bg-card hover:border-primary/50 hover:bg-accent'
      }`}
    >
      <p className="text-sm font-semibold truncate">{strategy.name}</p>
      <p className="text-xs text-muted-foreground mt-0.5 line-clamp-2">{strategy.description}</p>
    </button>
  )
}

// ── StrategyCardSkeleton ──────────────────────────────────────────────────────

function StrategyCardSkeleton() {
  return (
    <div className="p-3 rounded-lg border border-border bg-card animate-pulse">
      <div className="h-4 w-3/4 bg-muted rounded mb-2" />
      <div className="h-3 w-full bg-muted rounded mb-1" />
      <div className="h-3 w-2/3 bg-muted rounded" />
    </div>
  )
}

// ── Main Backtest page ────────────────────────────────────────────────────────

type ResultTab = 'overview' | 'trades' | 'chart'

export function Backtest() {
  // Strategy selection
  const [selectedStrategyId, setSelectedStrategyId] = useState<string | null>(null)
  const [params, setParams] = useState<Record<string, unknown>>({})

  // Run configuration
  const [adapter, setAdapter] = useState('binance')
  const [symbol, setSymbol] = useState('')
  const [market, setMarket] = useState('crypto')
  const [timeframe, setTimeframe] = useState('1h')
  const [startDate, setStartDate] = useState('2024-01-01')
  const [endDate, setEndDate] = useState('2024-12-31')
  const [initialCash, setInitialCash] = useState(10000)
  const [commission, setCommission] = useState(0.001)
  const [runName, setRunName] = useState('')

  // Progress
  const [isRunning, setIsRunning] = useState(false)
  const [progress, setProgress] = useState(0)
  const [activeRunId, setActiveRunId] = useState<number | null>(null)

  // Results panel
  const [resultTab, setResultTab] = useState<ResultTab>('overview')

  // Zustand store
  const activeBacktest = useBacktestStore((s) => s.activeBacktest)
  const setActiveBacktest = useBacktestStore((s) => s.setActiveBacktest)
  const updateBacktest = useBacktestStore((s) => s.updateBacktest)

  // Queries / mutations
  const { data: strategies, isLoading: strategiesLoading } = useStrategies()
  const { data: adapters } = useMarkets()
  const { data: recentRuns } = useBacktestRuns()
  const runMutation = useRunBacktest()

  // Selected strategy info
  const selectedStrategy = strategies?.find((s) => s.id === selectedStrategyId) ?? null

  // When a strategy is selected, initialize params from defaults
  const handleStrategySelect = useCallback(
    (strategy: StrategyInfo) => {
      setSelectedStrategyId(strategy.id)
      const defaults: Record<string, unknown> = {}
      for (const p of strategy.params) {
        defaults[p.name] = p.default
      }
      setParams(defaults)
    },
    [],
  )

  const handleParamChange = useCallback((name: string, value: unknown) => {
    setParams((prev) => ({ ...prev, [name]: value }))
  }, [])

  // Track the in-flight run detail
  const selectedRunId = activeBacktest?.id ?? null
  const { data: runDetail } = useBacktestRun(selectedRunId)

  // Auto-select detail from Zustand active backtest when detail loads
  useEffect(() => {
    if (runDetail?.backtest) {
      updateBacktest(runDetail.backtest)
    }
  }, [runDetail, updateBacktest])

  // WebSocket progress
  const handleProgress = useCallback((p: number) => {
    setProgress(p)
  }, [])

  const handleDone = useCallback(
    (_metrics: BacktestMetrics) => {
      setIsRunning(false)
      setProgress(100)
    },
    [],
  )

  useBacktestProgress(isRunning ? activeRunId : null, handleProgress, handleDone)

  const handleRun = useCallback(async () => {
    if (!selectedStrategyId || !symbol.trim()) return

    const name =
      runName.trim() ||
      `${selectedStrategyId.toUpperCase()} ${symbol.toUpperCase()} ${timeframe} ${startDate}`

    setIsRunning(true)
    setProgress(0)

    try {
      const result = await runMutation.mutateAsync({
        name,
        strategy: selectedStrategyId,
        adapter,
        symbol: symbol.trim().toUpperCase(),
        market,
        timeframe,
        start_date: startDate,
        end_date: endDate,
        params,
        initial_cash: initialCash,
        commission,
      })
      setActiveRunId(result.run_id)
    } catch {
      setIsRunning(false)
    }
  }, [
    selectedStrategyId,
    symbol,
    runName,
    timeframe,
    startDate,
    endDate,
    adapter,
    market,
    params,
    initialCash,
    commission,
    runMutation,
  ])

  const inputClass =
    'w-full bg-background border border-border rounded px-3 py-1.5 text-sm outline-none focus:ring-2 focus:ring-primary'
  const labelClass = 'text-xs text-muted-foreground uppercase tracking-wide mb-1 block'

  const trades = runDetail?.trades ?? []
  const equityCurve = runDetail?.equity_curve ?? []
  const metrics = runDetail?.backtest?.metrics

  const runCanRun = Boolean(selectedStrategyId && symbol.trim() && !isRunning)

  return (
    <div className="flex h-[calc(100vh-4rem)] overflow-hidden">
      {/* ── Left panel ── */}
      <div className="w-1/3 min-w-[280px] flex flex-col border-r border-border overflow-y-auto p-4 gap-6">
        {/* Strategy selector */}
        <section>
          <h2 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground mb-3">
            Strategy
          </h2>
          {strategiesLoading ? (
            <div className="grid grid-cols-2 gap-2">
              {Array.from({ length: 4 }).map((_, i) => (
                <StrategyCardSkeleton key={i} />
              ))}
            </div>
          ) : !strategies || strategies.length === 0 ? (
            <p className="text-sm text-muted-foreground">No strategies available.</p>
          ) : (
            <div className="grid grid-cols-2 gap-2">
              {strategies.map((s) => (
                <StrategyCard
                  key={s.id}
                  strategy={s}
                  selected={s.id === selectedStrategyId}
                  onSelect={() => handleStrategySelect(s)}
                />
              ))}
            </div>
          )}
        </section>

        {/* Params form */}
        {selectedStrategy && selectedStrategy.params.length > 0 && (
          <section>
            <h2 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground mb-3">
              Parameters — {selectedStrategy.name}
            </h2>
            <div className="flex flex-col gap-4">
              {selectedStrategy.params.map((param) => (
                <ParamField
                  key={param.name}
                  param={param}
                  value={params[param.name] ?? param.default}
                  onChange={(v) => handleParamChange(param.name, v)}
                />
              ))}
            </div>
          </section>
        )}

        {/* Run configuration */}
        <section>
          <h2 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground mb-3">
            Configuration
          </h2>
          <div className="flex flex-col gap-3">
            {/* Adapter */}
            <div>
              <label className={labelClass}>Adapter</label>
              <select
                className={inputClass}
                value={adapter}
                onChange={(e) => setAdapter(e.target.value)}
              >
                {(adapters ?? [{ id: 'binance', markets: ['crypto'] }]).map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.id.charAt(0).toUpperCase() + a.id.slice(1)}
                  </option>
                ))}
              </select>
            </div>

            {/* Symbol */}
            <div>
              <label className={labelClass}>Symbol</label>
              <input
                type="text"
                className={inputClass}
                placeholder="e.g. BTCUSDT"
                value={symbol}
                onChange={(e) => setSymbol(e.target.value)}
              />
            </div>

            {/* Market */}
            <div>
              <label className={labelClass}>Market</label>
              <select
                className={inputClass}
                value={market}
                onChange={(e) => setMarket(e.target.value)}
              >
                {MARKETS.map((m) => (
                  <option key={m} value={m}>
                    {m.charAt(0).toUpperCase() + m.slice(1)}
                  </option>
                ))}
              </select>
            </div>

            {/* Timeframe */}
            <div>
              <label className={labelClass}>Timeframe</label>
              <div className="flex flex-wrap gap-1">
                {TIMEFRAMES.map((tf) => (
                  <button
                    key={tf}
                    type="button"
                    onClick={() => setTimeframe(tf)}
                    className={`px-2 py-1 text-xs rounded border transition-colors ${
                      timeframe === tf
                        ? 'bg-primary text-primary-foreground border-primary'
                        : 'border-border text-muted-foreground hover:text-foreground hover:border-primary/50'
                    }`}
                  >
                    {tf}
                  </button>
                ))}
              </div>
            </div>

            {/* Date range */}
            <div className="grid grid-cols-2 gap-2">
              <div>
                <label className={labelClass}>Start Date</label>
                <input
                  type="date"
                  className={inputClass}
                  value={startDate}
                  onChange={(e) => setStartDate(e.target.value)}
                />
              </div>
              <div>
                <label className={labelClass}>End Date</label>
                <input
                  type="date"
                  className={inputClass}
                  value={endDate}
                  onChange={(e) => setEndDate(e.target.value)}
                />
              </div>
            </div>

            {/* Initial cash */}
            <div>
              <label className={labelClass}>Initial Cash ($)</label>
              <input
                type="number"
                className={inputClass}
                min={100}
                step={100}
                value={initialCash}
                onChange={(e) => setInitialCash(Number(e.target.value))}
              />
            </div>

            {/* Commission */}
            <div>
              <label className={labelClass}>Commission (e.g. 0.001 = 0.1%)</label>
              <input
                type="number"
                className={inputClass}
                min={0}
                max={0.1}
                step={0.0001}
                value={commission}
                onChange={(e) => setCommission(Number(e.target.value))}
              />
            </div>

            {/* Run name */}
            <div>
              <label className={labelClass}>Run Name (optional)</label>
              <input
                type="text"
                className={inputClass}
                placeholder="Auto-generated if empty"
                value={runName}
                onChange={(e) => setRunName(e.target.value)}
              />
            </div>
          </div>
        </section>

        {/* Run button + progress */}
        <section className="sticky bottom-0 bg-background pt-2 pb-1 -mx-4 px-4">
          <button
            type="button"
            onClick={handleRun}
            disabled={!runCanRun}
            className="w-full flex items-center justify-center gap-2 py-2.5 rounded-lg bg-primary text-primary-foreground font-medium text-sm transition-colors hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {isRunning ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              <Play className="h-4 w-4" />
            )}
            {isRunning ? 'Running…' : 'Run Backtest'}
          </button>

          {isRunning && (
            <div className="mt-3">
              <div className="flex items-center justify-between text-xs text-muted-foreground mb-1">
                <span>Progress</span>
                <span className="tabular-nums">{progress}%</span>
              </div>
              <div className="h-2 bg-muted rounded-full overflow-hidden">
                <div
                  className="h-full bg-primary rounded-full transition-all duration-300"
                  style={{ width: `${progress}%` }}
                />
              </div>
            </div>
          )}

          {!selectedStrategyId && (
            <p className="mt-2 text-xs text-muted-foreground text-center">
              Select a strategy to get started
            </p>
          )}
          {selectedStrategyId && !symbol.trim() && (
            <p className="mt-2 text-xs text-muted-foreground text-center">
              Enter a symbol to run
            </p>
          )}
        </section>

        {/* Recent runs */}
        {recentRuns && recentRuns.length > 0 && (
          <section>
            <h2 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground mb-3">
              Recent Runs
            </h2>
            <div className="flex flex-col gap-2">
              {recentRuns.slice(0, 10).map((run) => (
                <button
                  key={run.id}
                  type="button"
                  onClick={() => setActiveBacktest(run)}
                  className={`text-left p-3 rounded-lg border transition-all ${
                    activeBacktest?.id === run.id
                      ? 'border-primary bg-primary/10'
                      : 'border-border bg-card hover:border-primary/40 hover:bg-accent'
                  }`}
                >
                  <div className="flex items-center justify-between mb-1">
                    <span className="text-sm font-medium truncate max-w-[60%]">{run.name}</span>
                    <StatusBadge status={run.status} />
                  </div>
                  <div className="text-xs text-muted-foreground flex items-center gap-2">
                    <span>{run.symbol}</span>
                    <span>·</span>
                    <span>{run.timeframe}</span>
                    {run.metrics && (
                      <>
                        <span>·</span>
                        <span
                          className={
                            run.metrics.total_return >= 0 ? 'text-green-500' : 'text-red-500'
                          }
                        >
                          {(run.metrics.total_return * 100).toFixed(2)}%
                        </span>
                      </>
                    )}
                  </div>
                  <div className="text-xs text-muted-foreground mt-0.5">
                    {run.created_at
                      ? (() => {
                          try {
                            return format(parseISO(run.created_at), 'MMM d, yyyy')
                          } catch {
                            return run.created_at
                          }
                        })()
                      : ''}
                  </div>
                </button>
              ))}
            </div>
          </section>
        )}
      </div>

      {/* ── Right panel ── */}
      <div className="flex-1 flex flex-col overflow-hidden">
        {!activeBacktest ? (
          /* Empty state */
          <div className="flex flex-col items-center justify-center h-full gap-4 text-muted-foreground">
            <AlertCircle className="h-14 w-14 opacity-20" />
            <p className="text-lg font-medium">Run a backtest to see results</p>
            <p className="text-sm">Configure a strategy on the left and click Run Backtest</p>
          </div>
        ) : (
          <div className="flex flex-col h-full overflow-hidden">
            {/* Header */}
            <div className="px-6 pt-5 pb-4 border-b border-border flex-shrink-0">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <h2 className="text-lg font-semibold">{activeBacktest.name}</h2>
                  <div className="flex items-center gap-3 mt-1 text-sm text-muted-foreground">
                    <span>{activeBacktest.symbol}</span>
                    <span>·</span>
                    <span>{activeBacktest.timeframe}</span>
                    <span>·</span>
                    <span>
                      {activeBacktest.start_date} — {activeBacktest.end_date}
                    </span>
                  </div>
                </div>
                <StatusBadge status={activeBacktest.status} />
              </div>

              {/* Tabs */}
              <div className="flex items-center gap-1 mt-4">
                {(['overview', 'trades', 'chart'] as ResultTab[]).map((tab) => (
                  <button
                    key={tab}
                    type="button"
                    onClick={() => setResultTab(tab)}
                    className={`px-4 py-1.5 text-sm rounded-md capitalize transition-colors ${
                      resultTab === tab
                        ? 'bg-primary text-primary-foreground'
                        : 'text-muted-foreground hover:text-foreground hover:bg-accent'
                    }`}
                  >
                    {tab}
                  </button>
                ))}
              </div>
            </div>

            {/* Tab content */}
            <div className="flex-1 overflow-y-auto p-6">
              {resultTab === 'overview' && (
                <OverviewTab metrics={metrics} equityCurve={equityCurve} />
              )}
              {resultTab === 'trades' && <TradesTab trades={trades} />}
              {resultTab === 'chart' && (
                <ChartTab
                  adapter={adapter}
                  symbol={activeBacktest.symbol}
                  market={activeBacktest.market}
                  timeframe={activeBacktest.timeframe}
                  startDate={activeBacktest.start_date}
                  endDate={activeBacktest.end_date}
                />
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
