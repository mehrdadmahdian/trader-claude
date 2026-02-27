import apiClient from './client'
import type { Alert, AlertCreateRequest, Notification } from '@/types'

// --- Alerts ---

export async function fetchAlerts(): Promise<{ data: Alert[] }> {
  const { data } = await apiClient.get<{ data: Alert[] }>('/api/v1/alerts')
  return data
}

export async function createAlert(req: AlertCreateRequest): Promise<{ data: Alert }> {
  const { data } = await apiClient.post<{ data: Alert }>('/api/v1/alerts', req)
  return data
}

export async function deleteAlert(id: number): Promise<void> {
  await apiClient.delete(`/api/v1/alerts/${id}`)
}

export async function toggleAlert(id: number): Promise<{ data: Alert }> {
  const { data } = await apiClient.patch<{ data: Alert }>(`/api/v1/alerts/${id}/toggle`)
  return data
}

// --- Notifications ---

export interface NotificationsParams {
  limit?: number
  offset?: number
}

export interface NotificationsResponse {
  data: Notification[]
  total: number
  page: number
  page_size: number
}

export async function fetchNotifications(
  params: NotificationsParams = {},
): Promise<NotificationsResponse> {
  const query = new URLSearchParams()
  if (params.limit != null) query.set('limit', String(params.limit))
  if (params.offset != null) query.set('offset', String(params.offset))
  const { data } = await apiClient.get<NotificationsResponse>(
    `/api/v1/notifications?${query.toString()}`,
  )
  return data
}

export async function markNotificationRead(id: number): Promise<void> {
  await apiClient.patch(`/api/v1/notifications/${id}/read`)
}

export async function markAllNotificationsRead(): Promise<void> {
  await apiClient.post('/api/v1/notifications/read-all')
}

export async function fetchUnreadCount(): Promise<number> {
  const { data } = await apiClient.get<{ count: number }>('/api/v1/notifications/unread-count')
  return data.count
}
