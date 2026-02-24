import { useEffect, useRef } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiClient } from '@/api/client'
import type {
  ApiResponse,
  Backtest,
  BacktestMetrics,
  BacktestRunRequest,
  EquityPoint,
  StrategyInfo,
  Trade,
} from '@/types'

// ── Query key factory ────────────────────────────────────────────────────────

export const backtestKeys = {
  all: ['backtests'] as const,
  runs: () => [...backtestKeys.all, 'runs'] as const,
  run: (id: number) => [...backtestKeys.all, 'runs', id] as const,
  strategies: () => [...backtestKeys.all, 'strategies'] as const,
}

// ── useStrategies ────────────────────────────────────────────────────────────

export function useStrategies() {
  return useQuery({
    queryKey: backtestKeys.strategies(),
    queryFn: async () => {
      const res = await apiClient.get<ApiResponse<StrategyInfo[]>>('/api/v1/strategies')
      return res.data.data
    },
    staleTime: 5 * 60_000,
  })
}

// ── useBacktestRuns ──────────────────────────────────────────────────────────

export function useBacktestRuns() {
  return useQuery({
    queryKey: backtestKeys.runs(),
    queryFn: async () => {
      const res = await apiClient.get<ApiResponse<Backtest[]>>('/api/v1/backtest/runs')
      return res.data.data
    },
    staleTime: 10_000,
  })
}

// ── useBacktestRun ───────────────────────────────────────────────────────────

export interface BacktestRunDetail {
  backtest: Backtest
  trades: Trade[]
  equity_curve: EquityPoint[]
}

export function useBacktestRun(id: number | null) {
  return useQuery({
    queryKey: id != null ? backtestKeys.run(id) : backtestKeys.all,
    queryFn: async () => {
      const res = await apiClient.get<BacktestRunDetail>(`/api/v1/backtest/runs/${id}`)
      return res.data
    },
    enabled: id != null,
    staleTime: 5_000,
  })
}

// ── useRunBacktest ───────────────────────────────────────────────────────────

interface RunBacktestResponse {
  run_id: number
  status: string
}

export function useRunBacktest() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (req: BacktestRunRequest) => {
      const res = await apiClient.post<RunBacktestResponse>('/api/v1/backtest/run', req)
      return res.data
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: backtestKeys.runs() })
    },
  })
}

// ── useBacktestProgress ──────────────────────────────────────────────────────

export function useBacktestProgress(
  runId: number | null,
  onProgress: (p: number) => void,
  onDone: (metrics: BacktestMetrics) => void,
) {
  const wsRef = useRef<WebSocket | null>(null)
  const onProgressRef = useRef(onProgress)
  const onDoneRef = useRef(onDone)

  // Keep refs up to date without re-running the effect
  useEffect(() => {
    onProgressRef.current = onProgress
  }, [onProgress])

  useEffect(() => {
    onDoneRef.current = onDone
  }, [onDone])

  useEffect(() => {
    if (runId == null) return

    const wsBase = import.meta.env.VITE_WS_URL ?? 'ws://localhost:8080'
    const url = `${wsBase}/ws/backtest/${runId}/progress`
    const ws = new WebSocket(url)
    wsRef.current = ws

    ws.onmessage = (event: MessageEvent) => {
      try {
        const msg = JSON.parse(event.data as string) as {
          progress?: number
          done?: boolean
          metrics?: BacktestMetrics
        }
        if (typeof msg.progress === 'number') {
          onProgressRef.current(msg.progress)
        }
        if (msg.done) {
          if (msg.metrics) {
            onDoneRef.current(msg.metrics)
          }
          ws.close()
        }
      } catch {
        // ignore parse errors
      }
    }

    ws.onerror = () => {
      ws.close()
    }

    return () => {
      ws.close()
      wsRef.current = null
    }
  }, [runId])
}
