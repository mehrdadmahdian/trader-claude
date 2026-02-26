import { useState, useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { PortfolioSelector } from '@/components/portfolio/PortfolioSelector'
import { SummaryCards } from '@/components/portfolio/SummaryCards'
import { PositionsTable } from '@/components/portfolio/PositionsTable'
import { AllocationDonut } from '@/components/portfolio/AllocationDonut'
import { EquityCurveChart } from '@/components/portfolio/EquityCurveChart'
import { TransactionTable } from '@/components/portfolio/TransactionTable'
import { NewPortfolioModal } from '@/components/portfolio/NewPortfolioModal'
import { AddPositionModal } from '@/components/portfolio/AddPositionModal'
import { fetchPortfolio, fetchPortfolioSummary } from '@/api/portfolio'
import { usePortfolioStore } from '@/stores'
import { usePortfolioLive } from '@/hooks/usePortfolioLive'
import type { Position } from '@/types'

export function Portfolio() {
  const { activePortfolioId, setPositions, setSummary, summary, positions } =
    usePortfolioStore()
  const [highlightedSymbol, setHighlightedSymbol] = useState<string | null>(null)
  const [showNewPortfolioModal, setShowNewPortfolioModal] = useState(false)
  const [editingPosition, setEditingPosition] = useState<Position | null>(null)
  const [showAddPosition, setShowAddPosition] = useState(false)
  const [activeTab, setActiveTab] = useState<'equity' | 'transactions'>('equity')

  usePortfolioLive(activePortfolioId)

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
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Portfolio</h1>
        <PortfolioSelector onNewPortfolio={() => setShowNewPortfolioModal(true)} />
      </div>

      <SummaryCards summary={activePortfolioId ? summary : null} />

      {!activePortfolioId && (
        <div className="rounded-lg border border-border bg-card p-12 text-center text-muted-foreground">
          Select or create a portfolio to get started.
        </div>
      )}

      {activePortfolioId && (
        <>
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

          {/* Bottom tabs — raw Tailwind */}
          <div>
            <div className="flex border-b border-border">
              <button
                onClick={() => setActiveTab('equity')}
                className={`px-4 py-2 text-sm font-medium transition-colors -mb-px ${
                  activeTab === 'equity'
                    ? 'border-b-2 border-primary text-primary'
                    : 'text-muted-foreground hover:text-foreground'
                }`}
              >
                Equity Curve
              </button>
              <button
                onClick={() => setActiveTab('transactions')}
                className={`px-4 py-2 text-sm font-medium transition-colors -mb-px ${
                  activeTab === 'transactions'
                    ? 'border-b-2 border-primary text-primary'
                    : 'text-muted-foreground hover:text-foreground'
                }`}
              >
                Transactions
              </button>
            </div>
            <div className="mt-4">
              {activeTab === 'equity' && (
                <div className="rounded-lg border border-border bg-card p-6">
                  <EquityCurveChart portfolioId={activePortfolioId} />
                </div>
              )}
              {activeTab === 'transactions' && (
                <TransactionTable
                  portfolioId={activePortfolioId}
                  onAddTransaction={() => setActiveTab('transactions')}
                />
              )}
            </div>
          </div>
        </>
      )}

      <NewPortfolioModal
        open={showNewPortfolioModal}
        onClose={() => setShowNewPortfolioModal(false)}
      />

      <AddPositionModal
        open={showAddPosition || !!editingPosition}
        portfolioId={activePortfolioId ?? 0}
        editingPosition={editingPosition}
        onClose={() => {
          setShowAddPosition(false)
          setEditingPosition(null)
        }}
      />
    </div>
  )
}
