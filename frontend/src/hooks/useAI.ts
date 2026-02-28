import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { sendChat, getAISettings, saveAISettings, testAIConnection } from '../api/ai'
import type { AIChatRequest } from '../types'
import type { SaveAISettingsRequest } from '../api/ai'

// Fetch AI settings (cached, refetched on focus)
export function useAISettings() {
  return useQuery({
    queryKey: ['ai-settings'],
    queryFn: getAISettings,
  })
}

// Send a chat message — caller must handle onSuccess/onError
export function useSendChat() {
  return useMutation({
    mutationFn: (req: AIChatRequest) => sendChat(req),
  })
}

// Save AI settings, invalidate cache on success
export function useSaveAISettings() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (req: SaveAISettingsRequest) => saveAISettings(req),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['ai-settings'] })
    },
  })
}

// Test connection — no cache invalidation needed
export function useTestAIConnection() {
  return useMutation({
    mutationFn: (provider: 'openai' | 'ollama') => testAIConnection(provider),
  })
}
