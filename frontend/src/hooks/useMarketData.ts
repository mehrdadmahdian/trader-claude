import { useQuery } from '@tanstack/react-query'
import { apiClient } from '@/api/client'
import type { ApiResponse, MarketAdapter, MarketSymbol, OHLCVCandle } from '@/types'

// ── Query keys ──────────────────────────────────────────────────────────────

export const marketKeys = {
  all: ['markets'] as const,
  adapters: () => [...marketKeys.all, 'adapters'] as const,
  symbols: (adapterID: string, market?: string) =>
    [...marketKeys.all, 'symbols', adapterID, market ?? ''] as const,
  candles: (adapter: string, symbol: string, timeframe: string, from: string, to: string) =>
    [...marketKeys.all, 'candles', adapter, symbol, timeframe, from, to] as const,
  timeframes: () => [...marketKeys.all, 'timeframes'] as const,
}

// ── useMarkets ───────────────────────────────────────────────────────────────

export function useMarkets() {
  return useQuery({
    queryKey: marketKeys.adapters(),
    queryFn: async () => {
      const res = await apiClient.get<ApiResponse<MarketAdapter[]>>('/api/v1/markets')
      return res.data.data
    },
    staleTime: 60_000,
  })
}

// ── useSymbols ───────────────────────────────────────────────────────────────

export function useSymbols(adapterID: string, market?: string) {
  return useQuery({
    queryKey: marketKeys.symbols(adapterID, market),
    queryFn: async () => {
      const params = market ? { market } : {}
      const res = await apiClient.get<ApiResponse<MarketSymbol[]>>(
        `/api/v1/markets/${adapterID}/symbols`,
        { params },
      )
      return res.data.data
    },
    enabled: Boolean(adapterID),
    staleTime: 5 * 60_000,
  })
}

// ── useCandles ───────────────────────────────────────────────────────────────

interface UseCandlesParams {
  adapter: string
  symbol: string
  timeframe: string
  from: string // ISO8601
  to: string   // ISO8601
  market?: string
}

export function useCandles({ adapter, symbol, timeframe, from, to, market }: UseCandlesParams) {
  return useQuery({
    queryKey: marketKeys.candles(adapter, symbol, timeframe, from, to),
    queryFn: async () => {
      const params: Record<string, string> = { adapter, symbol, timeframe, from, to }
      if (market) params.market = market
      const res = await apiClient.get<ApiResponse<OHLCVCandle[]>>('/api/v1/candles', { params })
      return res.data.data
    },
    enabled: Boolean(adapter && symbol && timeframe && from && to),
    staleTime: 30_000,
  })
}

// ── useTimeframes ─────────────────────────────────────────────────────────────

export function useTimeframes() {
  return useQuery({
    queryKey: marketKeys.timeframes(),
    queryFn: async () => {
      const res = await apiClient.get<ApiResponse<string[]>>('/api/v1/candles/timeframes')
      return res.data.data
    },
    staleTime: Infinity,
  })
}
