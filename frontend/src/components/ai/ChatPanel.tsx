import { useState, useRef, useEffect, useCallback } from 'react'
import ReactMarkdown from 'react-markdown'
import { X, Send } from 'lucide-react'
import { useSendChat } from '../../hooks/useAI'
import { useMarketStore, useBacktestStore, usePortfolioStore } from '../../stores'
import type { AIChatMessage, AIPageContext } from '../../types'

interface ChatPanelProps {
  isOpen: boolean
  onClose: () => void
}

function getPageFromPath(path: string): string {
  if (path.includes('/chart')) return 'chart'
  if (path.includes('/backtest')) return 'backtest'
  if (path.includes('/portfolio')) return 'portfolio'
  if (path.includes('/monitor')) return 'monitor'
  if (path.includes('/alerts')) return 'alerts'
  if (path.includes('/news')) return 'news'
  return 'dashboard'
}

function TypingIndicator() {
  return (
    <div className="flex items-center gap-1 px-3 py-2">
      <span className="w-2 h-2 rounded-full bg-violet-400 animate-bounce [animation-delay:0ms]" />
      <span className="w-2 h-2 rounded-full bg-violet-400 animate-bounce [animation-delay:150ms]" />
      <span className="w-2 h-2 rounded-full bg-violet-400 animate-bounce [animation-delay:300ms]" />
    </div>
  )
}

export function ChatPanel({ isOpen, onClose }: ChatPanelProps) {
  const [messages, setMessages] = useState<AIChatMessage[]>([])
  const [input, setInput] = useState('')
  const [suggestions, setSuggestions] = useState<string[]>([])
  const [pageContext, setPageContext] = useState<AIPageContext>({ page: 'dashboard' })
  const messagesEndRef = useRef<HTMLDivElement>(null)

  const market = useMarketStore()
  const backtest = useBacktestStore()
  const portfolio = usePortfolioStore()

  const sendChat = useSendChat()

  // Capture context when panel opens
  useEffect(() => {
    if (!isOpen) return
    const page = getPageFromPath(window.location.pathname)
    const ctx: AIPageContext = {
      page,
      symbol: market.selectedSymbol ?? undefined,
      timeframe: market.selectedTimeframe,
    }
    if (page === 'backtest' && backtest.activeBacktest) {
      ctx.metrics = (backtest.activeBacktest.metrics as unknown as Record<string, unknown>) ?? {}
    }
    if (page === 'portfolio' && portfolio.positions.length > 0) {
      ctx.positions = portfolio.positions.map((p) => ({
        symbol: p.symbol,
        pnl_pct: p.unrealized_pnl_pct,
      }))
    }
    setPageContext(ctx)
  }, [isOpen]) // only recapture on open

  // Scroll to bottom when messages change
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages, sendChat.isPending])

  const sendMessage = useCallback(
    (text: string) => {
      const trimmed = text.trim()
      if (!trimmed || sendChat.isPending) return

      const userMsg: AIChatMessage = { role: 'user', content: trimmed }
      const newMessages = [...messages, userMsg]
      setMessages(newMessages)
      setInput('')
      setSuggestions([])

      sendChat.mutate(
        { messages: newMessages, page_context: pageContext },
        {
          onSuccess: (resp) => {
            setMessages((prev) => [
              ...prev,
              { role: 'assistant', content: resp.reply },
            ])
            setSuggestions(resp.suggested_questions ?? [])
          },
          onError: () => {
            setMessages((prev) => [
              ...prev,
              {
                role: 'assistant',
                content:
                  'Sorry, I encountered an error. Please try again.',
              },
            ])
          },
        }
      )
    },
    [messages, pageContext, sendChat]
  )

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      sendMessage(input)
    }
  }

  if (!isOpen) return null

  return (
    <div className="fixed bottom-0 left-0 right-0 z-40 flex justify-end px-4 pb-20 pointer-events-none">
      <div
        className="pointer-events-auto w-full max-w-lg bg-white dark:bg-zinc-900 border border-zinc-200 dark:border-zinc-700 rounded-t-2xl shadow-2xl flex flex-col"
        style={{ height: '50vh' }}
      >
        {/* Header */}
        <div className="flex items-center justify-between px-4 py-3 border-b border-zinc-200 dark:border-zinc-700">
          <div className="flex items-center gap-2">
            <span className="font-semibold text-sm text-zinc-900 dark:text-zinc-100">
              AI Assistant
            </span>
            <span className="text-xs px-2 py-0.5 rounded-full bg-violet-100 dark:bg-violet-900/40 text-violet-700 dark:text-violet-300">
              {pageContext.page}
            </span>
          </div>
          <button
            onClick={onClose}
            className="text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-200 transition-colors"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        {/* Messages */}
        <div className="flex-1 overflow-y-auto px-4 py-3 space-y-3">
          {messages.length === 0 && (
            <p className="text-sm text-zinc-400 dark:text-zinc-500 text-center mt-8">
              Ask me anything about your trading data.
            </p>
          )}
          {messages.map((msg, i) => (
            <div
              key={i}
              className={`flex ${
                msg.role === 'user' ? 'justify-end' : 'justify-start'
              }`}
            >
              <div
                className={`max-w-[85%] rounded-2xl px-3 py-2 text-sm ${
                  msg.role === 'user'
                    ? 'bg-violet-600 text-white rounded-br-sm'
                    : 'bg-zinc-100 dark:bg-zinc-800 text-zinc-900 dark:text-zinc-100 rounded-bl-sm'
                }`}
              >
                {msg.role === 'assistant' ? (
                  <div className="prose prose-sm dark:prose-invert max-w-none">
                    <ReactMarkdown>{msg.content}</ReactMarkdown>
                  </div>
                ) : (
                  msg.content
                )}
              </div>
            </div>
          ))}
          {sendChat.isPending && (
            <div className="flex justify-start">
              <div className="bg-zinc-100 dark:bg-zinc-800 rounded-2xl rounded-bl-sm">
                <TypingIndicator />
              </div>
            </div>
          )}
          <div ref={messagesEndRef} />
        </div>

        {/* Suggestion chips */}
        {suggestions.length > 0 && (
          <div className="px-4 pb-2 flex flex-wrap gap-2">
            {suggestions.map((s, i) => (
              <button
                key={i}
                onClick={() => sendMessage(s)}
                className="text-xs px-3 py-1 rounded-full border border-violet-300 dark:border-violet-700 text-violet-700 dark:text-violet-300 hover:bg-violet-50 dark:hover:bg-violet-900/30 transition-colors"
              >
                {s}
              </button>
            ))}
          </div>
        )}

        {/* Input */}
        <div className="px-4 py-3 border-t border-zinc-200 dark:border-zinc-700 flex items-end gap-2">
          <textarea
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Ask about your trades... (Enter to send)"
            rows={1}
            className="flex-1 resize-none bg-zinc-100 dark:bg-zinc-800 text-zinc-900 dark:text-zinc-100 placeholder-zinc-400 rounded-xl px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-violet-500"
          />
          <button
            onClick={() => sendMessage(input)}
            disabled={!input.trim() || sendChat.isPending}
            className="flex-shrink-0 w-8 h-8 flex items-center justify-center rounded-xl bg-violet-600 hover:bg-violet-700 disabled:bg-zinc-300 dark:disabled:bg-zinc-700 text-white transition-colors"
          >
            <Send className="h-3.5 w-3.5" />
          </button>
        </div>
      </div>
    </div>
  )
}
