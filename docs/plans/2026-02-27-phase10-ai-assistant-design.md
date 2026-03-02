# Phase 10 — AI Assistant Chatbot: Design Document

**Date:** 2026-02-27
**Status:** Draft

---

## Overview

Phase 10 adds a context-aware AI assistant chatbot that can answer questions about the user's current view (chart, backtest, portfolio, etc.). It supports two providers — OpenAI (cloud) and Ollama (local) — and surfaces as a floating panel on every page.

---

## Decisions Made

| Topic | Decision |
|---|---|
| Provider abstraction | `AIProvider` interface in `internal/ai/` — pluggable, tested via mock |
| Default model | `gpt-4o-mini` for OpenAI; configurable model for Ollama |
| Context injection | Dynamic system prompt built from `page_context` sent by the frontend |
| Streaming | Not in Phase 10 — full response returned; streaming deferred to a future enhancement |
| Suggested questions | 3 context-aware follow-up questions returned with every reply |
| Chat history | Client-side only (passed in `messages[]`); no server-side persistence |
| Rate limiting | 20 requests/minute per user_id (in-memory counter, resets each minute) |
| Settings storage | Reuses `settings` table from Phase 9 |
| Provider selection | Per-request — frontend sends `provider` field, backend routes to correct provider |
| Ollama network | Accessible as `http://ollama:11434` inside Docker network (optional container) |

---

## Section 1: Backend — `internal/ai/` Package

### File layout

```
internal/ai/
  provider.go        AIProvider interface + ChatMessage types
  openai.go          OpenAIProvider implementation
  openai_test.go     Mock HTTP tests
  ollama.go          OllamaProvider implementation
  ollama_test.go     Mock HTTP tests
  prompt.go          BuildSystemPrompt(pageContext) → string
  prompt_test.go     Tests for all page context types
```

### AIProvider Interface

```go
type Role string

const (
    RoleSystem    Role = "system"
    RoleUser      Role = "user"
    RoleAssistant Role = "assistant"
)

type ChatMessage struct {
    Role    Role   `json:"role"`
    Content string `json:"content"`
}

type ChatResponse struct {
    Reply              string   `json:"reply"`
    SuggestedQuestions []string `json:"suggested_questions"`
}

type AIProvider interface {
    Name() string
    Chat(ctx context.Context, messages []ChatMessage) (*ChatResponse, error)
    TestConnection(ctx context.Context) error
}
```

### OpenAIProvider

```go
type OpenAIProvider struct {
    apiKey     string
    model      string
    httpClient *http.Client
}

func NewOpenAIProvider(apiKey, model string) *OpenAIProvider
```

Calls `POST https://api.openai.com/v1/chat/completions` with:
- model: configurable (default `gpt-4o-mini`)
- messages: system prompt + user messages
- temperature: 0.7
- max_tokens: 1024

The system prompt instructs the model to:
1. Answer the user's question about their trading data
2. End every response with exactly 3 suggested follow-up questions in a JSON block

### OllamaProvider

```go
type OllamaProvider struct {
    baseURL string
    model   string
    httpClient *http.Client
}

func NewOllamaProvider(baseURL, model string) *OllamaProvider
```

Calls `POST {baseURL}/api/chat` with Ollama's chat format.

### System Prompt Builder

```go
func BuildSystemPrompt(ctx PageContext) string

type PageContext struct {
    Page       string                 `json:"page"`        // "chart", "backtest", "portfolio", "alerts", "monitor", "news"
    Symbol     string                 `json:"symbol,omitempty"`
    Timeframe  string                 `json:"timeframe,omitempty"`
    Indicators []string               `json:"indicators,omitempty"`
    Metrics    map[string]interface{} `json:"metrics,omitempty"`
    Positions  []PositionSummary      `json:"positions,omitempty"`
    Extra      map[string]interface{} `json:"extra,omitempty"`
}

type PositionSummary struct {
    Symbol string  `json:"symbol"`
    PnLPct float64 `json:"pnl_pct"`
}
```

Prompt template per page:
- **chart**: "User is viewing {symbol} {timeframe} chart with indicators: {indicators}. Current price: {price}."
- **backtest**: "User is viewing backtest results for {strategy} on {symbol}. Metrics: return={return}%, sharpe={sharpe}, max_dd={maxdd}%."
- **portfolio**: "User's portfolio has {N} positions worth ${total}. Top gainers: ... Top losers: ..."
- **monitor**: "User has {N} live monitors. Recent signals: ..."
- **news**: "User is viewing news for {symbol}. Recent headlines: ..."

---

## Section 2: AI API

### New file: `internal/api/ai_handler.go`

```
POST /api/v1/ai/chat               → ChatResponse
GET  /api/v1/settings/ai           → { provider, model, ollama_url, has_api_key }
POST /api/v1/settings/ai           → { saved: true }
POST /api/v1/settings/ai/test      → { ok, error? }
```

`POST /api/v1/ai/chat` body:
```json
{
  "messages": [
    {"role": "user", "content": "What does the RSI divergence mean?"}
  ],
  "page_context": {
    "page": "chart",
    "symbol": "BTCUSDT",
    "timeframe": "1h",
    "indicators": ["RSI(14)", "EMA(21)"]
  },
  "provider": "openai"
}
```

