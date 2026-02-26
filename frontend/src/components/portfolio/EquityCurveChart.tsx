import { useQuery } from '@tanstack/react-query'
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from 'recharts'
import { fetchEquityCurve } from '@/api/portfolio'

interface Props {
  portfolioId: number
}

export function EquityCurveChart({ portfolioId }: Props) {
  const { data: points = [], isLoading } = useQuery({
    queryKey: ['equity-curve', portfolioId],
    queryFn: () => fetchEquityCurve(portfolioId),
  })

  if (isLoading) return <div className="h-48 animate-pulse rounded bg-muted" />

  if (points.length === 0) {
    return (
      <div className="h-48 flex items-center justify-center text-muted-foreground text-sm">
        No transaction history yet
      </div>
    )
  }

  const chartData = points.map((p) => ({
    time: new Date(p.timestamp).toLocaleDateString(),
    value: p.value,
  }))

  return (
    <ResponsiveContainer width="100%" height={200}>
      <LineChart data={chartData}>
        <CartesianGrid strokeDasharray="3 3" />
        <XAxis dataKey="time" tick={{ fontSize: 11 }} />
        <YAxis
          tick={{ fontSize: 11 }}
          tickFormatter={(v: number) => `$${(v / 1000).toFixed(0)}k`}
        />
        <Tooltip
          formatter={(v: number) => [`$${v.toLocaleString()}`, 'Portfolio Value']}
        />
        <Line
          type="monotone"
          dataKey="value"
          stroke="#6366f1"
          dot={false}
          strokeWidth={2}
        />
      </LineChart>
    </ResponsiveContainer>
  )
}
