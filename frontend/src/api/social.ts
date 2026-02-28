import { apiClient } from './client'

export async function generateBacktestCard(runId: number, theme: 'dark' | 'light' = 'dark'): Promise<Blob> {
  const response = await apiClient.post(
    `/api/v1/social/backtest-card/${runId}?theme=${theme}`,
    {},
    { responseType: 'blob' }
  )
  return response.data
}

export async function generateSignalCard(signalId: number, theme: 'dark' | 'light' = 'dark'): Promise<Blob> {
  const response = await apiClient.post(
    `/api/v1/social/signal-card/${signalId}?theme=${theme}`,
    {},
    { responseType: 'blob' }
  )
  return response.data
}

export async function sendTelegram(payload: {
  chat_id?: string
  text?: string
  image_base64?: string
  caption?: string
}): Promise<{ success: boolean }> {
  const response = await apiClient.post('/api/v1/social/send-telegram', payload)
  return response.data
}
