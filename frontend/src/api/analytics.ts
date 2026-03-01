import { apiClient } from './client'
import type {
  HeatmapResult,
  MonteCarloResult,
  WalkForwardResult,
  CompareResult,
  AnalyticsJob,
} from '../types'

export interface JobResponse {
  job_id: number
  status: string
}

export const startParamHeatmap = async (
  runId: number,
  xParam: string,
  yParam: string,
  gridSize = 10,
): Promise<JobResponse> => {
  const { data } = await apiClient.get<JobResponse>(
    `/backtest/runs/${runId}/param-heatmap`,
    { params: { x_param: xParam, y_param: yParam, grid_size: gridSize } },
  )
  return data
}

export const startMonteCarlo = async (
  runId: number,
  numSimulations = 1000,
  ruinThreshold = 0.5,
): Promise<JobResponse> => {
  const { data } = await apiClient.post<JobResponse>(
    `/backtest/runs/${runId}/monte-carlo`,
    { num_simulations: numSimulations, ruin_threshold: ruinThreshold },
  )
  return data
}

export const startWalkForward = async (
  runId: number,
  windows = 5,
): Promise<JobResponse> => {
  const { data } = await apiClient.get<JobResponse>(
    `/backtest/runs/${runId}/walk-forward`,
    { params: { windows } },
  )
  return data
}

export const compareRuns = async (runIds: number[]): Promise<CompareResult> => {
  const { data } = await apiClient.post<CompareResult>('/backtest/compare', {
    run_ids: runIds,
  })
  return data
}

export const getAnalyticsJob = async (jobId: number): Promise<AnalyticsJob> => {
  const { data } = await apiClient.get<AnalyticsJob>(`/analytics/jobs/${jobId}`)
  return data
}

export type { HeatmapResult, MonteCarloResult, WalkForwardResult, CompareResult, AnalyticsJob }
