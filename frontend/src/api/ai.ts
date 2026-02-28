import { apiClient } from './client'
import type { AIChatRequest, AIChatResponse, AISettings } from '../types'

export interface SaveAISettingsRequest {
  provider: 'openai' | 'ollama'
  model: string
  api_key?: string
  ollama_url?: string
}

export const sendChat = async (req: AIChatRequest): Promise<AIChatResponse> => {
  const { data } = await apiClient.post<AIChatResponse>('/ai/chat', req)
  return data
}

export const getAISettings = async (): Promise<AISettings> => {
  const { data } = await apiClient.get<AISettings>('/settings/ai')
  return data
}

export const saveAISettings = async (req: SaveAISettingsRequest): Promise<void> => {
  await apiClient.post('/settings/ai', req)
}

export const testAIConnection = async (provider: 'openai' | 'ollama'): Promise<{ ok: boolean; error?: string }> => {
  const { data } = await apiClient.post<{ ok: boolean; error?: string }>('/settings/ai/test', { provider })
  return data
}
