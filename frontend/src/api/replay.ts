import apiClient from './client'
import type { CreateBookmarkRequest, ReplayBookmark } from '@/types'

export async function createReplaySession(runId: number): Promise<{ replay_id: string; total_candles: number }> {
  const res = await apiClient.post(`/api/v1/backtest/runs/${runId}/replay`)
  return res.data
}

export async function createBookmark(req: CreateBookmarkRequest): Promise<ReplayBookmark> {
  const res = await apiClient.post('/api/v1/replay/bookmarks', req)
  return res.data.data
}

export async function listBookmarks(runId: number): Promise<ReplayBookmark[]> {
  const res = await apiClient.get('/api/v1/replay/bookmarks', { params: { run_id: runId } })
  return res.data.data
}

export async function deleteBookmark(id: number): Promise<void> {
  await apiClient.delete(`/api/v1/replay/bookmarks/${id}`)
}
