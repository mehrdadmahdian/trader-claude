import { useEffect, useRef } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  fetchNotifications,
  markNotificationRead,
  markAllNotificationsRead,
} from '@/api/alerts'
import { useNotificationStore } from '@/stores'
import type { Notification } from '@/types'

export function useNotifications(page = 1, pageSize = 20) {
  const setNotifications = useNotificationStore((s) => s.setNotifications)

  const query = useQuery({
    queryKey: ['notifications', page, pageSize],
    queryFn: () => fetchNotifications({ limit: pageSize, offset: (page - 1) * pageSize }),
  })

  useEffect(() => {
    if (query.data?.data) {
      setNotifications(query.data.data)
    }
  }, [query.data, setNotifications])

  return query
}

export function useMarkRead() {
  const qc = useQueryClient()
  const markRead = useNotificationStore((s) => s.markRead)
  return useMutation({
    mutationFn: (id: number) => markNotificationRead(id),
    onSuccess: (_, id) => {
      markRead(id)
      qc.invalidateQueries({ queryKey: ['notifications'] })
    },
  })
}

export function useMarkAllRead() {
  const qc = useQueryClient()
  const markAllRead = useNotificationStore((s) => s.markAllRead)
  return useMutation({
    mutationFn: markAllNotificationsRead,
    onSuccess: () => {
      markAllRead()
      qc.invalidateQueries({ queryKey: ['notifications'] })
    },
  })
}

// useNotificationWS connects to /ws/notifications and adds incoming
// notifications to the Zustand store (which updates the unread badge).
export function useNotificationWS() {
  const addNotification = useNotificationStore((s) => s.addNotification)
  const wsRef = useRef<WebSocket | null>(null)

  useEffect(() => {
    const wsUrl = (import.meta.env.VITE_WS_URL ?? 'ws://localhost:8080') as string
    const ws = new WebSocket(`${wsUrl}/ws/notifications`)
    wsRef.current = ws

    ws.onmessage = (e: MessageEvent) => {
      try {
        const notif = JSON.parse(e.data as string) as Notification
        addNotification(notif)
      } catch {
        // ignore malformed messages
      }
    }

    ws.onerror = () => {
      // suppress console errors — server may not be running locally
    }

    return () => {
      ws.close()
    }
  }, [addNotification])
}
