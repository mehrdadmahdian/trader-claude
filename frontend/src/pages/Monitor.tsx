import { useState, useEffect } from 'react'
import { Plus, Play, Pause, Trash2, Zap, Clock, ChevronDown, ChevronUp } from 'lucide-react'
import { formatDistanceToNow } from 'date-fns'
import {
  useMonitors,
  useCreateMonitor,
  useDeleteMonitor,
  useToggleMonitor,
  useMonitorSignals,
} from '@/hooks/useMonitors'
import { useMonitorSignalsWS } from '@/hooks/useMonitorSignalsWS'
import { useMonitorStore } from '@/stores'
import type { Monitor, MonitorCreateRequest, StrategyInfo } from '@/types'

// ── Signal history sub-component ───────────────────────────────────────────

function SignalHistoryTable({ monitorId }: { monitorId: number }) {
  const [page, setPage] = useState(1)
  const { data } = useMonitorSignals(monitorId, page, 20)

  if (!data || data.data.length === 0) {
    return (
      <p className="text-sm text-zinc-400 py-4 text-center">No signals yet.</p>
    )
  }

  const totalPages = Math.ceil(data.total / 20)

  return (
    <div className="mt-3">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-zinc-700 text-left text-zinc-400">
            <th className="pb-2 font-medium">Time</th>
            <th className="pb-2 font-medium">Direction</th>
            <th className="pb-2 font-medium">Price</th>
            <th className="pb-2 font-medium">Strength</th>
          </tr>
        </thead>
        <tbody>
          {data.data.map((sig) => (
            <tr key={sig.id} className="border-b border-zinc-800 last:border-0">
              <td className="py-2 text-zinc-400">
                {formatDistanceToNow(new Date(sig.created_at), { addSuffix: true })}
              </td>
              <td className="py-2">
                <span
                  className={`inline-flex items-center px-2 py-0.5 rounded border text-xs font-medium ${
                    sig.direction === 'long'
                      ? 'text-green-400 border-green-600'
                      : 'text-red-400 border-red-600'
                  }`}
                >
                  {sig.direction.toUpperCase()}
                </span>
              </td>
              <td className="py-2 font-mono">
                ${sig.price.toLocaleString(undefined, { maximumFractionDigits: 2 })}
              </td>
              <td className="py-2">{(sig.strength * 100).toFixed(0)}%</td>
            </tr>
          ))}
        </tbody>
      </table>
      {totalPages > 1 && (
        <div className="flex justify-end gap-2 mt-3">
          <button
            disabled={page === 1}
            onClick={() => setPage((p) => p - 1)}
            className="px-3 py-1.5 rounded border border-zinc-700 text-sm text-zinc-300 hover:bg-zinc-800 disabled:opacity-40 disabled:cursor-not-allowed"
          >
            Prev
          </button>
          <span className="text-sm text-zinc-400 self-center">
            {page} / {totalPages}
          </span>
          <button
            disabled={page === totalPages}
            onClick={() => setPage((p) => p + 1)}
            className="px-3 py-1.5 rounded border border-zinc-700 text-sm text-zinc-300 hover:bg-zinc-800 disabled:opacity-40 disabled:cursor-not-allowed"
          >
            Next
          </button>
        </div>
      )}
    </div>
  )
}

// ── Monitor card ───────────────────────────────────────────────────────────

