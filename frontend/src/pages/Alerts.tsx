import { useState } from 'react'
import { Plus, Trash2, ToggleLeft, ToggleRight, AlertTriangle } from 'lucide-react'
import { formatDistanceToNow } from 'date-fns'
import { useAlerts, useCreateAlert, useDeleteAlert, useToggleAlert } from '@/hooks/useAlerts'
import { cn } from '@/lib/utils'
import type { AlertCondition, AlertCreateRequest, AlertStatus } from '@/types'

// ── AddAlertModal ──────────────────────────────────────────────────────────────

interface AddAlertModalProps {
  onClose: () => void
}

const CONDITIONS: { value: AlertCondition; label: string; hint: string }[] = [
  { value: 'price_above', label: 'Price Above', hint: 'Fires when price exceeds threshold' },
  { value: 'price_below', label: 'Price Below', hint: 'Fires when price falls below threshold' },
  {
    value: 'price_change_pct',
    label: 'Price Change %',
    hint: 'Fires when price moves ±N% from current price',
  },
]

function AddAlertModal({ onClose }: AddAlertModalProps) {
  const { mutateAsync: createAlert, isPending } = useCreateAlert()

  const [form, setForm] = useState<AlertCreateRequest>({
    name: '',
    adapter_id: 'binance',
    symbol: '',
    market: 'crypto',
    condition: 'price_above',
    threshold: 0,
    recurring_enabled: true,
    cooldown_minutes: 60,
  })

  const set = <K extends keyof AlertCreateRequest>(k: K, v: AlertCreateRequest[K]) =>
    setForm((prev) => ({ ...prev, [k]: v }))

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    await createAlert(form)
    onClose()
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
      <div className="bg-card border border-border rounded-lg shadow-xl w-full max-w-md p-6">
        <h2 className="text-lg font-semibold mb-4">New Alert</h2>
        <form onSubmit={handleSubmit} className="space-y-4">
          {/* Name */}
          <div>
            <label className="block text-sm font-medium mb-1">Name</label>
            <input
              type="text"
              className="w-full bg-background border border-border rounded-md px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-primary"
              placeholder="BTC above 100k"
              value={form.name}
              onChange={(e) => set('name', e.target.value)}
              required
            />
          </div>

          {/* Adapter */}
          <div>
            <label className="block text-sm font-medium mb-1">Adapter</label>
            <select
              className="w-full bg-background border border-border rounded-md px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-primary"
              value={form.adapter_id}
              onChange={(e) => set('adapter_id', e.target.value)}
            >
              <option value="binance">Binance (Crypto)</option>
              <option value="yahoo">Yahoo Finance (Stocks)</option>
            </select>
          </div>

          {/* Symbol */}
          <div>
            <label className="block text-sm font-medium mb-1">Symbol</label>
            <input
              type="text"
              className="w-full bg-background border border-border rounded-md px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-primary"
              placeholder="BTCUSDT"
              value={form.symbol}
              onChange={(e) => set('symbol', e.target.value.toUpperCase())}
              required
            />
          </div>

          {/* Condition */}
          <div>
            <label className="block text-sm font-medium mb-1">Condition</label>
            <select
              className="w-full bg-background border border-border rounded-md px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-primary"
              value={form.condition}
              onChange={(e) => set('condition', e.target.value as AlertCondition)}
            >
              {CONDITIONS.map((c) => (
                <option key={c.value} value={c.value}>
                  {c.label}
                </option>
              ))}
            </select>
            <p className="mt-1 text-xs text-muted-foreground">
              {CONDITIONS.find((c) => c.value === form.condition)?.hint}
            </p>
          </div>

          {/* Threshold */}
          <div>
            <label className="block text-sm font-medium mb-1">
              {form.condition === 'price_change_pct' ? 'Change % Threshold' : 'Price Threshold ($)'}
            </label>
            <input
              type="number"
              step="any"
              min="0"
              className="w-full bg-background border border-border rounded-md px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-primary"
              value={form.threshold}
              onChange={(e) => set('threshold', parseFloat(e.target.value) || 0)}
              required
            />
          </div>

          {/* Recurring */}
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">Recurring</p>
              <p className="text-xs text-muted-foreground">Re-fires after cooldown period</p>
            </div>
            <button
              type="button"
              onClick={() => set('recurring_enabled', !form.recurring_enabled)}
              className="text-muted-foreground hover:text-foreground transition-colors"
              aria-label="Toggle recurring"
            >
              {form.recurring_enabled ? (
                <ToggleRight className="w-8 h-8 text-primary" />
              ) : (
                <ToggleLeft className="w-8 h-8" />
              )}
            </button>
          </div>

          {/* Cooldown */}
          {form.recurring_enabled && (
            <div>
              <label className="block text-sm font-medium mb-1">Cooldown (minutes)</label>
              <input
                type="number"
                min="1"
                className="w-full bg-background border border-border rounded-md px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-primary"
                value={form.cooldown_minutes}
                onChange={(e) => set('cooldown_minutes', parseInt(e.target.value) || 60)}
              />
            </div>
          )}

          {/* Actions */}
          <div className="flex gap-3 pt-2">
            <button
              type="button"
              onClick={onClose}
              className="flex-1 px-4 py-2 text-sm border border-border rounded-md hover:bg-accent transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={isPending}
              className="flex-1 px-4 py-2 text-sm bg-primary text-primary-foreground rounded-md hover:bg-primary/90 transition-colors disabled:opacity-50"
            >
              {isPending ? 'Creating…' : 'Create Alert'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

// ── Status badge ───────────────────────────────────────────────────────────────

function StatusBadge({ status }: { status: AlertStatus }) {
  return (
    <span
      className={cn(
        'inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium',
        status === 'active' && 'bg-green-500/15 text-green-600 dark:text-green-400',
        status === 'triggered' && 'bg-yellow-500/15 text-yellow-600 dark:text-yellow-400',
        status === 'disabled' && 'bg-muted text-muted-foreground',
      )}
    >
      {status}
    </span>
  )
}

// ── Alerts page ────────────────────────────────────────────────────────────────

export function Alerts() {
  const [showModal, setShowModal] = useState(false)
  const { data, isLoading, isError } = useAlerts()
  const { mutate: deleteAlert } = useDeleteAlert()
  const { mutate: toggleAlert } = useToggleAlert()

  const alerts = data?.data ?? []

  return (
    <div>
      {/* Page header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold">Alerts</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Price alerts — evaluated every 60 seconds
          </p>
        </div>
        <button
          onClick={() => setShowModal(true)}
          className="flex items-center gap-2 px-4 py-2 bg-primary text-primary-foreground rounded-md text-sm hover:bg-primary/90 transition-colors"
        >
          <Plus className="w-4 h-4" />
          New Alert
        </button>
      </div>

      {/* Table */}
      {isLoading && (
        <p className="text-sm text-muted-foreground">Loading alerts…</p>
      )}
      {isError && (
        <div className="flex items-center gap-2 text-destructive text-sm">
          <AlertTriangle className="w-4 h-4" />
          Failed to load alerts
        </div>
      )}
      {!isLoading && alerts.length === 0 && (
        <div className="text-center py-16 text-muted-foreground">
          <AlertTriangle className="w-12 h-12 mx-auto mb-3 opacity-30" />
          <p className="font-medium">No alerts yet</p>
          <p className="text-sm mt-1">Create your first alert to get notified on price moves</p>
        </div>
      )}

      {alerts.length > 0 && (
        <div className="bg-card border border-border rounded-lg overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border text-muted-foreground">
                <th className="text-left px-4 py-3 font-medium">Name</th>
                <th className="text-left px-4 py-3 font-medium">Symbol</th>
                <th className="text-left px-4 py-3 font-medium">Condition</th>
                <th className="text-left px-4 py-3 font-medium">Threshold</th>
                <th className="text-left px-4 py-3 font-medium">Status</th>
                <th className="text-left px-4 py-3 font-medium">Last Fired</th>
                <th className="text-right px-4 py-3 font-medium">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {alerts.map((alert) => (
                <tr key={alert.id} className="hover:bg-accent/30 transition-colors">
                  <td className="px-4 py-3 font-medium">{alert.name}</td>
                  <td className="px-4 py-3 font-mono text-xs">{alert.symbol}</td>
                  <td className="px-4 py-3 text-muted-foreground">
                    {alert.condition.replace(/_/g, ' ')}
                  </td>
                  <td className="px-4 py-3">
                    {alert.condition === 'price_change_pct'
                      ? `±${alert.threshold}%`
                      : `$${alert.threshold.toLocaleString()}`}
                  </td>
                  <td className="px-4 py-3">
                    <StatusBadge status={alert.status} />
                  </td>
                  <td className="px-4 py-3 text-xs text-muted-foreground">
                    {alert.last_fired_at
                      ? formatDistanceToNow(new Date(alert.last_fired_at), { addSuffix: true })
                      : '—'}
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center justify-end gap-2">
                      <button
                        onClick={() => toggleAlert(alert.id)}
                        className="p-1.5 rounded text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
                        title={alert.status === 'active' ? 'Disable' : 'Enable'}
                        aria-label={alert.status === 'active' ? 'Disable alert' : 'Enable alert'}
                      >
                        {alert.status === 'active' ? (
                          <ToggleRight className="w-4 h-4 text-primary" />
                        ) : (
                          <ToggleLeft className="w-4 h-4" />
                        )}
                      </button>
                      <button
                        onClick={() => deleteAlert(alert.id)}
                        className="p-1.5 rounded text-muted-foreground hover:text-destructive hover:bg-destructive/10 transition-colors"
                        title="Delete alert"
                        aria-label="Delete alert"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Add Alert Modal */}
      {showModal && <AddAlertModal onClose={() => setShowModal(false)} />}
    </div>
  )
}
