import { useEffect, useRef } from 'react'
import {
  createChart,
  ColorType,
  type IChartApi,
  type ISeriesApi,
  type UTCTimestamp,
} from 'lightweight-charts'
import { X } from 'lucide-react'
import type { ActiveIndicator } from '@/types'

interface PanelChartProps {
  indicator: ActiveIndicator
  onClose: () => void
  isDark: boolean
}

export function PanelChart({ indicator, onClose, isDark }: PanelChartProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const chartRef = useRef<IChartApi | null>(null)

  useEffect(() => {
    if (!containerRef.current) return

    const bg = isDark ? '#0f1117' : '#ffffff'
    const textColor = isDark ? '#d1d5db' : '#374151'
    const gridColor = isDark ? '#1f2937' : '#f3f4f6'

    const chart = createChart(containerRef.current, {
      width: containerRef.current.clientWidth,
      height: 120,
      layout: {
        background: { type: ColorType.Solid, color: bg },
        textColor,
      },
      grid: {
        vertLines: { color: gridColor },
        horzLines: { color: gridColor },
      },
      rightPriceScale: { borderVisible: false },
      timeScale: { borderVisible: false, visible: false },
      crosshair: { horzLine: { visible: false } },
      handleScroll: false,
      handleScale: false,
    })
    chartRef.current = chart

    const result = indicator.result
    if (result && result.timestamps.length > 0) {
      indicator.meta.outputs.forEach((output) => {
        const series = result.series[output.name]
        if (!series) return

        let s: ISeriesApi<'Line'> | ISeriesApi<'Histogram'>

        if (output.name === 'histogram') {
          s = chart.addHistogramSeries({ color: output.color })
        } else {
          s = chart.addLineSeries({ color: output.color, lineWidth: 1 })
        }

        const points = result.timestamps
          .map((ts, i) => ({ time: ts as UTCTimestamp, value: series[i] }))
          .filter((p): p is { time: UTCTimestamp; value: number } =>
            p.value !== null && p.value !== undefined && !isNaN(p.value as number),
          )

        s.setData(points)
      })

      chart.timeScale().fitContent()
    }

    const ro = new ResizeObserver(() => {
      if (containerRef.current && chartRef.current) {
        chartRef.current.applyOptions({ width: containerRef.current.clientWidth })
      }
    })
    ro.observe(containerRef.current)

    return () => {
      ro.disconnect()
      chart.remove()
      chartRef.current = null
    }
  }, [indicator, isDark])

  const paramSummary = Object.entries(indicator.params)
    .map(([k, v]) => `${k}:${v}`)
    .join(', ')

  return (
    <div className="border-t border-border">
      <div className="flex items-center justify-between px-3 py-1 text-xs text-muted-foreground">
        <span>
          <span className="font-medium text-foreground">{indicator.meta.name}</span>
          {paramSummary && ` (${paramSummary})`}
        </span>
        <button
          onClick={onClose}
          className="hover:text-destructive transition-colors"
          aria-label={`Remove ${indicator.meta.name}`}
        >
          <X className="w-3 h-3" />
        </button>
      </div>
      <div ref={containerRef} />
    </div>
  )
}
