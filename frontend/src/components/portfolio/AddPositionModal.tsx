import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { X } from 'lucide-react'
import { addPosition, updatePosition } from '@/api/portfolio'
import type { Position } from '@/types'

interface Props {
  open: boolean
  portfolioId: number
  editingPosition?: Position | null
  onClose: () => void
}

export function AddPositionModal({ open, portfolioId, editingPosition, onClose }: Props) {
  const qc = useQueryClient()
  const [adapterID, setAdapterID] = useState(editingPosition?.adapter_id ?? 'binance')
  const [symbol, setSymbol] = useState(editingPosition?.symbol ?? '')
  const [market, setMarket] = useState(editingPosition?.market ?? 'crypto')
  const [quantity, setQuantity] = useState(editingPosition?.quantity?.toString() ?? '')
  const [avgCost, setAvgCost] = useState(editingPosition?.avg_cost?.toString() ?? '')
  const [openedAt, setOpenedAt] = useState(
    editingPosition?.opened_at?.slice(0, 10) ?? new Date().toISOString().slice(0, 10),
  )

  const addMut = useMutation({
    mutationFn: () =>
      addPosition(portfolioId, {
        adapter_id: adapterID,
        symbol: symbol.trim().toUpperCase(),
        market,
        quantity: parseFloat(quantity),
        avg_cost: parseFloat(avgCost),
        opened_at: new Date(openedAt).toISOString(),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['portfolio', portfolioId] })
      onClose()
    },
  })

  const editMut = useMutation({
    mutationFn: () =>
      updatePosition(portfolioId, editingPosition!.id, {
        quantity: parseFloat(quantity),
        avg_cost: parseFloat(avgCost),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['portfolio', portfolioId] })
      onClose()
    },
  })

  if (!open) return null

  const isEditing = !!editingPosition
  const isValid = symbol.trim() && parseFloat(quantity) > 0 && parseFloat(avgCost) > 0
  const isPending = addMut.isPending || editMut.isPending

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!isValid) return
    if (isEditing) editMut.mutate()
    else addMut.mutate()
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="bg-background rounded-lg shadow-xl w-[480px] border">
        <div className="flex items-center justify-between px-4 py-3 border-b">
          <h2 className="font-semibold text-sm">
            {isEditing ? 'Edit Position' : 'Add Position'}
          </h2>
          <button onClick={onClose} className="text-muted-foreground hover:text-foreground">
            <X className="w-4 h-4" />
          </button>
        </div>
        <form onSubmit={handleSubmit} className="p-4 space-y-3">
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-xs font-medium text-muted-foreground mb-1">
                Adapter
              </label>
              <select
                value={adapterID}
                onChange={(e) => setAdapterID(e.target.value)}
                disabled={isEditing}
                className="w-full bg-muted border border-border rounded px-3 py-2 text-sm focus:outline-none disabled:opacity-50"
              >
                <option value="binance">Binance (Crypto)</option>
                <option value="yahoo">Yahoo (Stocks)</option>
              </select>
            </div>
            <div>
              <label className="block text-xs font-medium text-muted-foreground mb-1">
                Market
              </label>
              <select
                value={market}
                onChange={(e) => setMarket(e.target.value)}
                disabled={isEditing}
                className="w-full bg-muted border border-border rounded px-3 py-2 text-sm focus:outline-none disabled:opacity-50"
              >
                <option value="crypto">Crypto</option>
                <option value="stock">Stock</option>
              </select>
            </div>
          </div>
          <div>
            <label className="block text-xs font-medium text-muted-foreground mb-1">
              Symbol *
            </label>
            <input
              value={symbol}
              onChange={(e) => setSymbol(e.target.value)}
              placeholder={adapterID === 'binance' ? 'BTCUSDT' : 'AAPL'}
              disabled={isEditing}
              className="w-full bg-muted border border-border rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-ring disabled:opacity-50"
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-xs font-medium text-muted-foreground mb-1">
                Quantity *
              </label>
              <input
                type="number"
                min="0"
                step="any"
                value={quantity}
                onChange={(e) => setQuantity(e.target.value)}
                className="w-full bg-muted border border-border rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-muted-foreground mb-1">
                Avg Cost *
              </label>
              <input
                type="number"
                min="0"
                step="any"
                value={avgCost}
                onChange={(e) => setAvgCost(e.target.value)}
                className="w-full bg-muted border border-border rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
              />
            </div>
          </div>
          {!isEditing && (
            <div>
              <label className="block text-xs font-medium text-muted-foreground mb-1">
                Opened At
              </label>
              <input
                type="date"
                value={openedAt}
                onChange={(e) => setOpenedAt(e.target.value)}
                className="w-full bg-muted border border-border rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
              />
            </div>
          )}
          <div className="flex justify-end gap-2 pt-2">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-sm border border-border rounded hover:bg-accent transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={!isValid || isPending}
              className="px-4 py-2 text-sm bg-primary text-primary-foreground rounded hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              {isPending ? 'Saving…' : isEditing ? 'Save' : 'Add'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
