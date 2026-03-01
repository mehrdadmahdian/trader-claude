import type { HeatmapResult, HeatmapCell } from '../../types'

interface Props {
  result: HeatmapResult
}

function sharpeToColor(sharpe: number): string {
  // Clamp -1 to 3 range, map to red-yellow-green
  const clamped = Math.max(-1, Math.min(3, sharpe))
  const t = (clamped + 1) / 4 // 0=red, 0.25=yellow, 1=green
  if (t < 0.5) {
    // red → yellow
    const r = 255
    const g = Math.round(255 * (t * 2))
    return `rgb(${r},${g},0)`
  } else {
    // yellow → green
    const r = Math.round(255 * (1 - (t - 0.5) * 2))
    const g = 200
    return `rgb(${r},${g},50)`
  }
}

export function ParamHeatmap({ result }: Props) {
  return (
    <div className="space-y-3">
      <div className="flex items-center gap-4 text-xs text-zinc-500 dark:text-zinc-400">
        <span>X: {result.x_param}</span>
        <span>Y: {result.y_param}</span>
        <span className="ml-auto flex items-center gap-1">
          <span className="inline-block w-4 h-3 rounded" style={{ background: 'rgb(255,0,0)' }} /> Bad
          <span className="inline-block w-4 h-3 rounded ml-2" style={{ background: 'rgb(255,255,0)' }} /> Neutral
          <span className="inline-block w-4 h-3 rounded ml-2" style={{ background: 'rgb(0,200,50)' }} /> Good
        </span>
      </div>

      <div className="overflow-x-auto">
        <div className="inline-grid gap-0.5" style={{ gridTemplateColumns: `auto repeat(${result.x_values.length}, minmax(40px, 1fr))` }}>
          {/* Header row */}
          <div />
          {result.x_values.map(x => (
            <div key={x} className="text-center text-[10px] text-zinc-400 pb-1">{x.toFixed(0)}</div>
          ))}

          {/* Data rows (y values descending so top = max) */}
          {[...result.cells].reverse().map((row, ri) => {
            const yi = result.cells.length - 1 - ri
            return (
              <>
                <div key={`label-${yi}`} className="text-[10px] text-zinc-400 pr-1 flex items-center justify-end">
                  {result.y_values[yi]?.toFixed(0)}
                </div>
                {row.map((cell, xi) => (
                  <HeatmapCellView key={`${yi}-${xi}`} cell={cell} />
                ))}
              </>
            )
          })}
        </div>
      </div>
    </div>
  )
}

function HeatmapCellView({ cell }: { cell: HeatmapCell }) {
  const bg = sharpeToColor(cell.sharpe_ratio)
  const title = `Sharpe: ${cell.sharpe_ratio.toFixed(2)}\nReturn: ${(cell.total_return * 100).toFixed(1)}%\nDrawdown: ${(cell.max_drawdown * 100).toFixed(1)}%\nTrades: ${cell.total_trades}`
  return (
    <div
      className="h-8 rounded cursor-default flex items-center justify-center text-[9px] font-bold text-black/70"
      style={{ background: bg }}
      title={title}
    >
      {cell.sharpe_ratio.toFixed(1)}
    </div>
  )
}
