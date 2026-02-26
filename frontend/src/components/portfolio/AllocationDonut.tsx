import { PieChart, Pie, Cell, Tooltip, ResponsiveContainer } from 'recharts'
import type { Position } from '@/types'

const COLORS = ['#6366f1', '#22c55e', '#f59e0b', '#ef4444', '#3b82f6', '#a855f7', '#ec4899', '#14b8a6']

interface Props {
  positions: Position[]
  onHover: (symbol: string | null) => void
}

export function AllocationDonut({ positions, onHover }: Props) {
  const totalValue = positions.reduce((sum, p) => sum + p.current_value, 0)
  const data = positions.map((p) => ({
    name: p.symbol,
    value: totalValue > 0 ? (p.current_value / totalValue) * 100 : 0,
  }))

  if (data.length === 0) {
    return (
      <div className="rounded-lg border border-border bg-card h-full flex items-center justify-center p-8">
        <p className="text-muted-foreground text-sm">Add positions to see allocation</p>
      </div>
    )
  }

  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <h3 className="font-semibold text-sm mb-3">Allocation</h3>
      <ResponsiveContainer width="100%" height={260}>
        <PieChart>
          <Pie
            data={data}
            cx="50%"
            cy="50%"
            innerRadius={60}
            outerRadius={100}
            paddingAngle={2}
            dataKey="value"
            onMouseEnter={(_, index) => onHover(data[index].name)}
            onMouseLeave={() => onHover(null)}
          >
            {data.map((_, index) => (
              <Cell key={index} fill={COLORS[index % COLORS.length]} />
            ))}
          </Pie>
          <Tooltip formatter={(v: number) => `${v.toFixed(1)}%`} />
        </PieChart>
      </ResponsiveContainer>
      <div className="flex flex-wrap gap-2 mt-2">
        {data.map((d, i) => (
          <div key={d.name} className="flex items-center gap-1 text-xs">
            <span className="h-2 w-2 rounded-full" style={{ background: COLORS[i % COLORS.length] }} />
            <span>{d.name}</span>
          </div>
        ))}
      </div>
    </div>
  )
}
