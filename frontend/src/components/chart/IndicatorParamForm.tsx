import { ParamDefinition } from '../../types'

interface Props {
  params: ParamDefinition[]
  values: Record<string, unknown>
  onChange: (key: string, value: unknown) => void
}

export function IndicatorParamForm({ params, values, onChange }: Props) {
  if (!params || params.length === 0) {
    return <p className="text-sm text-muted-foreground">No parameters</p>
  }
  return (
    <div className="space-y-3">
      {params.map((p) => (
        <div key={p.name}>
          <label className="text-sm font-medium capitalize">{p.name.replace(/_/g, ' ')}</label>
          {p.description && (
            <p className="text-xs text-muted-foreground mb-1">{p.description}</p>
          )}
          {(p.type === 'int' || p.type === 'float') && (
            <div className="flex items-center gap-2">
              <input
                type="range"
                min={p.min as number ?? 1}
                max={p.max as number ?? 500}
                step={p.type === 'float' ? 0.1 : 1}
                value={Number(values[p.name] ?? p.default)}
                onChange={(e) =>
                  onChange(p.name, p.type === 'int' ? parseInt(e.target.value) : parseFloat(e.target.value))
                }
                className="flex-1 accent-primary"
              />
              <input
                type="number"
                min={p.min as number ?? 1}
                max={p.max as number ?? 500}
                step={p.type === 'float' ? 0.1 : 1}
                value={Number(values[p.name] ?? p.default)}
                onChange={(e) =>
                  onChange(p.name, p.type === 'int' ? parseInt(e.target.value) : parseFloat(e.target.value))
                }
                className="w-20 border rounded px-2 py-1 text-sm bg-background"
              />
            </div>
          )}
          {p.type === 'bool' && (
            <input
              type="checkbox"
              checked={Boolean(values[p.name] ?? p.default)}
              onChange={(e) => onChange(p.name, e.target.checked)}
              className="ml-1"
            />
          )}
          {p.type === 'select' && p.options && (
            <select
              value={String(values[p.name] ?? p.default)}
              onChange={(e) => onChange(p.name, e.target.value)}
              className="border rounded px-2 py-1 text-sm bg-background w-full"
            >
              {p.options.map((o) => (
                <option key={o} value={o}>{o}</option>
              ))}
            </select>
          )}
        </div>
      ))}
    </div>
  )
}
