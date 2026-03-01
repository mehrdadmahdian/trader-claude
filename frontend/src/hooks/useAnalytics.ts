import { useQuery, useMutation } from '@tanstack/react-query'
import {
  startParamHeatmap,
  startMonteCarlo,
  startWalkForward,
  compareRuns,
  getAnalyticsJob,
} from '../api/analytics'
import type { CompareResult, AnalyticsJob } from '../types'

interface JobResponse {
  job_id: number
  status: string
}

// Poll a job until it completes (every 2s while pending/running)
export function useAnalyticsJob(jobId: number | null) {
  return useQuery({
    queryKey: ['analytics-job', jobId],
    queryFn: () => getAnalyticsJob(jobId!),
    enabled: jobId !== null,
    refetchInterval: (data) => {
      if (!data) return 2000
      const status = (data as { status?: string }).status
      return status === 'pending' || status === 'running' ? 2000 : false
    },
  })
}

// Start a param heatmap job — caller handles onSuccess to get job_id
export function useStartParamHeatmap() {
  return useMutation({
    mutationFn: ({
      runId,
      xParam,
      yParam,
      gridSize,
    }: {
      runId: number
      xParam: string
      yParam: string
      gridSize?: number
    }) => startParamHeatmap(runId, xParam, yParam, gridSize),
  })
}

// Start a Monte Carlo job
export function useStartMonteCarlo() {
  return useMutation({
    mutationFn: ({
      runId,
      numSimulations,
      ruinThreshold,
    }: {
      runId: number
      numSimulations?: number
      ruinThreshold?: number
    }) => startMonteCarlo(runId, numSimulations, ruinThreshold),
  })
}

// Start a walk-forward job
export function useStartWalkForward() {
  return useMutation({
    mutationFn: ({ runId, windows }: { runId: number; windows?: number }) =>
      startWalkForward(runId, windows),
  })
}

// Compare runs — synchronous query (only runs when runIds is non-empty)
export function useCompareRuns(runIds: number[]) {
  return useQuery<CompareResult>({
    queryKey: ['compare-runs', runIds],
    queryFn: () => compareRuns(runIds),
    enabled: runIds.length >= 2,
  })
}
