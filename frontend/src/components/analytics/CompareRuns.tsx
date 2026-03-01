import { LineChart, Line, XAxis, YAxis, Tooltip, Legend, ResponsiveContainer } from 'recharts'
import type { CompareResult } from '../../types'

const COLORS = ['#8b5cf6', '#06b6d4', '#10b981', '#f59e0b', '#ef4444']

interface Props {
  result: CompareResult
}

export function CompareRuns({ result }: Props) {
  // Build equity curve overlay data: array of {index, [runName]: value}
  // Each run may have different length equity curves — use index as x axis

  return (
    <div className="space-y-4">
      {/* Metrics table */}
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-zinc-200 dark:border-zinc-700">
              <th className="text-left py-2 pr-4 text-zinc-500 dark:text-zinc-400 font-medium">Metric</th>
              {result.runs.map((r, i) => (
                <th key={r.run_id} className="text-right py-2 px-2 font-medium" style={{ color: COLORS[i] }}>
                  {r.name}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {['total_return', 'sharpe_ratio', 'max_drawdown', 'win_rate', 'total_trades', 'profit_factor'].map(key => (
              <tr key={key} className="border-b border-zinc-100 dark:border-zinc-800">
                <td className="py-1.5 pr-4 text-zinc-500 dark:text-zinc-400 capitalize">{key.replace(/_/g, ' ')}</td>
                {result.runs.map(r => {
                  const val = r.metrics[key]
                  const formatted = typeof val === 'number'
                    ? key.includes('rate') || key.includes('return') || key.includes('drawdown')
                      ? `${(Number(val) * 100).toFixed(2)}%`
                      : Number(val).toFixed(2)
                    : String(val ?? '—')
                  return (
                    <td key={r.run_id} className="py-1.5 px-2 text-right text-zinc-900 dark:text-zinc-100 font-mono text-xs">
                      {formatted}
                    </td>
                  )
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Equity curve overlay */}
      <div>
        <p className="text-xs text-zinc-500 dark:text-zinc-400 mb-2">Equity Curves</p>
        <ResponsiveContainer width="100%" height={220}>
          <LineChart data={buildEquityOverlay(result)}>
            <XAxis dataKey="index" hide />
            <YAxis tickFormatter={v => `$${v.toLocaleString()}`} width={70} tick={{ fontSize: 11 }} />
            <Tooltip formatter={(v: number) => `$${v.toLocaleString()}`} />
            <Legend />
            {result.runs.map((r, i) => (
              <Line key={r.run_id} type="monotone" dataKey={r.name} stroke={COLORS[i]} dot={false} strokeWidth={1.5} />
            ))}
          </LineChart>
        </ResponsiveContainer>
      </div>
    </div>
  )
}

function buildEquityOverlay(result: CompareResult) {
  const maxLen = Math.max(...result.runs.map(r => r.equity_curve?.length ?? 0))
  return Array.from({ length: maxLen }, (_, i) => {
    const point: Record<string, unknown> = { index: i }
    for (const run of result.runs) {
      if (run.equity_curve && i < run.equity_curve.length) {
        point[run.name] = run.equity_curve[i].value
      }
    }
    return point
  })
}
