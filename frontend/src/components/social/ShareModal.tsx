import { useState, useEffect } from 'react'
import { generateBacktestCard, sendTelegram } from '@/api/social'

interface ShareModalProps {
  backtestId: number
  onClose: () => void
}

export function ShareModal({ backtestId, onClose }: ShareModalProps) {
  const [theme, setTheme] = useState<'dark' | 'light'>('dark')
  const [cardUrl, setCardUrl] = useState<string | null>(null)
  const [cardBlob, setCardBlob] = useState<Blob | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [sending, setSending] = useState(false)
  const [sendResult, setSendResult] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    let url: string | null = null
    setLoading(true)
    setError(null)
    generateBacktestCard(backtestId, theme)
      .then((blob) => {
        url = URL.createObjectURL(blob)
        setCardBlob(blob)
        setCardUrl(url)
      })
      .catch(() => setError('Failed to generate card'))
      .finally(() => setLoading(false))
    return () => {
      if (url) URL.revokeObjectURL(url)
    }
  }, [backtestId, theme])

  function handleDownload() {
    if (!cardBlob) return
    const url = URL.createObjectURL(cardBlob)
    const a = document.createElement('a')
    a.href = url
    a.download = `backtest-${backtestId}-card.png`
    a.click()
    URL.revokeObjectURL(url)
  }

  async function handleSendTelegram() {
    if (!cardBlob) return
    setSending(true)
    setSendResult(null)
    try {
      const reader = new FileReader()
      const base64 = await new Promise<string>((resolve, reject) => {
        reader.onload = () => resolve((reader.result as string).split(',')[1])
        reader.onerror = reject
        reader.readAsDataURL(cardBlob)
      })
      await sendTelegram({ image_base64: base64, caption: `Backtest #${backtestId} results` })
      setSendResult('Sent to Telegram!')
    } catch {
      setSendResult('Failed to send')
    } finally {
      setSending(false)
    }
  }

  function handleCopyText() {
    navigator.clipboard.writeText(
      `Check out my backtest #${backtestId} results on trader-claude!`
    ).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60" onClick={onClose}>
      <div
        className="relative bg-white dark:bg-gray-800 rounded-xl shadow-2xl p-6 w-full max-w-2xl mx-4"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Share Backtest</h2>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 text-xl font-bold"
          >
            ×
          </button>
        </div>

        {/* Theme Toggle */}
        <div className="flex items-center gap-2 mb-4">
          <span className="text-sm text-gray-600 dark:text-gray-400">Theme:</span>
          {(['dark', 'light'] as const).map((t) => (
            <button
              key={t}
              onClick={() => setTheme(t)}
              className={`px-3 py-1 rounded text-sm font-medium transition-colors ${
                theme === t
                  ? 'bg-blue-600 text-white'
                  : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'
              }`}
            >
              {t === 'dark' ? '🌙 Dark' : '☀️ Light'}
            </button>
          ))}
        </div>

        {/* Card Preview */}
        <div className="rounded-lg overflow-hidden bg-gray-100 dark:bg-gray-700 mb-4 flex items-center justify-center min-h-[200px]">
          {loading && (
            <span className="text-gray-400 text-sm">Generating card...</span>
          )}
          {error && (
            <span className="text-red-500 text-sm">{error}</span>
          )}
          {cardUrl && !loading && (
            <img src={cardUrl} alt="Social card preview" className="w-full rounded-lg" />
          )}
        </div>

        {/* Actions */}
        <div className="flex flex-wrap gap-2">
          <button
            onClick={handleDownload}
            disabled={!cardBlob || loading}
            className="flex-1 px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white rounded-lg text-sm font-medium transition-colors"
          >
            Download PNG
          </button>
          <button
            onClick={handleSendTelegram}
            disabled={!cardBlob || loading || sending}
            className="flex-1 px-4 py-2 bg-green-600 hover:bg-green-700 disabled:opacity-50 text-white rounded-lg text-sm font-medium transition-colors"
          >
            {sending ? 'Sending...' : 'Send to Telegram'}
          </button>
          <button
            onClick={handleCopyText}
            disabled={loading}
            className="flex-1 px-4 py-2 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 text-gray-700 dark:text-gray-300 rounded-lg text-sm font-medium transition-colors"
          >
            {copied ? 'Copied!' : 'Copy Text'}
          </button>
        </div>

        {sendResult && (
          <p className={`mt-2 text-sm text-center ${sendResult.includes('Failed') ? 'text-red-500' : 'text-green-500'}`}>
            {sendResult}
          </p>
        )}
      </div>
    </div>
  )
}
