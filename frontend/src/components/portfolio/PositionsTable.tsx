import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Pencil, Trash2, Plus } from 'lucide-react'
import { deletePosition } from '@/api/portfolio'
import type { Position } from '@/types'

interface Props {
  portfolioId: number
  positions: Position[]
  highlightedSymbol?: string
  onAddPosition: () => void
  onEditPosition: (pos: Position) => void
}

function pnlClass(v: number) {
  return v >= 0 ? 'text-green-500' : 'text-red-500'
}

function fmt(v: number, decimals = 2) {
  return v.toLocaleString('en-US', { minimumFractionDigits: decimals, maximumFractionDigits: decimals })
}

export function PositionsTable({ portfolioId, positions, highlightedSymbol, onAddPosition, onEditPosition }: Props) {
  const qc = useQueryClient()
  const totalValue = positions.reduce((sum, p) => sum + p.current_value, 0)

  const deleteMut = useMutation({
    mutationFn: (posId: number) => deletePosition(portfolioId, posId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['portfolio', portfolioId] }),
  })

  return (
    <div className="rounded-lg border border-border bg-card">
      <div className="flex items-center justify-between px-4 py-3 border-b border-border">
        <h3 className="font-semibold text-sm">Positions</h3>
        <button
          onClick={onAddPosition}
          className="flex items-center gap-1 bg-primary text-primary-foreground rounded px-3 py-1.5 text-xs font-medium hover:bg-primary/90 transition-colors"
        >
          <Plus className="h-3.5 w-3.5" />
          Add Position
        </button>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border text-muted-foreground text-xs">
              <th className="px-4 py-2 text-left font-medium">Asset</th>
              <th className="px-4 py-2 text-right font-medium">Qty</th>
              <th className="px-4 py-2 text-right font-medium">Avg Cost</th>
              <th className="px-4 py-2 text-right font-medium">Price</th>
              <th className="px-4 py-2 text-right font-medium">Value</th>
              <th className="px-4 py-2 text-right font-medium">PnL</th>
              <th className="px-4 py-2 text-right font-medium">PnL %</th>
              <th className="px-4 py-2 text-right font-medium">Weight</th>
              <th className="px-4 py-2" />
            </tr>
          </thead>
          <tbody>
            {positions.length === 0 && (
              <tr>
                <td colSpan={9} className="px-4 py-8 text-center text-muted-foreground text-sm">
                  No positions yet. Add one to get started.
                </td>
              </tr>
            )}
            {positions.map((pos) => {
              const weight = totalValue > 0 ? (pos.current_value / totalValue) * 100 : 0
              const isHighlighted = highlightedSymbol === pos.symbol
              return (
                <tr
                  key={pos.id}
                  className={`border-b border-border/50 hover:bg-muted/30 transition-colors ${isHighlighted ? 'bg-primary/10' : ''}`}
                >
                  <td className="px-4 py-2.5 font-medium">{pos.symbol}</td>
                  <td className="px-4 py-2.5 text-right tabular-nums">{fmt(pos.quantity, 4)}</td>
                  <td className="px-4 py-2.5 text-right tabular-nums">${fmt(pos.avg_cost)}</td>
                  <td className="px-4 py-2.5 text-right tabular-nums">${fmt(pos.current_price)}</td>
                  <td className="px-4 py-2.5 text-right tabular-nums">${fmt(pos.current_value)}</td>
                  <td className={`px-4 py-2.5 text-right tabular-nums ${pnlClass(pos.unrealized_pnl)}`}>
                    ${fmt(pos.unrealized_pnl)}
                  </td>
                  <td className={`px-4 py-2.5 text-right tabular-nums ${pnlClass(pos.unrealized_pnl_pct)}`}>
                    {fmt(pos.unrealized_pnl_pct)}%
                  </td>
                  <td className="px-4 py-2.5 text-right tabular-nums">{fmt(weight)}%</td>
                  <td className="px-4 py-2.5">
                    <div className="flex gap-1 justify-end">
                      <button
                        onClick={() => onEditPosition(pos)}
                        className="p-1 rounded hover:bg-accent text-muted-foreground hover:text-foreground transition-colors"
                      >
                        <Pencil className="h-3.5 w-3.5" />
                      </button>
                      <button
                        onClick={() => deleteMut.mutate(pos.id)}
                        className="p-1 rounded hover:bg-destructive/10 text-muted-foreground hover:text-destructive transition-colors"
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </button>
                    </div>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </div>
  )
}
