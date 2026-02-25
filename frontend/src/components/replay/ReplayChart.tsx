import { useEffect, useRef } from 'react'
import {
  createChart,
  ColorType,
  CrosshairMode,
  type IChartApi,
  type ISeriesApi,
  type CandlestickData,
  type UTCTimestamp,
  type SeriesMarker,
  type Time,
} from 'lightweight-charts'
import { useBacktestStore } from '@/stores'
import { useThemeStore } from '@/stores'
import type { ReplayCandle } from '@/types'

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

function toChartData(candle: ReplayCandle): CandlestickData {
  return {
    time: (new Date(candle.timestamp).getTime() / 1000) as UTCTimestamp,
    open: candle.open,
    high: candle.high,
    low: candle.low,
    close: candle.close,
  }
}

export function ReplayChart() {
  const containerRef = useRef<HTMLDivElement>(null)
  const chartRef = useRef<IChartApi | null>(null)
  const seriesRef = useRef<ISeriesApi<'Candlestick'> | null>(null)
  const prevCandleCountRef = useRef(0)

  const theme = useThemeStore((s) => s.theme)
  const replayCandles = useBacktestStore((s) => s.replayCandles)
  const replayTrades = useBacktestStore((s) => s.replayTrades)

  // Create chart on mount
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
    prevCandleCountRef.current = 0

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
      prevCandleCountRef.current = 0
    }
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  // Theme changes
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

  // Append-only candle updates — avoid full setData on each tick
  useEffect(() => {
    if (!seriesRef.current || replayCandles.length === 0) return

    const prev = prevCandleCountRef.current

    if (replayCandles.length < prev) {
      // Seek happened — full reset
      const data = replayCandles.map(toChartData)
      seriesRef.current.setData(data)
      chartRef.current?.timeScale().fitContent()
    } else {
      // Append new candles only
      const newCandles = replayCandles.slice(prev)
      for (const candle of newCandles) {
        seriesRef.current.update(toChartData(candle))
      }
      chartRef.current?.timeScale().scrollToRealTime()
    }

    prevCandleCountRef.current = replayCandles.length
  }, [replayCandles])

  // Trade markers
  useEffect(() => {
    if (!seriesRef.current) return
    const markers: SeriesMarker<Time>[] = []
    for (const trade of replayTrades) {
      const entryTs = Math.floor(new Date(trade.entry_time).getTime() / 1000) as Time
      markers.push({
        time: entryTs,
        position: 'belowBar',
        color: '#22c55e',
        shape: 'arrowUp',
        text: 'Buy',
      })
      if (trade.exit_time) {
        const exitTs = Math.floor(new Date(trade.exit_time).getTime() / 1000) as Time
        markers.push({
          time: exitTs,
          position: 'aboveBar',
          color: '#ef4444',
          shape: 'arrowDown',
          text: 'Sell',
        })
      }
    }
    markers.sort((a, b) => (a.time as number) - (b.time as number))
    seriesRef.current.setMarkers(markers)
  }, [replayTrades])

  return <div ref={containerRef} className="w-full h-full" />
}
