import { BarChart, Bar, XAxis, YAxis, Tooltip, Legend, ResponsiveContainer, ReferenceLine } from 'recharts'
import type { WalkForwardResult } from '../../types'

interface Props {
  result: WalkForwardResult
}

export function WalkForwardChart({ result }: Props) {
  const chartData = result.windows.map(w => ({
    name: `W${w.window_index + 1}`,
    'Train Sharpe': parseFloat(w.train_sharpe.toFixed(3)),
    'Test Sharpe': parseFloat(w.test_sharpe.toFixed(3)),
  }))

  const fmt = (v: number) => `${(v * 100).toFixed(1)}%`

  return (
    <div className="space-y-4">
      {/* Summary cards */}
      <div className="grid grid-cols-3 gap-3">
        <div className="rounded-lg bg-zinc-50 dark:bg-zinc-800 p-3">
          <p className="text-xs text-zinc-500 dark:text-zinc-400 mb-1">Avg Test Sharpe</p>
          <p className="text-sm font-semibold text-zinc-900 dark:text-zinc-100">{result.summary.avg_test_sharpe.toFixed(2)}</p>
        </div>
        <div className="rounded-lg bg-zinc-50 dark:bg-zinc-800 p-3">
          <p className="text-xs text-zinc-500 dark:text-zinc-400 mb-1">Avg Test Return</p>
          <p className="text-sm font-semibold text-zinc-900 dark:text-zinc-100">{fmt(result.summary.avg_test_return)}</p>
        </div>
        <div className="rounded-lg bg-zinc-50 dark:bg-zinc-800 p-3">
          <p className="text-xs text-zinc-500 dark:text-zinc-400 mb-1">Consistency</p>
          <p className="text-sm font-semibold text-zinc-900 dark:text-zinc-100">{(result.summary.consistency_ratio * 100).toFixed(0)}%</p>
        </div>
      </div>

      <ResponsiveContainer width="100%" height={220}>
        <BarChart data={chartData} barCategoryGap="20%">
          <XAxis dataKey="name" tick={{ fontSize: 11 }} />
          <YAxis tick={{ fontSize: 11 }} />
          <ReferenceLine y={0} stroke="#71717a" />
          <Tooltip />
          <Legend />
          <Bar dataKey="Train Sharpe" fill="#3b82f6" radius={[2,2,0,0]} />
          <Bar dataKey="Test Sharpe" fill="#f97316" radius={[2,2,0,0]} />
        </BarChart>
      </ResponsiveContainer>
    </div>
  )
}
