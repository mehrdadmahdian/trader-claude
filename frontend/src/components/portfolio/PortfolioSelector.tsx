import { useQuery } from '@tanstack/react-query'
import { Plus, ChevronDown } from 'lucide-react'
import { fetchPortfolios } from '@/api/portfolio'
import { usePortfolioStore } from '@/stores'

interface Props {
  onNewPortfolio: () => void
}

export function PortfolioSelector({ onNewPortfolio }: Props) {
  const { activePortfolioId, setActivePortfolioId } = usePortfolioStore()
  const { data: portfolios = [] } = useQuery({
    queryKey: ['portfolios'],
    queryFn: fetchPortfolios,
  })

  return (
    <div className="flex items-center gap-3">
      <div className="relative">
        <select
          value={activePortfolioId ?? ''}
          onChange={(e) => setActivePortfolioId(Number(e.target.value) || null)}
          className="appearance-none bg-muted border border-border rounded-md pl-3 pr-8 py-1.5 text-sm cursor-pointer focus:outline-none focus:ring-1 focus:ring-ring w-52"
        >
          <option value="">Select portfolio…</option>
          {portfolios.map((p) => (
            <option key={p.id} value={p.id}>{p.name}</option>
          ))}
        </select>
        <ChevronDown className="absolute right-2 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground pointer-events-none" />
      </div>
      <button
        onClick={onNewPortfolio}
        className="flex items-center gap-1 border border-border rounded-md px-3 py-1.5 text-sm hover:bg-accent transition-colors"
      >
        <Plus className="h-4 w-4" />
        New Portfolio
      </button>
    </div>
  )
}
