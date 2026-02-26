import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { X } from 'lucide-react'
import { createPortfolio } from '@/api/portfolio'
import { usePortfolioStore } from '@/stores'
import type { PortfolioType } from '@/types'

interface Props {
  open: boolean
  onClose: () => void
}

export function NewPortfolioModal({ open, onClose }: Props) {
  const qc = useQueryClient()
  const { setActivePortfolioId } = usePortfolioStore()
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [type, setType] = useState<PortfolioType>('manual')
  const [currency, setCurrency] = useState('USD')
  const [initialCash, setInitialCash] = useState('0')

  const mut = useMutation({
    mutationFn: createPortfolio,
    onSuccess: (p) => {
      qc.invalidateQueries({ queryKey: ['portfolios'] })
      setActivePortfolioId(p.id)
      setName('')
      setDescription('')
      setInitialCash('0')
      onClose()
    },
  })

  if (!open) return null

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) return
    mut.mutate({
      name: name.trim(),
      description,
      type,
      currency,
      initial_cash: parseFloat(initialCash) || 0,
    })
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="bg-background rounded-lg shadow-xl w-[480px] border">
        <div className="flex items-center justify-between px-4 py-3 border-b">
          <h2 className="font-semibold text-sm">New Portfolio</h2>
          <button onClick={onClose} className="text-muted-foreground hover:text-foreground">
            <X className="w-4 h-4" />
          </button>
        </div>
        <form onSubmit={handleSubmit} className="p-4 space-y-3">
          <div>
            <label className="block text-xs font-medium text-muted-foreground mb-1">
              Name *
            </label>
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="My Crypto Portfolio"
              className="w-full bg-muted border border-border rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
              required
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-muted-foreground mb-1">
              Description
            </label>
            <input
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Optional"
              className="w-full bg-muted border border-border rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-xs font-medium text-muted-foreground mb-1">
                Type
              </label>
              <select
                value={type}
                onChange={(e) => setType(e.target.value as PortfolioType)}
                className="w-full bg-muted border border-border rounded px-3 py-2 text-sm focus:outline-none"
              >
                <option value="manual">Manual</option>
                <option value="paper">Paper</option>
                <option value="live">Live</option>
              </select>
            </div>
            <div>
              <label className="block text-xs font-medium text-muted-foreground mb-1">
                Currency
              </label>
              <select
                value={currency}
                onChange={(e) => setCurrency(e.target.value)}
                className="w-full bg-muted border border-border rounded px-3 py-2 text-sm focus:outline-none"
              >
                <option value="USD">USD</option>
                <option value="EUR">EUR</option>
                <option value="BTC">BTC</option>
              </select>
            </div>
          </div>
          <div>
            <label className="block text-xs font-medium text-muted-foreground mb-1">
              Initial Cash
            </label>
            <input
              type="number"
              min="0"
              step="100"
              value={initialCash}
              onChange={(e) => setInitialCash(e.target.value)}
              className="w-full bg-muted border border-border rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
            />
          </div>
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
              disabled={!name.trim() || mut.isPending}
              className="px-4 py-2 text-sm bg-primary text-primary-foreground rounded hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              {mut.isPending ? 'Creating…' : 'Create'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
