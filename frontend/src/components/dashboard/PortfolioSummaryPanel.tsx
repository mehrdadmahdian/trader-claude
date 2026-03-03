import { Link } from 'react-router-dom'
import { Briefcase, TrendingUp, TrendingDown } from 'lucide-react'
import { cn } from '@/lib/utils'
import { usePortfolioStore } from '@/stores'

const usdFormatter = new Intl.NumberFormat('en-US', {
  style: 'currency',
  currency: 'USD',
  maximumFractionDigits: 2,
})
const formatUSD = (n: number) => usdFormatter.format(n)
const formatPct = (n: number) => `${n >= 0 ? '+' : ''}${n.toFixed(2)}%`

function StatCard({
  label,
  value,
  delta,
  deltaPositive,
}: {
  label: string
  value: string
  delta?: string
  deltaPositive?: boolean
}) {
  return (
    <div className="flex flex-col gap-0.5 p-3 rounded-xl bg-slate-50 border border-slate-100">
      <span className="text-[10px] font-semibold uppercase tracking-wider text-slate-400">{label}</span>
      <span className="font-mono text-sm font-bold text-slate-900 truncate">{value}</span>
      {delta !== undefined && (
        <span className={cn(
          'inline-flex items-center gap-1 text-[10px] font-mono rounded-full w-fit px-1.5 py-0.5 mt-0.5',
          deltaPositive ? 'bg-green-50 text-green-600' : 'bg-red-50 text-red-600',
        )}>
          {deltaPositive
            ? <TrendingUp className="w-2.5 h-2.5" />
            : <TrendingDown className="w-2.5 h-2.5" />
          }
          {delta}
        </span>
      )}
    </div>
  )
}

export function PortfolioSummaryPanel({ className }: { className?: string }) {
  const { summary, positions, activePortfolioId } = usePortfolioStore()

  return (
    <div className={cn("flex flex-col rounded-2xl bg-white shadow-sm border border-slate-100 overflow-hidden", className)}>
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-slate-100 shrink-0">
        <div className="flex items-center gap-2">
          <Briefcase className="w-3.5 h-3.5 text-slate-400" />
          <span className="text-xs font-semibold uppercase tracking-wider text-slate-400">Portfolio</span>
        </div>
        {activePortfolioId && (
          <span className="text-[10px] text-slate-300 font-mono">#{activePortfolioId}</span>
        )}
      </div>

      {/* Stats */}
      <div className="p-3 space-y-2">
        {!activePortfolioId || !summary ? (
          <div className="flex flex-col items-center gap-2 py-4 text-center">
            <Briefcase className="w-6 h-6 text-slate-200" />
            <p className="text-xs text-slate-400">No portfolio selected</p>
            <Link to="/portfolio" className="text-xs text-primary hover:underline">
              Open Portfolio →
            </Link>
          </div>
        ) : (
          <>
            <StatCard
              label="Total Value"
              value={formatUSD(summary.total_value)}
            />
            <div className="grid grid-cols-2 gap-2">
              <StatCard
                label="P&L"
                value={formatUSD(summary.total_pnl)}
                delta={formatPct(summary.total_pnl_pct)}
                deltaPositive={summary.total_pnl >= 0}
              />
              <StatCard
                label="Positions"
                value={String(positions.length)}
              />
            </div>
          </>
        )}
      </div>
    </div>
  )
}
