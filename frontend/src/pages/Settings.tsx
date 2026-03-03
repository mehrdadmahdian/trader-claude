import { useState, useEffect } from 'react'
import {
  useNotificationSettings,
  useSaveNotificationSettings,
  useTestNotificationConnection,
} from '@/hooks/useSettings'
import {
  useAISettings,
  useSaveAISettings,
  useTestAIConnection,
} from '@/hooks/useAI'
import type { NotificationSettings, AISettings } from '@/types'
import type { SaveAISettingsRequest } from '@/api/ai'

export function Settings() {
  // Notification Settings
  const { data: settings, isLoading: notifLoading } = useNotificationSettings()
  const saveMutation = useSaveNotificationSettings()
  const testMutation = useTestNotificationConnection()

  // AI Settings
  const { data: aiSettings, isLoading: aiLoading } = useAISettings()
  const aiSaveMutation = useSaveAISettings()
  const aiTestMutation = useTestAIConnection()

  // Notification local state
  const [localSettings, setLocalSettings] = useState<NotificationSettings>({
    telegram: { bot_token: '', chat_id: '', enabled: false },
    webhook: { url: '', secret: '', enabled: false },
  })

  // AI local state
  const [localAISettings, setLocalAISettings] = useState<{
    provider: 'openai' | 'ollama'
    model: string
    apiKey: string
    ollamaUrl: string
  }>({
    provider: 'openai',
    model: 'gpt-4',
    apiKey: '',
    ollamaUrl: 'http://localhost:11434',
  })

  const [saveMessage, setSaveMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)
  const [testMessage, setTestMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)
  const [aiSaveMessage, setAISaveMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)
  const [aiTestMessage, setAITestMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)

  // Initialize notification settings from loaded data
  useEffect(() => {
    if (settings) {
      setLocalSettings(settings)
    }
  }, [settings])

  // Initialize AI settings from loaded data
  useEffect(() => {
    if (aiSettings) {
      setLocalAISettings({
        provider: aiSettings.provider,
        model: aiSettings.model,
        apiKey: '',
        ollamaUrl: aiSettings.ollama_url,
      })
    }
  }, [aiSettings])

  const handleSave = () => {
    saveMutation.mutate(localSettings, {
      onSuccess: () => {
        setSaveMessage({ type: 'success', text: 'Settings saved successfully!' })
        setTimeout(() => setSaveMessage(null), 3000)
      },
      onError: () => {
        setSaveMessage({ type: 'error', text: 'Failed to save settings.' })
      },
    })
  }

  const handleTestConnection = () => {
    setTestMessage(null)
    testMutation.mutate(undefined, {
      onSuccess: (result) => {
        const data = result as { ok?: boolean; bot_name?: string; error?: string }
        if (data.ok) {
          setTestMessage({ type: 'success', text: `Connection successful! Bot: ${data.bot_name || 'Unknown'}` })
        } else {
          setTestMessage({ type: 'error', text: `Connection failed: ${data.error || 'Unknown error'}` })
        }
        setTimeout(() => setTestMessage(null), 5000)
      },
      onError: () => {
        setTestMessage({ type: 'error', text: 'Test connection failed.' })
      },
    })
  }

  const handleAISave = () => {
    const req: SaveAISettingsRequest = {
      provider: localAISettings.provider,
      model: localAISettings.model,
      api_key: localAISettings.apiKey || undefined,
      ollama_url: localAISettings.ollamaUrl || undefined,
    }
    aiSaveMutation.mutate(req, {
      onSuccess: () => {
        setAISaveMessage({ type: 'success', text: 'AI settings saved successfully!' })
        setTimeout(() => setAISaveMessage(null), 3000)
      },
      onError: () => {
        setAISaveMessage({ type: 'error', text: 'Failed to save AI settings.' })
      },
    })
  }

  const handleAITestConnection = () => {
    setAITestMessage(null)
    aiTestMutation.mutate(localAISettings.provider, {
      onSuccess: (result) => {
        if (result.ok) {
          setAITestMessage({ type: 'success', text: 'Connection successful!' })
        } else {
          setAITestMessage({ type: 'error', text: `Connection failed: ${result.error || 'Unknown error'}` })
        }
        setTimeout(() => setAITestMessage(null), 5000)
      },
      onError: () => {
        setAITestMessage({ type: 'error', text: 'Test connection failed.' })
      },
    })
  }

  if (notifLoading || aiLoading) {
    return (
      <div className="space-y-6">
        <h1 className="text-xl font-semibold text-slate-900 tracking-tight text-gray-900 dark:text-white">Settings</h1>
        <p className="text-gray-600 dark:text-gray-400">Loading settings...</p>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-semibold text-slate-900 tracking-tight text-gray-900 dark:text-white">Settings</h1>

      {/* Notifications Section */}
      <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6 space-y-6">
        <h2 className="text-xl font-semibold text-gray-900 dark:text-white">Notifications</h2>

        {/* Telegram Subsection */}
        <div className="space-y-4 pb-6 border-b border-gray-200 dark:border-gray-700">
          <div className="flex items-center justify-between">
            <h3 className="text-lg font-medium text-gray-900 dark:text-white">Telegram</h3>
            <button
              onClick={() =>
                setLocalSettings({
                  ...localSettings,
                  telegram: { ...localSettings.telegram, enabled: !localSettings.telegram.enabled },
                })
              }
              className={`px-4 py-2 rounded-md font-medium text-sm transition-colors ${
                localSettings.telegram.enabled
                  ? 'bg-blue-500 text-white hover:bg-blue-600'
                  : 'bg-gray-300 dark:bg-gray-600 text-gray-900 dark:text-white hover:bg-gray-400 dark:hover:bg-gray-500'
              }`}
            >
              {localSettings.telegram.enabled ? 'Enabled' : 'Disabled'}
            </button>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Bot Token
            </label>
            <input
              type="password"
              value={localSettings.telegram.bot_token}
              onChange={(e) =>
                setLocalSettings({
                  ...localSettings,
                  telegram: { ...localSettings.telegram, bot_token: e.target.value },
                })
              }
              placeholder="xxx:yyy..."
              className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Chat ID
            </label>
            <input
              type="text"
              value={localSettings.telegram.chat_id}
              onChange={(e) =>
                setLocalSettings({
                  ...localSettings,
                  telegram: { ...localSettings.telegram, chat_id: e.target.value },
                })
              }
              placeholder="123456789"
              className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>
        </div>

        {/* Webhook Subsection */}
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <h3 className="text-lg font-medium text-gray-900 dark:text-white">Webhook</h3>
            <button
              onClick={() =>
                setLocalSettings({
                  ...localSettings,
                  webhook: { ...localSettings.webhook, enabled: !localSettings.webhook.enabled },
                })
              }
              className={`px-4 py-2 rounded-md font-medium text-sm transition-colors ${
                localSettings.webhook.enabled
                  ? 'bg-blue-500 text-white hover:bg-blue-600'
                  : 'bg-gray-300 dark:bg-gray-600 text-gray-900 dark:text-white hover:bg-gray-400 dark:hover:bg-gray-500'
              }`}
            >
              {localSettings.webhook.enabled ? 'Enabled' : 'Disabled'}
            </button>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Webhook URL
            </label>
            <input
              type="url"
              value={localSettings.webhook.url}
              onChange={(e) =>
                setLocalSettings({
                  ...localSettings,
                  webhook: { ...localSettings.webhook, url: e.target.value },
                })
              }
              placeholder="https://example.com/webhook"
              className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Secret (optional)
            </label>
            <input
              type="password"
              value={localSettings.webhook.secret}
              onChange={(e) =>
                setLocalSettings({
                  ...localSettings,
                  webhook: { ...localSettings.webhook, secret: e.target.value },
                })
              }
              placeholder="your-secret-key"
              className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>
        </div>

        {/* Action Buttons */}
        <div className="flex gap-3 pt-6 border-t border-gray-200 dark:border-gray-700">
          <button
            onClick={handleSave}
            disabled={saveMutation.isPending}
            className="px-6 py-2 bg-blue-500 text-white rounded-md font-medium hover:bg-blue-600 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            {saveMutation.isPending ? 'Saving...' : 'Save Settings'}
          </button>
          <button
            onClick={handleTestConnection}
            disabled={testMutation.isPending || (!localSettings.telegram.enabled && !localSettings.webhook.enabled)}
            className="px-6 py-2 bg-gray-300 dark:bg-gray-600 text-gray-900 dark:text-white rounded-md font-medium hover:bg-gray-400 dark:hover:bg-gray-500 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            {testMutation.isPending ? 'Testing...' : 'Test Connection'}
          </button>
        </div>

        {/* Feedback Messages */}
        {saveMessage && (
          <div
            className={`text-sm px-3 py-2 rounded-md ${
              saveMessage.type === 'success'
                ? 'bg-green-100 dark:bg-green-900 text-green-800 dark:text-green-200'
                : 'bg-red-100 dark:bg-red-900 text-red-800 dark:text-red-200'
            }`}
          >
            {saveMessage.text}
          </div>
        )}
        {testMessage && (
          <div
            className={`text-sm px-3 py-2 rounded-md ${
              testMessage.type === 'success'
                ? 'bg-green-100 dark:bg-green-900 text-green-800 dark:text-green-200'
                : 'bg-red-100 dark:bg-red-900 text-red-800 dark:text-red-200'
            }`}
          >
            {testMessage.text}
          </div>
        )}
      </div>

      {/* AI Assistant Section */}
      <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6 space-y-6">
        <h2 className="text-xl font-semibold text-gray-900 dark:text-white">AI Assistant</h2>

        {/* Provider Selector */}
        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
            Provider
          </label>
          <select
            value={localAISettings.provider}
            onChange={(e) =>
              setLocalAISettings({
                ...localAISettings,
                provider: e.target.value as 'openai' | 'ollama',
              })
            }
            className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
          >
            <option value="openai">OpenAI</option>
            <option value="ollama">Ollama</option>
          </select>
        </div>

        {/* API Key Input (OpenAI only) */}
        {localAISettings.provider === 'openai' && (
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              API Key
              {aiSettings?.has_api_key && !localAISettings.apiKey && (
                <span className="ml-2 text-xs text-green-600 dark:text-green-400">Key saved ✓</span>
              )}
            </label>
            <input
              type="password"
              value={localAISettings.apiKey}
              onChange={(e) =>
                setLocalAISettings({
                  ...localAISettings,
                  apiKey: e.target.value,
                })
              }
              placeholder={aiSettings?.has_api_key ? 'Leave empty to keep existing key' : 'sk-...'}
              className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>
        )}

        {/* Ollama URL Input (Ollama only) */}
        {localAISettings.provider === 'ollama' && (
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Ollama URL
            </label>
            <input
              type="url"
              value={localAISettings.ollamaUrl}
              onChange={(e) =>
                setLocalAISettings({
                  ...localAISettings,
                  ollamaUrl: e.target.value,
                })
              }
              placeholder="http://localhost:11434"
              className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>
        )}

        {/* Model Input */}
        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
            Model
          </label>
          <input
            type="text"
            value={localAISettings.model}
            onChange={(e) =>
              setLocalAISettings({
                ...localAISettings,
                model: e.target.value,
              })
            }
            placeholder={localAISettings.provider === 'openai' ? 'gpt-4' : 'llama2'}
            className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
        </div>

        {/* Action Buttons */}
        <div className="flex gap-3 pt-6 border-t border-gray-200 dark:border-gray-700">
          <button
            onClick={handleAISave}
            disabled={aiSaveMutation.isPending}
            className="px-6 py-2 bg-blue-500 text-white rounded-md font-medium hover:bg-blue-600 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            {aiSaveMutation.isPending ? 'Saving...' : 'Save Settings'}
          </button>
          <button
            onClick={handleAITestConnection}
            disabled={aiTestMutation.isPending}
            className="px-6 py-2 bg-gray-300 dark:bg-gray-600 text-gray-900 dark:text-white rounded-md font-medium hover:bg-gray-400 dark:hover:bg-gray-500 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            {aiTestMutation.isPending ? 'Testing...' : 'Test Connection'}
          </button>
        </div>

        {/* Feedback Messages */}
        {aiSaveMessage && (
          <div
            className={`text-sm px-3 py-2 rounded-md ${
              aiSaveMessage.type === 'success'
                ? 'bg-green-100 dark:bg-green-900 text-green-800 dark:text-green-200'
                : 'bg-red-100 dark:bg-red-900 text-red-800 dark:text-red-200'
            }`}
          >
            {aiSaveMessage.text}
          </div>
        )}
        {aiTestMessage && (
          <div
            className={`text-sm px-3 py-2 rounded-md ${
              aiTestMessage.type === 'success'
                ? 'bg-green-100 dark:bg-green-900 text-green-800 dark:text-green-200'
                : 'bg-red-100 dark:bg-red-900 text-red-800 dark:text-red-200'
            }`}
          >
            {aiTestMessage.text}
          </div>
        )}
      </div>
    </div>
  )
}