function MonitorCard({ monitor }: { monitor: Monitor }) {
  const [expanded, setExpanded] = useState(false)
  const toggleMon = useToggleMonitor()
  const deleteMon = useDeleteMonitor()

  const isActive = monitor.status === 'active'
  const isPaper = monitor.mode === 'paper'

  return (
    <div className="rounded-lg border border-zinc-800 bg-zinc-900 p-4 flex flex-col gap-3">
      {/* Header row */}
      <div className="flex items-start justify-between gap-2">
        <div className="flex items-center gap-2 min-w-0">
          <span
            className={`h-2.5 w-2.5 rounded-full flex-shrink-0 ${
              isActive ? 'bg-green-500 animate-pulse' : 'bg-zinc-500'
            }`}
          />
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <p className="font-medium truncate text-zinc-100">{monitor.name}</p>
              {isPaper ? (
                <span className="inline-flex items-center px-1.5 py-0.5 rounded text-xs font-semibold bg-amber-500/15 text-amber-400 border border-amber-500/30">
                  PAPER
                </span>
              ) : (
                <span className="inline-flex items-center px-1.5 py-0.5 rounded text-xs font-semibold bg-green-500/15 text-green-400 border border-green-500/30">
                  LIVE
                </span>
              )}
            </div>
            <p className="text-xs text-zinc-400">
              {monitor.symbol} · {monitor.timeframe} · {monitor.adapter_id}
            </p>
          </div>
        </div>
        <span className="flex-shrink-0 inline-flex items-center px-2 py-0.5 rounded border border-zinc-700 text-xs text-zinc-300">
          {monitor.strategy_name.replace('_', ' ')}
        </span>
      </div>

      {/* Last signal row */}
      {monitor.last_signal_at ? (
        <div className="flex items-center gap-1.5 text-xs">
          <Zap className="h-3.5 w-3.5 text-yellow-400" />
          <span
            className={
              monitor.last_signal_dir === 'long' ? 'text-green-400' : 'text-red-400'
            }
          >
            {monitor.last_signal_dir?.toUpperCase()}
          </span>
          <span className="text-zinc-400">
            @ ${monitor.last_signal_price?.toLocaleString(undefined, { maximumFractionDigits: 2 })}
          </span>
          <span className="text-zinc-400">
            · {formatDistanceToNow(new Date(monitor.last_signal_at), { addSuffix: true })}
          </span>
        </div>
      ) : monitor.last_polled_at ? (
        <div className="flex items-center gap-1.5 text-xs text-zinc-400">
          <Clock className="h-3.5 w-3.5" />
          <span>
            No signals · polled{' '}
            {formatDistanceToNow(new Date(monitor.last_polled_at), { addSuffix: true })}
          </span>
        </div>
      ) : (
        <p className="text-xs text-zinc-500">Waiting for first poll…</p>
      )}

      {/* Action row */}
      <div className="flex items-center justify-between">
        <div className="flex gap-2">
          <button
            onClick={() => toggleMon.mutate(monitor.id)}
            disabled={toggleMon.isPending}
            className="inline-flex items-center gap-1 px-3 py-1.5 rounded border border-zinc-700 text-sm text-zinc-300 hover:bg-zinc-800 disabled:opacity-40 disabled:cursor-not-allowed"
          >
            {isActive ? (
              <>
                <Pause className="h-3.5 w-3.5" /> Pause
              </>
            ) : (
              <>
                <Play className="h-3.5 w-3.5" /> Resume
              </>
            )}
          </button>
          <button
            onClick={() => {
              if (confirm('Delete this monitor?')) deleteMon.mutate(monitor.id)
            }}
            disabled={deleteMon.isPending}
            className="inline-flex items-center gap-1 px-3 py-1.5 rounded border border-zinc-700 text-sm text-red-400 hover:bg-zinc-800 disabled:opacity-40 disabled:cursor-not-allowed"
          >
            <Trash2 className="h-3.5 w-3.5" />
          </button>
        </div>
        <button
          onClick={() => setExpanded((v) => !v)}
          className="inline-flex items-center gap-1 px-2 py-1.5 rounded text-sm text-zinc-400 hover:bg-zinc-800"
        >
          {expanded ? (
            <ChevronUp className="h-4 w-4" />
          ) : (
            <ChevronDown className="h-4 w-4" />
          )}
          <span className="text-xs">Signals</span>
        </button>
      </div>

      {/* Expanded signal history */}
      {expanded && <SignalHistoryTable monitorId={monitor.id} />}
    </div>
  )
}

// ── Create monitor modal ───────────────────────────────────────────────────

const TIMEFRAMES = ['1m', '5m', '15m', '1h', '4h', '1d']

interface CreateModalProps {
  open: boolean
  onClose: () => void
  strategies: StrategyInfo[]
}

