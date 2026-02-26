import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { ChevronLeft, ChevronRight, Plus } from 'lucide-react'
import { fetchTransactions } from '@/api/portfolio'

interface Props {
  portfolioId: number
  onAddTransaction: () => void
}

export function TransactionTable({ portfolioId, onAddTransaction }: Props) {
  const [page, setPage] = useState(1)

  const { data } = useQuery({
    queryKey: ['transactions', portfolioId, page],
    queryFn: () => fetchTransactions(portfolioId, page, 10),
  })

  const txs = data?.data ?? []
  const total = data?.total ?? 0
  const totalPages = Math.ceil(total / 10) || 1

  const typeClass = (t: string) =>
    t === 'buy' || t === 'deposit' ? 'text-green-500' : 'text-red-500'

  return (
    <div className="rounded-lg border border-border bg-card">
      <div className="flex items-center justify-between px-4 py-3 border-b border-border">
        <h3 className="font-semibold text-sm">Transaction History</h3>
        <button
          onClick={onAddTransaction}
          className="flex items-center gap-1 border border-border rounded px-3 py-1.5 text-xs hover:bg-accent transition-colors"
        >
          <Plus className="h-3.5 w-3.5" />
          Log Transaction
        </button>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border text-muted-foreground text-xs">
              <th className="px-4 py-2 text-left font-medium">Date</th>
              <th className="px-4 py-2 text-left font-medium">Type</th>
              <th className="px-4 py-2 text-left font-medium">Symbol</th>
              <th className="px-4 py-2 text-right font-medium">Qty</th>
              <th className="px-4 py-2 text-right font-medium">Price</th>
              <th className="px-4 py-2 text-right font-medium">Fee</th>
              <th className="px-4 py-2 text-left font-medium">Notes</th>
            </tr>
          </thead>
          <tbody>
            {txs.length === 0 && (
              <tr>
                <td
                  colSpan={7}
                  className="px-4 py-8 text-center text-muted-foreground text-sm"
                >
                  No transactions yet.
                </td>
              </tr>
            )}
            {txs.map((tx) => (
              <tr
                key={tx.id}
                className="border-b border-border/50 hover:bg-muted/30 transition-colors"
              >
                <td className="px-4 py-2.5 text-sm">
                  {new Date(tx.executed_at).toLocaleDateString()}
                </td>
                <td className={`px-4 py-2.5 font-medium capitalize ${typeClass(tx.type)}`}>
                  {tx.type}
                </td>
                <td className="px-4 py-2.5">{tx.symbol || '—'}</td>
                <td className="px-4 py-2.5 text-right tabular-nums">
                  {tx.quantity > 0 ? tx.quantity : '—'}
                </td>
                <td className="px-4 py-2.5 text-right tabular-nums">
                  ${tx.price.toFixed(2)}
                </td>
                <td className="px-4 py-2.5 text-right tabular-nums">
                  {tx.fee > 0 ? `$${tx.fee.toFixed(2)}` : '—'}
                </td>
                <td className="px-4 py-2.5 text-muted-foreground text-sm">
                  {tx.notes || '—'}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {totalPages > 1 && (
        <div className="flex items-center justify-end gap-2 p-3 border-t border-border">
          <button
            disabled={page === 1}
            onClick={() => setPage((p) => p - 1)}
            className="p-1 rounded border border-border hover:bg-accent disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
          >
            <ChevronLeft className="h-4 w-4" />
          </button>
          <span className="text-sm text-muted-foreground">
            {page} / {totalPages}
          </span>
          <button
            disabled={page === totalPages}
            onClick={() => setPage((p) => p + 1)}
            className="p-1 rounded border border-border hover:bg-accent disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
          >
            <ChevronRight className="h-4 w-4" />
          </button>
        </div>
      )}
    </div>
  )
}
