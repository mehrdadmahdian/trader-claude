import { useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  fetchMonitors,
  createMonitor,
  deleteMonitor,
  toggleMonitor,
  fetchMonitorSignals,
} from '@/api/monitors'
import { useMonitorStore } from '@/stores'
import type { MonitorCreateRequest } from '@/types'

export function useMonitors() {
  const setMonitors = useMonitorStore((s) => s.setMonitors)

  const query = useQuery({
    queryKey: ['monitors'],
    queryFn: fetchMonitors,
  })

  useEffect(() => {
    if (query.data?.data) {
      setMonitors(query.data.data)
    }
  }, [query.data, setMonitors])

  return query
}

export function useCreateMonitor() {
  const qc = useQueryClient()
  const addMonitor = useMonitorStore((s) => s.addMonitor)
  return useMutation({
    mutationFn: (req: MonitorCreateRequest) => createMonitor(req),
    onSuccess: (mon) => {
      addMonitor(mon)
      qc.invalidateQueries({ queryKey: ['monitors'] })
    },
  })
}

export function useDeleteMonitor() {
  const qc = useQueryClient()
  const removeMonitor = useMonitorStore((s) => s.removeMonitor)
  return useMutation({
    mutationFn: (id: number) => deleteMonitor(id),
    onSuccess: (_, id) => {
      removeMonitor(id)
      qc.invalidateQueries({ queryKey: ['monitors'] })
    },
  })
}

export function useToggleMonitor() {
  const qc = useQueryClient()
  const updateMonitor = useMonitorStore((s) => s.updateMonitor)
  return useMutation({
    mutationFn: (id: number) => toggleMonitor(id),
    onSuccess: (mon) => {
      updateMonitor(mon)
      qc.invalidateQueries({ queryKey: ['monitors'] })
    },
  })
}

export function useMonitorSignals(id: number, page = 1, pageSize = 20) {
  return useQuery({
    queryKey: ['monitor-signals', id, page, pageSize],
    queryFn: () => fetchMonitorSignals(id, { limit: pageSize, offset: (page - 1) * pageSize }),
    enabled: id > 0,
  })
}
