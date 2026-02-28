import { apiClient } from './client'
import type { NotificationSettings, TelegramTestResult } from '@/types'

export async function getNotificationSettings(): Promise<NotificationSettings> {
  const response = await apiClient.get('/api/v1/settings/notifications')
  return response.data
}

export async function saveNotificationSettings(settings: NotificationSettings): Promise<{ saved: boolean }> {
  const response = await apiClient.post('/api/v1/settings/notifications', settings)
  return response.data
}

export async function testNotificationConnection(): Promise<{ telegram?: TelegramTestResult }> {
  const response = await apiClient.post('/api/v1/settings/notifications/test', {})
  return response.data
}
