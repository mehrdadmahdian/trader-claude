import { AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts'
import type { MonteCarloResult } from '../../types'

interface Props {
  result: MonteCarloResult
}

export function MonteCarloChart({ result }: Props) {
  // Build chart data: for each trade index, get all percentile values
  const numPoints = result.percentiles[0]?.values.length ?? 0
  const chartData = Array.from({ length: numPoints }, (_, i) => {
    const point: Record<string, number> = { index: i }
    for (const p of result.percentiles) {
      point[`p${p.percentile}`] = Math.round(p.values[i] ?? 0)
    }
    return point
  })

  const fmt = (v: number) => `${(v * 100).toFixed(1)}%`

  return (
    <div className="space-y-4">
      {/* Stat cards */}
      <div className="grid grid-cols-3 gap-3">
        <StatCard label="Probability of Ruin" value={fmt(result.probability_of_ruin)} highlight={result.probability_of_ruin > 0.1} />
        <StatCard label="Median Return" value={fmt(result.median_return)} />
        <StatCard label="Min / Max Return" value={`${fmt(result.min_return)} / ${fmt(result.max_return)}`} />
      </div>

      {/* Fan chart */}
      <ResponsiveContainer width="100%" height={220}>
        <AreaChart data={chartData}>
          <XAxis dataKey="index" hide />
          <YAxis tickFormatter={v => `$${v.toLocaleString()}`} width={70} tick={{ fontSize: 11 }} />
          <Tooltip formatter={(v: number) => `$${v.toLocaleString()}`} />
          {/* Stacked transparent areas for fan effect */}
          <Area type="monotone" dataKey="p5"  stroke="none" fill="#8b5cf6" fillOpacity={0.1} />
          <Area type="monotone" dataKey="p25" stroke="none" fill="#8b5cf6" fillOpacity={0.15} />
          <Area type="monotone" dataKey="p50" stroke="#8b5cf6" fill="#8b5cf6" fillOpacity={0.2} strokeWidth={2} />
          <Area type="monotone" dataKey="p75" stroke="none" fill="#8b5cf6" fillOpacity={0.15} />
          <Area type="monotone" dataKey="p95" stroke="none" fill="#8b5cf6" fillOpacity={0.1} />
        </AreaChart>
      </ResponsiveContainer>
      <p className="text-xs text-zinc-400 text-center">Trade index → equity (5th–95th percentile, {result.num_simulations.toLocaleString()} simulations)</p>
    </div>
  )
}

function StatCard({ label, value, highlight }: { label: string; value: string; highlight?: boolean }) {
  return (
    <div className="rounded-lg bg-zinc-50 dark:bg-zinc-800 p-3">
      <p className="text-xs text-zinc-500 dark:text-zinc-400 mb-1">{label}</p>
      <p className={`text-sm font-semibold ${highlight ? 'text-red-500' : 'text-zinc-900 dark:text-zinc-100'}`}>{value}</p>
    </div>
  )
}
