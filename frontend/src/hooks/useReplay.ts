import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createReplaySession, createBookmark, listBookmarks, deleteBookmark } from '@/api/replay'
import type { CreateBookmarkRequest } from '@/types'

export function useCreateReplaySession() {
  return useMutation({
    mutationFn: (runId: number) => createReplaySession(runId),
  })
}

export function useReplayBookmarks(runId: number | null) {
  return useQuery({
    queryKey: ['replay-bookmarks', runId],
    queryFn: () => listBookmarks(runId!),
    enabled: runId != null,
  })
}

export function useCreateBookmark(runId: number | null) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (req: CreateBookmarkRequest) => createBookmark(req),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['replay-bookmarks', runId] })
    },
  })
}

export function useDeleteBookmark(runId: number | null) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => deleteBookmark(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['replay-bookmarks', runId] })
    },
  })
}