Response:
```json
{
  "reply": "RSI divergence occurs when...",
  "suggested_questions": [
    "Should I add Bollinger Bands for confirmation?",
    "What timeframe works best for RSI divergence?",
    "How does this compare to MACD divergence?"
  ]
}
```

Handler logic:
1. Load AI settings from DB (API key, model, Ollama URL)
2. Determine provider from request (or default from settings)
3. Build system prompt from `page_context`
4. Prepend system message to `messages[]`
5. Call provider.Chat()
6. Return response

---

## Section 3: Frontend

### New TypeScript types

```ts
export interface AIChatMessage {
  role: 'user' | 'assistant' | 'system'
  content: string
}

export interface AIChatRequest {
  messages: AIChatMessage[]
  page_context: AIPageContext
  provider?: 'openai' | 'ollama'
}

export interface AIChatResponse {
  reply: string
  suggested_questions: string[]
}

export interface AIPageContext {
  page: string
  symbol?: string
  timeframe?: string
  indicators?: string[]
  metrics?: Record<string, unknown>
  positions?: Array<{ symbol: string; pnl_pct: number }>
  extra?: Record<string, unknown>
}

export interface AISettings {
  provider: 'openai' | 'ollama'
  model: string
  ollama_url: string
  has_api_key: boolean
}
```

### Chat Panel Component

New component: `components/ai/ChatPanel.tsx`

Layout:
```
┌─────────────────────────────────────┐
│  AI Assistant  ·  chart context  [✕] │
│─────────────────────────────────────│
│                                     │
│  ┌─ assistant ──────────────────┐   │
│  │ RSI divergence occurs when   │   │
│  │ the price makes a new high   │   │
│  │ but RSI makes a lower high...│   │
│  └──────────────────────────────┘   │
│                                     │
│         ┌── user ──────────────┐    │
│         │ What about MACD?     │    │
│         └──────────────────────┘    │
│                                     │
│  ┌─ Suggested ─────────────────┐    │
│  │ [Add Bollinger Bands?]      │    │
│  │ [Best timeframe for RSI?]   │    │
│  │ [Compare to MACD?]          │    │
│  └─────────────────────────────┘    │
│                                     │
│  ┌──────────────────────┐ [Send]    │
│  │ Ask about your data...│          │
│  └──────────────────────┘           │
└─────────────────────────────────────┘
```

Features:
- Floating sparkle button (bottom-right, every page) → opens slide-up panel (~50% height)
- Header: "AI Assistant" + context chip (e.g., "chart: BTCUSDT 1h") + close button
- Message list: user right-aligned, assistant left-aligned, rendered with `react-markdown`
- Input: textarea + send button (Enter to send, Shift+Enter for newline)
- 3 suggested question chips below input (clickable)
- Typing indicator while waiting for response
- Context auto-captured from Zustand stores on panel open

### Context Capture Logic

```ts
function capturePageContext(): AIPageContext {
  const path = window.location.pathname
  if (path.includes('/chart')) {
    const { symbol, timeframe, activeIndicators } = useMarketStore.getState()
    return { page: 'chart', symbol, timeframe, indicators: activeIndicators.map(i => i.name) }
  }
  if (path.includes('/backtest')) {
    const { activeBacktest } = useBacktestStore.getState()
    return { page: 'backtest', metrics: activeBacktest?.metrics }
  }
  // ... similar for portfolio, monitor, etc.
  return { page: 'general' }
}
```

### New Files

```
frontend/src/api/ai.ts                    sendChat(), getAISettings(), saveAISettings(), testAIConnection()
frontend/src/hooks/useAI.ts               useSendChat mutation, useAISettings query
frontend/src/components/ai/ChatPanel.tsx   Slide-up chat panel
frontend/src/components/ai/AIButton.tsx    Floating sparkle button
```

### Settings Page "AI" Section

Add to `pages/Settings.tsx`:
- Provider selector (OpenAI / Ollama)
- API key input (masked, with "has key" indicator)
- Ollama URL input (shown when Ollama selected)
- Model input
- "Test Connection" button + result display

---

## Implementation Order

```
B1: Add AIProvider interface + ChatMessage types                   (no deps)
B2: OpenAIProvider + tests                                         (needs B1)
B3: OllamaProvider + tests                                        (needs B1, parallel with B2)
B4: prompt.go — BuildSystemPrompt + tests                          (no deps)
B5: api/ai_handler.go — chat endpoint                              (needs B1-B4)
B6: api/settings_handler.go — AI settings CRUD + test              (needs Phase 9 B1)
B7: Wire routes + main.go                                          (needs B5, B6)
F1: types + api/ai.ts                                              (no deps)
F2: hooks/useAI.ts                                                 (needs F1)
F3: components/ai/ChatPanel.tsx + AIButton.tsx                      (needs F2)
F4: Wire AIButton into Layout.tsx                                   (needs F3)
F5: Settings page AI section                                        (needs F2)
```

---

## Testing Requirements

| Task | Tests |
|---|---|
| B2 | OpenAI: mock HTTP, verify request format, parse response, handle errors |
| B3 | Ollama: mock HTTP, verify Ollama-format request, parse response |
| B4 | Prompt builder: all 6 page contexts produce correct system prompts |
| B5 | Chat endpoint: valid request → response with reply + 3 suggestions |
| B6 | AI settings: save/load/test round-trip |
| F3 | ChatPanel: renders messages, sends on Enter, shows suggestions |
| F4 | AIButton: click opens panel, Escape closes |