function CreateMonitorModal({ open, onClose, strategies }: CreateModalProps) {
  const createMon = useCreateMonitor()
  const [adapterID, setAdapterID] = useState('binance')
  const [symbol, setSymbol] = useState('')
  const [market, setMarket] = useState('crypto')
  const [timeframe, setTimeframe] = useState('1h')
  const [strategyName, setStrategyName] = useState('')
  const [notifyInApp, setNotifyInApp] = useState(true)
  const [mode, setMode] = useState<'live' | 'paper'>('live')
  const [name, setName] = useState('')
  const [error, setError] = useState('')

  if (!open) return null

  function handleSubmit() {
    setError('')
    if (!symbol.trim() || !strategyName) {
      setError('Symbol and strategy are required.')
      return
    }
    const req: MonitorCreateRequest = {
      name: name.trim() || undefined,
      adapter_id: adapterID,
      symbol: symbol.trim().toUpperCase(),
      market,
      timeframe,
      strategy_name: strategyName,
      notify_in_app: notifyInApp,
      mode,
    }
    createMon.mutate(req, {
      onSuccess: () => {
        onClose()
        setSymbol('')
        setStrategyName('')
        setName('')
      },
      onError: (e: Error) => setError(e.message),
    })
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      {/* Backdrop */}
      <div
        className="absolute inset-0 bg-black/60"
        onClick={onClose}
      />

      {/* Modal */}
      <div className="relative z-10 w-full max-w-lg mx-4 rounded-xl border border-zinc-800 bg-zinc-950 shadow-2xl p-6">
        <h2 className="text-lg font-semibold text-zinc-100 mb-4">Create Monitor</h2>

        <div className="space-y-4">
          {/* Adapter + Symbol */}
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1">
              <label className="text-sm font-medium text-zinc-300">Adapter</label>
              <select
                value={adapterID}
                onChange={(e) => setAdapterID(e.target.value)}
                className="w-full h-9 rounded-md border border-zinc-700 bg-zinc-900 px-3 text-sm text-zinc-200 focus:outline-none focus:ring-1 focus:ring-blue-500"
              >
                <option value="binance">Binance</option>
                <option value="yahoo">Yahoo Finance</option>
              </select>
            </div>
            <div className="space-y-1">
              <label className="text-sm font-medium text-zinc-300">Symbol</label>
              <input
                type="text"
                placeholder="e.g. BTCUSDT"
                value={symbol}
                onChange={(e) => setSymbol(e.target.value)}
                className="w-full h-9 rounded-md border border-zinc-700 bg-zinc-900 px-3 text-sm text-zinc-200 placeholder-zinc-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
              />
            </div>
          </div>

          {/* Market */}
          <div className="space-y-1">
            <label className="text-sm font-medium text-zinc-300">Market</label>
            <select
              value={market}
              onChange={(e) => setMarket(e.target.value)}
              className="w-full h-9 rounded-md border border-zinc-700 bg-zinc-900 px-3 text-sm text-zinc-200 focus:outline-none focus:ring-1 focus:ring-blue-500"
            >
              <option value="crypto">Crypto</option>
              <option value="stock">Stock</option>
              <option value="etf">ETF</option>
              <option value="forex">Forex</option>
            </select>
          </div>

          {/* Timeframe */}
          <div className="space-y-1">
            <label className="text-sm font-medium text-zinc-300">Timeframe</label>
            <div className="flex gap-2 flex-wrap">
              {TIMEFRAMES.map((tf) => (
                <button
                  key={tf}
                  type="button"
                  onClick={() => setTimeframe(tf)}
                  className={`px-3 py-1.5 rounded border text-sm transition-colors ${
                    timeframe === tf
                      ? 'border-blue-500 bg-blue-500/10 text-blue-400'
                      : 'border-zinc-700 text-zinc-400 hover:border-zinc-500'
                  }`}
                >
                  {tf}
                </button>
              ))}
            </div>
          </div>

          {/* Strategy */}
          <div className="space-y-1">
            <label className="text-sm font-medium text-zinc-300">Strategy</label>
            {strategies.length === 0 ? (
              <input
                type="text"
                placeholder="Strategy name (e.g. ema_crossover)"
                value={strategyName}
                onChange={(e) => setStrategyName(e.target.value)}
                className="w-full h-9 rounded-md border border-zinc-700 bg-zinc-900 px-3 text-sm text-zinc-200 placeholder-zinc-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
              />
            ) : (
              <div className="grid grid-cols-2 gap-2">
                {strategies.map((s) => (
                  <button
                    key={s.id}
                    type="button"
                    onClick={() => setStrategyName(s.id)}
                    className={`text-left p-3 rounded-lg border text-sm transition-colors ${
                      strategyName === s.id
                        ? 'border-blue-500 bg-blue-500/10'
                        : 'border-zinc-700 hover:border-zinc-500'
                    }`}
                  >
                    <p className="font-medium capitalize text-zinc-200">
                      {s.name.replace('_', ' ')}
                    </p>
                    <p className="text-xs text-zinc-400 line-clamp-1">{s.description}</p>
                  </button>
                ))}
              </div>
            )}
          </div>

          {/* Name (optional) */}
          <div className="space-y-1">
            <label className="text-sm font-medium text-zinc-300">
              Name{' '}
              <span className="text-zinc-500 text-xs">(optional, auto-generated)</span>
            </label>
            <input
              type="text"
              placeholder={
                strategyName
                  ? `${strategyName} ${symbol || 'SYMBOL'} ${timeframe}`
                  : 'Auto-generated'
              }
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="w-full h-9 rounded-md border border-zinc-700 bg-zinc-900 px-3 text-sm text-zinc-200 placeholder-zinc-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
          </div>

          {/* Mode selector */}
          <div className="space-y-1">
            <label className="text-sm font-medium text-zinc-300">Mode</label>
            <div className="flex gap-2">
              <button
                type="button"
                onClick={() => setMode('live')}
                className={`flex-1 py-2 rounded border text-sm font-medium transition-colors ${
                  mode === 'live'
                    ? 'border-green-500 bg-green-500/10 text-green-400'
                    : 'border-zinc-700 text-zinc-400 hover:border-zinc-500'
                }`}
              >
                Live Alert
              </button>
              <button
                type="button"
                onClick={() => setMode('paper')}
                className={`flex-1 py-2 rounded border text-sm font-medium transition-colors ${
                  mode === 'paper'
                    ? 'border-amber-500 bg-amber-500/10 text-amber-400'
                    : 'border-zinc-700 text-zinc-400 hover:border-zinc-500'
                }`}
              >
                Paper Trade
              </button>
            </div>
            <p className="text-xs text-zinc-500">
              {mode === 'live'
                ? 'Monitor sends alerts only — no trades are executed.'
                : 'Monitor auto-executes paper trades when signals fire.'}
            </p>
          </div>

          {/* Notify in-app */}
          <div className="flex items-center justify-between">
            <label className="text-sm font-medium text-zinc-300">In-app notifications</label>
            <input
              type="checkbox"
              checked={notifyInApp}
              onChange={(e) => setNotifyInApp(e.target.checked)}
              className="h-4 w-4 rounded border-zinc-700 bg-zinc-900 text-blue-500"
            />
          </div>

          {error && <p className="text-sm text-red-400">{error}</p>}

          <div className="flex justify-end gap-2 pt-2">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 rounded border border-zinc-700 text-sm text-zinc-300 hover:bg-zinc-800"
            >
              Cancel
            </button>
            <button
              type="button"
              onClick={handleSubmit}
              disabled={createMon.isPending}
              className="px-4 py-2 rounded bg-blue-600 hover:bg-blue-500 text-sm text-white disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {createMon.isPending ? 'Creating…' : 'Create Monitor'}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

// ── Monitor page ───────────────────────────────────────────────────────────

export function Monitor() {
  const { isLoading } = useMonitors()
  const monitors = useMonitorStore((s) => s.monitors)
  const [showCreate, setShowCreate] = useState(false)
  const [strategies, setStrategies] = useState<StrategyInfo[]>([])

  useEffect(() => {
    fetch('/api/v1/strategies')
      .then((r) => r.json())
      .then((d: { data?: StrategyInfo[] }) => setStrategies(d.data ?? []))
      .catch(() => {})
  }, [])

  // Connect to monitor signals WS for all active monitors
  const activeIds = monitors.filter((m) => m.status === 'active').map((m) => m.id)
  useMonitorSignalsWS(activeIds)

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold">Live Monitors</h1>
          <p className="text-sm text-zinc-400 mt-1">
            Strategy-based real-time market watchers
          </p>
        </div>
        <button
          onClick={() => setShowCreate(true)}
          className="inline-flex items-center gap-2 px-4 py-2 rounded bg-blue-600 hover:bg-blue-500 text-sm text-white"
        >
          <Plus className="h-4 w-4" />
          Create Monitor
        </button>
      </div>

      {isLoading && (
        <p className="text-zinc-400">Loading monitors…</p>
      )}

      {!isLoading && monitors.length === 0 && (
        <div className="flex flex-col items-center justify-center py-24 text-center">
          <Zap className="h-12 w-12 text-zinc-600 mb-4" />
          <h2 className="text-lg font-semibold">No monitors yet</h2>
          <p className="text-sm text-zinc-400 mt-1 mb-4">
            Create a monitor to watch a strategy on a live market feed.
          </p>
          <button
            onClick={() => setShowCreate(true)}
            className="inline-flex items-center gap-2 px-4 py-2 rounded bg-blue-600 hover:bg-blue-500 text-sm text-white"
          >
            <Plus className="h-4 w-4" />
            Create your first monitor
          </button>
        </div>
      )}

      {monitors.length > 0 && (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {monitors.map((mon) => (
            <MonitorCard key={mon.id} monitor={mon} />
          ))}
        </div>
      )}

      <CreateMonitorModal
        open={showCreate}
        onClose={() => setShowCreate(false)}
        strategies={strategies}
      />
    </div>
  )
}
