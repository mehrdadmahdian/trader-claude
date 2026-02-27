import { useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { fetchAlerts, createAlert, deleteAlert, toggleAlert } from '@/api/alerts'
import { useAlertStore } from '@/stores'
import type { AlertCreateRequest } from '@/types'

export function useAlerts() {
  const setAlerts = useAlertStore((s) => s.setAlerts)

  const query = useQuery({
    queryKey: ['alerts'],
    queryFn: fetchAlerts,
  })

  useEffect(() => {
    if (query.data?.data) {
      setAlerts(query.data.data)
    }
  }, [query.data, setAlerts])

  return query
}

export function useCreateAlert() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (req: AlertCreateRequest) => createAlert(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['alerts'] }),
  })
}

export function useDeleteAlert() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => deleteAlert(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['alerts'] }),
  })
}

export function useToggleAlert() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => toggleAlert(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['alerts'] }),
  })
}
