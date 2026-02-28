import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  getNotificationSettings,
  saveNotificationSettings,
  testNotificationConnection,
} from '@/api/settings'
import type { NotificationSettings } from '@/types'

const SETTINGS_KEY = ['notificationSettings']

export function useNotificationSettings() {
  return useQuery({
    queryKey: SETTINGS_KEY,
    queryFn: getNotificationSettings,
  })
}

export function useSaveNotificationSettings() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (settings: NotificationSettings) => saveNotificationSettings(settings),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: SETTINGS_KEY })
    },
  })
}

export function useTestNotificationConnection() {
  return useMutation({
    mutationFn: testNotificationConnection,
  })
}
