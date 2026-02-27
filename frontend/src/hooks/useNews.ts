import { useQuery } from '@tanstack/react-query'
import { fetchNews, fetchNewsBySymbol, type NewsFilters } from '@/api/news'

const NEWS_STALE_MS = 5 * 60 * 1000 // 5 minutes

export function useNews(filters: NewsFilters = {}) {
  return useQuery({
    queryKey: ['news', filters],
    queryFn: () => fetchNews(filters),
    staleTime: NEWS_STALE_MS,
  })
}

export function useNewsBySymbol(symbol: string | null, limit = 10) {
  return useQuery({
    queryKey: ['news', 'symbol', symbol, limit],
    queryFn: () => fetchNewsBySymbol(symbol!, limit),
    enabled: !!symbol,
    staleTime: NEWS_STALE_MS,
  })
}
