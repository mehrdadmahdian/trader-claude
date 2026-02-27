import apiClient from './client'
import type { NewsItem } from '@/types'

export interface NewsFilters {
  symbol?: string
  limit?: number
  offset?: number
  from?: string
  to?: string
}

export interface NewsListResponse {
  data: NewsItem[]
  total: number
  page: number
  page_size: number
}

export async function fetchNews(filters: NewsFilters = {}): Promise<NewsListResponse> {
  const params = new URLSearchParams()
  if (filters.symbol) params.set('symbol', filters.symbol)
  if (filters.limit != null) params.set('limit', String(filters.limit))
  if (filters.offset != null) params.set('offset', String(filters.offset))
  if (filters.from) params.set('from', filters.from)
  if (filters.to) params.set('to', filters.to)
  const { data } = await apiClient.get<NewsListResponse>(`/api/v1/news?${params.toString()}`)
  return data
}

export async function fetchNewsBySymbol(
  symbol: string,
  limit = 10,
): Promise<{ data: NewsItem[] }> {
  const { data } = await apiClient.get<{ data: NewsItem[] }>(
    `/api/v1/news/symbols/${encodeURIComponent(symbol)}?limit=${limit}`,
  )
  return data
}
