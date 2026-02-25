import apiClient from './client'
import type { IndicatorMeta, CalcResult, CalculateRequest } from '../types'

export async function fetchIndicators(): Promise<IndicatorMeta[]> {
  const { data } = await apiClient.get<{ indicators: IndicatorMeta[] }>('/indicators')
  return data.indicators
}

export async function calculateIndicator(req: CalculateRequest): Promise<CalcResult> {
  const { data } = await apiClient.post<CalcResult>('/indicators/calculate', req)
  return data
}
