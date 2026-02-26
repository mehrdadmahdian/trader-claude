import { useState, useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { PortfolioSelector } from '@/components/portfolio/PortfolioSelector'
import { SummaryCards } from '@/components/portfolio/SummaryCards'
import { PositionsTable } from '@/components/portfolio/PositionsTable'
import { AllocationDonut } from '@/components/portfolio/AllocationDonut'
import { fetchPortfolio, fetchPortfolioSummary } from '@/api/portfolio'
import { usePortfolioStore } from '@/stores'
import type { Position } from '@/types'

export function Portfolio() {
  const { activePortfolioId, setPositions, setSummary, summary, positions } =
    usePortfolioStore()
  const [highlightedSymbol, setHighlightedSymbol] = useState<string | null>(null)
  const [showNewPortfolioModal, setShowNewPortfolioModal] = useState(false)
  const [editingPosition, setEditingPosition] = useState<Position | null>(null)
  const [showAddPosition, setShowAddPosition] = useState(false)

  // Load portfolio + positions
  const portfolioQuery = useQuery({
    queryKey: ['portfolio', activePortfolioId],
    queryFn: () => fetchPortfolio(activePortfolioId!),
    enabled: !!activePortfolioId,
  })

  useEffect(() => {
    if (portfolioQuery.data) {
      setPositions(portfolioQuery.data.positions)
    }
  }, [portfolioQuery.data, setPositions])

  // Load summary
  const summaryQuery = useQuery({
    queryKey: ['portfolio-summary', activePortfolioId],
    queryFn: () => fetchPortfolioSummary(activePortfolioId!),
    enabled: !!activePortfolioId,
    refetchInterval: 30_000,
  })

  useEffect(() => {
    if (summaryQuery.data) {
      setSummary(summaryQuery.data)
    }
  }, [summaryQuery.data, setSummary])

  return (
    <div className="space-y-6">
      {/* Header row */}
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Portfolio</h1>
        <PortfolioSelector onNewPortfolio={() => setShowNewPortfolioModal(true)} />
      </div>

      {/* Summary cards */}
      <SummaryCards summary={activePortfolioId ? summary : null} />

      {!activePortfolioId && (
        <div className="rounded-lg border border-border bg-card p-12 text-center text-muted-foreground">
          Select or create a portfolio to get started.
        </div>
      )}

      {activePortfolioId && (
        <>
          {/* Main split: table 60% + donut 40% */}
          <div className="grid grid-cols-1 lg:grid-cols-5 gap-4">
            <div className="lg:col-span-3">
              <PositionsTable
                portfolioId={activePortfolioId}
                positions={positions}
                highlightedSymbol={highlightedSymbol ?? undefined}
                onAddPosition={() => setShowAddPosition(true)}
                onEditPosition={setEditingPosition}
              />
            </div>
            <div className="lg:col-span-2">
              <AllocationDonut positions={positions} onHover={setHighlightedSymbol} />
            </div>
          </div>

          {/* Bottom tabs */}
          <div>
            <div className="flex border-b border-border mb-4">
              <button className="px-4 py-2 text-sm font-medium border-b-2 border-primary text-primary -mb-px">
                Equity Curve
              </button>
              <button className="px-4 py-2 text-sm font-medium text-muted-foreground hover:text-foreground">
                Transactions
              </button>
            </div>
            <div className="rounded-lg border border-border bg-card p-6 text-muted-foreground text-sm text-center">
              Equity curve and transactions — implemented in Task 8
            </div>
          </div>
        </>
      )}

      {/* Suppress unused state warnings — modals wired in Task 8 */}
      {showNewPortfolioModal && null}
      {editingPosition && null}
      {showAddPosition && null}
    </div>
  )
}
