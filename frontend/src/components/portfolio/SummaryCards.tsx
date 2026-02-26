import type { PortfolioSummary } from '@/types'

interface Props {
  summary: PortfolioSummary | null
}

function StatCard({ label, value, valueClass }: { label: string; value: string; valueClass?: string }) {
  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <p className="text-sm text-muted-foreground">{label}</p>
      <p className={`text-2xl font-bold mt-1 ${valueClass ?? ''}`}>{value}</p>
    </div>
  )
}

function fmtCurrency(v: number) {
  return new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' }).format(v)
}

function fmtPct(v: number) {
  const sign = v >= 0 ? '+' : ''
  return `${sign}${v.toFixed(2)}%`
}

export function SummaryCards({ summary }: Props) {
  if (!summary) {
    return (
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        {[0, 1, 2, 3].map((i) => (
          <div key={i} className="rounded-lg border border-border bg-card p-4 h-24 animate-pulse" />
        ))}
      </div>
    )
  }

  const pnlClass = summary.total_pnl >= 0 ? 'text-green-500' : 'text-red-500'
  const dayClass = summary.day_change_pct >= 0 ? 'text-green-500' : 'text-red-500'

  return (
    <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
      <StatCard label="Total Value" value={fmtCurrency(summary.total_value)} />
      <StatCard label="Total PnL" value={fmtCurrency(summary.total_pnl)} valueClass={pnlClass} />
      <StatCard label="PnL %" value={fmtPct(summary.total_pnl_pct)} valueClass={pnlClass} />
      <StatCard label="Day Change" value={fmtPct(summary.day_change_pct)} valueClass={dayClass} />
    </div>
  )
}
