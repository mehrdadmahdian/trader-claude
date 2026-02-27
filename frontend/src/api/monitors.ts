import { apiClient } from './client'
import type { Monitor, MonitorCreateRequest, MonitorSignalsResponse } from '@/types'

export async function fetchMonitors(): Promise<{ data: Monitor[] }> {
  const { data } = await apiClient.get<{ data: Monitor[] }>('/api/v1/monitors')
  return data
}

export async function fetchMonitor(id: number): Promise<Monitor> {
  const { data } = await apiClient.get<Monitor>(`/api/v1/monitors/${id}`)
  return data
}

export async function createMonitor(req: MonitorCreateRequest): Promise<Monitor> {
  const { data } = await apiClient.post<Monitor>('/api/v1/monitors', req)
  return data
}

export async function deleteMonitor(id: number): Promise<void> {
  await apiClient.delete(`/api/v1/monitors/${id}`)
}

export async function toggleMonitor(id: number): Promise<Monitor> {
  const { data } = await apiClient.patch<Monitor>(`/api/v1/monitors/${id}/toggle`)
  return data
}

export async function fetchMonitorSignals(
  id: number,
  params: { limit?: number; offset?: number } = {},
): Promise<MonitorSignalsResponse> {
  const query = new URLSearchParams()
  if (params.limit != null) query.set('limit', String(params.limit))
  if (params.offset != null) query.set('offset', String(params.offset))
  const { data } = await apiClient.get<MonitorSignalsResponse>(
    `/api/v1/monitors/${id}/signals?${query.toString()}`,
  )
  return data
}
