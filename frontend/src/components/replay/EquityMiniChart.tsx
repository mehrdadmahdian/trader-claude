import { LineChart, Line, ResponsiveContainer, Tooltip } from 'recharts'
import { useBacktestStore } from '@/stores'

export function EquityMiniChart() {
  const replayEquity = useBacktestStore((s) => s.replayEquity)

  if (replayEquity.length < 2) return null

  const initial = replayEquity[0].value
  const current = replayEquity[replayEquity.length - 1].value
  const pctReturn = initial !== 0 ? ((current - initial) / initial) * 100 : 0
  const isPositive = pctReturn >= 0

  const chartData = replayEquity.map((pt) => ({ value: pt.value }))

  return (
    <div className="absolute bottom-4 right-4 w-48 bg-background/90 backdrop-blur-sm border border-border rounded-lg p-2 pointer-events-none">
      <div className="flex items-baseline justify-between mb-1">
        <span className="text-xs text-muted-foreground">Equity</span>
        <span className={`text-xs font-medium tabular-nums ${isPositive ? 'text-green-500' : 'text-red-500'}`}>
          {isPositive ? '+' : ''}{pctReturn.toFixed(2)}%
        </span>
      </div>
      <span className="text-sm font-semibold tabular-nums">
        ${current.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
      </span>
      <div className="h-12 mt-1">
        <ResponsiveContainer width="100%" height="100%">
          <LineChart data={chartData}>
            <Tooltip
              contentStyle={{ display: 'none' }}
            />
            <Line
              type="monotone"
              dataKey="value"
              stroke={isPositive ? '#22c55e' : '#ef4444'}
              strokeWidth={1.5}
              dot={false}
              isAnimationActive={false}
            />
          </LineChart>
        </ResponsiveContainer>
      </div>
    </div>
  )
}
