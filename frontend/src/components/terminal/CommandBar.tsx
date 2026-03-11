import { useState, useRef, useEffect, useCallback } from 'react'
import { Terminal } from 'lucide-react'
import { cn } from '@/lib/utils'
import { FUNCTION_META, type FunctionCode, type CommandSuggestion } from '@/types/terminal'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import { useMarketStore } from '@/stores'
import type { Symbol as DBSymbol } from '@/types'

const FUNCTION_CODES = Object.keys(FUNCTION_META) as FunctionCode[]

export function CommandBar() {
  const [input, setInput] = useState('')
  const [suggestions, setSuggestions] = useState<CommandSuggestion[]>([])
  const [selectedIdx, setSelectedIdx] = useState(0)
  const inputRef = useRef<HTMLInputElement>(null)
  const addPanel = useWorkspaceStore((s) => s.addPanel)
  const symbols  = useMarketStore((s) => s.symbols) as DBSymbol[]

  const buildSuggestions = useCallback((value: string) => {
    const parts = value.trim().toUpperCase().split(/\s+/)
    const tickerPart = parts[0] ?? ''
    const fnPart     = parts[1]

    if (!tickerPart) { setSuggestions([]); return }

    // If we have both ticker and start of function code
    if (tickerPart && fnPart !== undefined) {
      const fnSuggestions = FUNCTION_CODES
        .filter((code) => code.startsWith(fnPart))
        .slice(0, 8)
        .map((code) => ({
          type: 'function' as const,
          value: `${tickerPart} ${code}`,
          label: code,
          description: FUNCTION_META[code].description,
        }))
      setSuggestions(fnSuggestions)
      return
    }

    // Ticker suggestions only
    const tickerSuggestions = symbols
      .filter((s) => s.ticker.toUpperCase().startsWith(tickerPart))
      .slice(0, 6)
      .map((s) => ({
        type: 'ticker' as const,
        value: s.ticker,
        label: s.ticker,
        description: s.description ?? '',
      }))
    setSuggestions(tickerSuggestions)
  }, [symbols])

  useEffect(() => {
    buildSuggestions(input)
    setSelectedIdx(0)
  }, [input, buildSuggestions])

  const execute = useCallback((command: string) => {
    const parts = command.trim().toUpperCase().split(/\s+/)
    const ticker = parts[0] ?? ''
    const fnCode = parts[1] as FunctionCode | undefined

    if (!ticker || !fnCode || !FUNCTION_META[fnCode]) return

    addPanel({ functionCode: fnCode, ticker, market: '', timeframe: '1h' })
    setInput('')
    setSuggestions([])
  }, [addPanel])

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'ArrowDown') { setSelectedIdx((i) => Math.min(i + 1, suggestions.length - 1)); e.preventDefault() }
    if (e.key === 'ArrowUp')   { setSelectedIdx((i) => Math.max(i - 1, 0)); e.preventDefault() }
    if (e.key === 'Enter') {
      const cmd = suggestions[selectedIdx]?.value ?? input
      execute(cmd)
      e.preventDefault()
    }
    if (e.key === 'Escape') { setSuggestions([]); setInput('') }
  }

  return (
    <div className="relative flex items-center gap-2 px-3 py-1.5 border-b border-border bg-background">
      <Terminal size={14} className="text-muted-foreground shrink-0" />

      <div className="flex-1 relative">
        <input
          ref={inputRef}
          className="w-full bg-transparent text-sm font-mono outline-none placeholder:text-muted-foreground/50"
          placeholder="BTC GP — type ticker + function (e.g. AAPL FA, BTC NEWS)"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={onKeyDown}
          spellCheck={false}
          autoComplete="off"
        />

        {suggestions.length > 0 && (
          <div className="absolute top-full left-0 z-50 mt-1 w-96 rounded-md border border-border bg-popover shadow-lg overflow-hidden">
            {suggestions.map((s, i) => (
              <div
                key={s.value}
                className={cn(
                  'flex items-center gap-3 px-3 py-2 cursor-pointer text-sm',
                  i === selectedIdx ? 'bg-accent text-accent-foreground' : 'hover:bg-accent/50',
                )}
                onClick={() => execute(s.value)}
              >
                <span className="font-mono font-semibold w-16 shrink-0">{s.label}</span>
                <span className="text-muted-foreground text-xs truncate">{s.description}</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
