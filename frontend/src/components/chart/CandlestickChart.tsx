import { useEffect, useRef, useCallback } from 'react'
import {
  createChart,
  ColorType,
  CrosshairMode,
  type IChartApi,
  type ISeriesApi,
  type CandlestickData,
  type UTCTimestamp,
} from 'lightweight-charts'
import type { OHLCVCandle } from '@/types'
import { useThemeStore } from '@/stores'

interface CandlestickChartProps {
  candles: OHLCVCandle[]
  isLoading?: boolean
  className?: string
}

function toChartData(candle: OHLCVCandle): CandlestickData {
  return {
    time: candle.timestamp as UTCTimestamp,
    open: candle.open,
    high: candle.high,
    low: candle.low,
    close: candle.close,
  }
}

function getChartColors(theme: 'light' | 'dark') {
  return theme === 'dark'
    ? {
        background: '#0f1117',
        text: '#d1d5db',
        grid: '#1f2937',
        border: '#374151',
        upColor: '#22c55e',
        downColor: '#ef4444',
        wickUpColor: '#22c55e',
        wickDownColor: '#ef4444',
      }
    : {
        background: '#ffffff',
        text: '#374151',
        grid: '#f3f4f6',
        border: '#e5e7eb',
        upColor: '#16a34a',
        downColor: '#dc2626',
        wickUpColor: '#16a34a',
        wickDownColor: '#dc2626',
      }
}

export function CandlestickChart({ candles, isLoading = false, className = '' }: CandlestickChartProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const chartRef = useRef<IChartApi | null>(null)
  const seriesRef = useRef<ISeriesApi<'Candlestick'> | null>(null)
  const theme = useThemeStore((s) => s.theme)

  // Create chart on mount, destroy on unmount
  useEffect(() => {
    if (!containerRef.current) return
    const colors = getChartColors(theme)

    const chart = createChart(containerRef.current, {
      layout: {
        background: { type: ColorType.Solid, color: colors.background },
        textColor: colors.text,
      },
      grid: {
        vertLines: { color: colors.grid },
        horzLines: { color: colors.grid },
      },
      crosshair: { mode: CrosshairMode.Normal },
      rightPriceScale: { borderColor: colors.border },
      timeScale: {
        borderColor: colors.border,
        timeVisible: true,
        secondsVisible: false,
      },
      width: containerRef.current.clientWidth,
      height: containerRef.current.clientHeight,
    })

    const series = chart.addCandlestickSeries({
      upColor: colors.upColor,
      downColor: colors.downColor,
      borderVisible: false,
      wickUpColor: colors.wickUpColor,
      wickDownColor: colors.wickDownColor,
    })

    chartRef.current = chart
    seriesRef.current = series

    // Resize observer for responsive container
    const resizeObserver = new ResizeObserver((entries) => {
      if (entries.length === 0 || !chartRef.current) return
      const { width, height } = entries[0].contentRect
      chartRef.current.resize(width, height)
    })
    resizeObserver.observe(containerRef.current)

    return () => {
      resizeObserver.disconnect()
      chart.remove()
      chartRef.current = null
      seriesRef.current = null
    }
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  // Apply theme changes without recreating chart
  useEffect(() => {
    if (!chartRef.current || !seriesRef.current) return
    const colors = getChartColors(theme)
    chartRef.current.applyOptions({
      layout: {
        background: { type: ColorType.Solid, color: colors.background },
        textColor: colors.text,
      },
      grid: {
        vertLines: { color: colors.grid },
        horzLines: { color: colors.grid },
      },
      rightPriceScale: { borderColor: colors.border },
      timeScale: { borderColor: colors.border },
    })
    seriesRef.current.applyOptions({
      upColor: colors.upColor,
      downColor: colors.downColor,
      wickUpColor: colors.wickUpColor,
      wickDownColor: colors.wickDownColor,
    })
  }, [theme])

  // Update data without destroying/recreating chart
  useEffect(() => {
    if (!seriesRef.current || candles.length === 0) return
    const data = candles.map(toChartData)
    seriesRef.current.setData(data)
    chartRef.current?.timeScale().fitContent()
  }, [candles])

  return (
    <div className={`relative w-full h-full ${className}`}>
      <div ref={containerRef} className="w-full h-full" />

      {/* Loading overlay — shown on data change, no chart destroy */}
      {isLoading && (
        <div className="absolute inset-0 flex items-center justify-center bg-background/70 backdrop-blur-sm z-10">
          <div className="flex flex-col items-center gap-3">
            <div className="h-8 w-8 animate-spin rounded-full border-4 border-muted border-t-primary" />
            <span className="text-sm text-muted-foreground">Loading candles…</span>
          </div>
        </div>
      )}
    </div>
  )
}
