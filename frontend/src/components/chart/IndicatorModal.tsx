import { useState, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Search, X } from 'lucide-react'
import { fetchIndicators } from '../../api/indicators'
import type { IndicatorMeta, ActiveIndicator } from '../../types'
import { IndicatorParamForm } from './IndicatorParamForm'

interface Props {
  open: boolean
  onClose: () => void
  active: ActiveIndicator[]
  onAdd: (indicator: ActiveIndicator) => void
}

const GROUPS = [
  { id: 'trend', label: 'Trend' },
  { id: 'momentum', label: 'Momentum' },
  { id: 'volatility', label: 'Volatility' },
  { id: 'volume', label: 'Volume' },
] as const

export function IndicatorModal({ open, onClose, active, onAdd }: Props) {
  const [search, setSearch] = useState('')
  const [selected, setSelected] = useState<IndicatorMeta | null>(null)
  const [paramValues, setParamValues] = useState<Record<string, unknown>>({})

  const { data: indicators = [] } = useQuery({
    queryKey: ['indicators'],
    queryFn: fetchIndicators,
    staleTime: Infinity,
  })

  const filtered = useMemo(
    () =>
      indicators.filter(
        (ind) =>
          ind.full_name.toLowerCase().includes(search.toLowerCase()) ||
          ind.name.toLowerCase().includes(search.toLowerCase()),
      ),
    [indicators, search],
  )

  const grouped = useMemo(
    () =>
      GROUPS.map((g) => ({
        ...g,
        items: filtered.filter((ind) => ind.group === g.id),
      })),
    [filtered],
  )

  function selectIndicator(meta: IndicatorMeta) {
    setSelected(meta)
    const defaults: Record<string, unknown> = {}
    meta.params?.forEach((p) => {
      defaults[p.name] = p.default
    })
    setParamValues(defaults)
  }

  function handleAdd() {
    if (!selected) return
    onAdd({ meta: selected, params: paramValues })
    setSelected(null)
    setParamValues({})
    onClose()
  }

  const isActive = (id: string) => active.some((a) => a.meta.id === id)

  if (!open) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="bg-background rounded-lg shadow-xl w-[560px] max-h-[80vh] flex flex-col border">
        {/* Header */}
        <div className="flex items-center justify-between px-4 py-3 border-b">
          <h2 className="font-semibold text-sm">Indicators</h2>
          <button onClick={onClose} className="text-muted-foreground hover:text-foreground">
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Search */}
        <div className="px-4 py-2 border-b">
          <div className="flex items-center gap-2 border rounded px-3 py-1.5 bg-muted/30">
            <Search className="w-4 h-4 text-muted-foreground shrink-0" />
            <input
              className="flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
              placeholder="Search indicators..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              autoFocus
            />
          </div>
        </div>

        {/* Body */}
        <div className="flex flex-1 overflow-hidden">
          {/* List */}
          <div className="w-52 border-r overflow-y-auto py-2 shrink-0">
            {grouped.map((g) =>
              g.items.length === 0 ? null : (
                <div key={g.id}>
                  <p className="px-3 py-1 text-xs font-semibold uppercase text-muted-foreground tracking-wide">
                    {g.label}
                  </p>
                  {g.items.map((ind) => (
                    <button
                      key={ind.id}
                      onClick={() => selectIndicator(ind)}
                      className={`w-full text-left px-3 py-1.5 text-sm flex items-center justify-between transition-colors
                        hover:bg-accent hover:text-accent-foreground
                        ${selected?.id === ind.id ? 'bg-accent text-accent-foreground' : ''}
                      `}
                    >
                      <span className={isActive(ind.id) ? 'text-muted-foreground' : ''}>
                        {ind.full_name}
                      </span>
                      {isActive(ind.id) && (
                        <span className="w-1.5 h-1.5 rounded-full bg-primary shrink-0" />
                      )}
                    </button>
                  ))}
                </div>
              ),
            )}
          </div>

          {/* Param form */}
          <div className="flex-1 p-4 overflow-y-auto">
            {selected ? (
              <div className="space-y-4">
                <div>
                  <h3 className="font-medium text-sm">{selected.full_name}</h3>
                  <p className="text-xs text-muted-foreground capitalize mt-0.5">
                    {selected.group} · {selected.type}
                  </p>
                </div>
                <IndicatorParamForm
                  params={selected.params ?? []}
                  values={paramValues}
                  onChange={(k, v) => setParamValues((prev) => ({ ...prev, [k]: v }))}
                />
                <button
                  onClick={handleAdd}
                  className="w-full bg-primary text-primary-foreground rounded px-4 py-2 text-sm font-medium hover:bg-primary/90 transition-colors"
                >
                  Add to Chart
                </button>
              </div>
            ) : (
              <p className="text-sm text-muted-foreground mt-2">Select an indicator from the list</p>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
