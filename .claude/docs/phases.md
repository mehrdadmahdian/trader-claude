# Implementation Phases

> Legend: ✅ Done · 🔲 Todo · 🚧 In Progress
>
> Sub-phases are independently completable in one focused session.
> Dependencies noted inline. Backend and frontend separated for parallel work.

---

## Phase 1 — Project Scaffold & Infrastructure ✅ COMPLETE

### 1.1 — Backend Infrastructure ✅
- [x] Config loader (`godotenv` + `Config` struct, all env vars + defaults)
- [x] Fiber app with CORS, request logger, error handler, graceful shutdown
- [x] `GET /health` → `{status, db, redis, version}`
- [x] MySQL via GORM with retry logic (10 attempts, 3s intervals) + AutoMigrate
- [x] Redis via go-redis, ping on startup
- [x] `AdapterRegistry` + `StrategyRegistry` singletons
- [x] `MarketAdapter` + `Strategy` interfaces (`internal/registry/interfaces.go`)
- [x] Worker pool (configurable concurrency + panic recovery)
- [x] WebSocket hub singleton with channel subscriptions
- [x] All stub API routes registered

### 1.2 — Frontend Shell ✅
- [x] React 18 + TypeScript + Vite 5, TailwindCSS v3 + shadcn/ui
- [x] React Router v6 with all 9 page routes
- [x] Zustand stores: theme, sidebar, market, backtest, portfolio, alert, notification
- [x] Axios instance (`api/client.ts`), TanStack Query provider
- [x] Layout: collapsible sidebar, top bar (theme toggle + notification bell)
- [x] Dark/light mode (`localStorage: trader-theme`)
- [x] All TypeScript interfaces (`types/index.ts`), 9 stub pages

### 1.3 — Infrastructure ✅
- [x] Docker Compose: MySQL 8.0, Redis 7, Go backend, React frontend
- [x] Health checks, Air hot-reload, Vite HMR
- [x] Makefile with all dev targets, `.env.example`

---

## Phase 2 — Market Data Layer ✅ COMPLETE

### 2.1 — Binance Adapter ✅
- [x] `internal/adapter/binance.go` — `FetchOHLCV`, `FetchSymbols`, `StreamTicker`, rate-limit + backoff
- [x] Registered in `main.go`
- [x] Unit tests (mock HTTP + WS) — all passing

### 2.2 — Yahoo Finance Adapter ✅
- [x] `internal/adapter/yahoo.go` — `FetchOHLCV`, `FetchSymbols`, hardcoded symbol list
- [x] Registered in `main.go`
- [x] Unit tests (mock HTTP) — all passing

### 2.3 — Data Service ✅
- [x] `internal/adapter/dataservice.go` — `GetCandles` (gap-filling), `SyncRecent`, background sync worker
- [x] Unit tests: gap-detection, merge/dedup — all passing

### 2.4 — Market API Endpoints ✅
- [x] `GET /api/v1/markets`, `GET /api/v1/markets/:adapterID/symbols`
- [x] `GET /api/v1/candles?adapter=&symbol=&timeframe=&from=&to=`
- [x] `GET /api/v1/candles/timeframes`

### 2.5 — Chart Component + Page ✅
- [x] `<CandlestickChart>` (lightweight-charts, full lifecycle management)
- [x] React Query hooks: `useCandles`, `useSymbols`, `useMarkets`
- [x] Chart page: adapter selector, symbol search, timeframe buttons, loading/error states

---

## Phase 3 — Strategy Engine & Backtesting ✅ COMPLETE

### 3.1 — Strategy Implementations ✅
- [x] `internal/strategy/ema_crossover.go` — params: fast_period, slow_period, signal_on_close
- [x] `internal/strategy/rsi.go` — params: period, overbought, oversold, use_divergence
- [x] `internal/strategy/macd.go` — params: fast, slow, signal, histogram_threshold
- [x] All strategies registered in `main.go`

### 3.2 — Backtest Engine ✅
- [x] `internal/backtest/engine.go` — iterate candles one-by-one (no look-ahead)
- [x] Track open/closed positions, commission (0.1%) + slippage (0.05%)
- [x] Full trade record: entry/exit price, time, PnL, PnL%
- [x] Equity curve: `{timestamp, value}` per candle
- [x] Metrics: total/annualized return, Sharpe, Sortino, max drawdown, win rate, profit factor, avg win/loss, trade counts, largest win/loss
- [x] Progress published to Redis `backtest:<run_id>:progress`
- [x] Runs inside worker pool goroutine

### 3.3 — Backtest API Endpoints ✅
- [x] `GET /api/v1/strategies` — list all with param schemas
- [x] `GET /api/v1/strategies/:id`
- [x] `POST /api/v1/backtest/run` — async, returns run_id immediately
- [x] `GET /api/v1/backtest/runs` — list past runs
- [x] `GET /api/v1/backtest/runs/:id` — full result (equity curve + metrics + trades)
- [x] `DELETE /api/v1/backtest/runs/:id`
- [x] `WS /ws/backtest/:id/progress` — stream 0–100% + final result

### 3.4 — Backtest Frontend — Config Panel ✅
- [x] Two-column layout (30/70 split)
- [x] Strategy card grid selector
- [x] Auto-generated param form (slider+number for int/float, toggle for bool, segmented for enum, tooltips)
- [x] Symbol/adapter/timeframe/date-range/capital/commission inputs
- [x] "Run Backtest" button (disabled while running) + progress bar

### 3.5 — Backtest Frontend — Results UI ✅
- [x] Overview tab: metric stat cards + equity area chart
- [x] Trades tab: sortable table, click trade highlights equity curve
- [x] Chart tab: candlestick with buy/sell markers overlaid

### 3.6 — Backtest Tests ✅
- [x] EMA/RSI/MACD unit tests — all passing
- [x] Backtest engine unit tests — all passing
- [x] Backtest API integration tests — all passing

---

## Phase 4 — Slow-Motion Replay Engine 🔲

### 4.1 — Replay Backend 🔲
*Requires Phase 3 complete.*
- [ ] `POST /api/v1/backtest/runs/:id/replay` → returns `replay_id`
- [ ] `WS /ws/replay/:replay_id`
  - [ ] Control messages: start, pause, resume, step, set_speed (0.25x–10x), seek
  - [ ] Emit: candle, signal, trade_open, trade_close, equity_update, status
  - [ ] Speed via `time.Sleep` (1x = 300ms)
- [ ] Tests: session lifecycle, speed timing, WS message sequence

### 4.2 — Replay Frontend 🔲
*Requires 4.1.*
- [ ] "Replay" button on completed backtest results
- [ ] Chart builds up candle-by-candle; buy/sell markers appear at signal candles
- [ ] Control bar: Play/Pause, Step Forward, speed selector, seekable progress bar, timestamp display
- [ ] Live equity mini-chart (bottom-right corner)
- [ ] Signal toast ("BUY signal at $42,150") per trade signal

---

## Phase 5 — Technical Indicators on Chart ✅

### 5.1 — Overlay Indicator Calculations ✅
*Backend. No dependencies.*
- [x] `internal/indicator/` — pure Go
- [x] SMA, EMA, WMA, Bollinger Bands, VWAP, Parabolic SAR, Ichimoku Cloud
- [x] Unit tests (known inputs → expected outputs)

### 5.2 — Panel Indicator Calculations ✅
*Backend. No dependencies (parallel with 5.1).*
- [x] RSI, MACD (line + signal + histogram), Stochastic (%K/%D), ATR, OBV, Volume (colored)
- [x] Unit tests

### 5.3 — Indicators API ✅
*Backend. Requires 5.1 + 5.2.*
- [x] `GET /api/v1/indicators` — metadata + param schemas (grouped by type)
- [x] `POST /api/v1/indicators/calculate` — `{indicator_id, params, candles}` → time-series arrays
- [x] API integration tests

### 5.4 — Frontend Indicator Modal + Toolbar ✅
*Frontend. Requires 5.3.*
- [x] "Indicators" button → searchable modal (grouped: Trend, Momentum, Volatility, Volume)
- [x] Auto-generated param config form per indicator
- [x] Active indicator chips in toolbar (click to edit/remove)
- [x] Persist active indicators + params to `localStorage`

### 5.5 — Frontend Chart Rendering ✅
*Frontend. Requires 5.4.*
- [x] Overlay indicators as line series on main chart pane
- [x] Bollinger Bands: 3 lines; Ichimoku: 5 lines (cloud fill deferred — plugin required)
- [x] Panel indicators in separate panes below chart (header + close button)
- [x] Re-calculate on candle load and on indicator add
- [x] localStorage persistence per symbol:timeframe

---

## Phase 6 — Portfolio Tracker ✅

### 6.1 — Portfolio Service + CRUD API ✅
*Backend. No dependencies.*
- [x] `internal/portfolio/` — CRUD for portfolios + positions
- [x] `POST/GET/PUT/DELETE /api/v1/portfolios`
- [x] `GET /api/v1/portfolios/:id` (with positions)
- [x] `POST/PUT/DELETE /api/v1/portfolios/:id/positions`
- [x] `POST/GET /api/v1/portfolios/:id/transactions` (paginated)

### 6.2 — Price Service + PnL Calculator ✅
*Backend. Requires 6.1.*
- [x] `PriceService` — Binance `/api/v3/ticker/price` + Yahoo, Redis cache 30s TTL
- [x] `PnLCalculator` — unrealized PnL per position (current price vs avg cost)
- [x] Tests: cache behavior, PnL math

### 6.3 — Portfolio WebSocket ✅
*Backend. Requires 6.2.*
- [x] `WS /ws/portfolio/:id/live` — PnL updates every 5s
- [x] Tests: WS stream delivery

### 6.4 — Portfolio Frontend — Layout + Table ✅
*Frontend. Requires 6.1.*
- [x] Portfolio selector dropdown + "New Portfolio" button
- [x] Summary cards: Total Value, Total PnL (green/red), PnL%, Day Change%
- [x] Positions table: Asset, Qty, Avg Cost, Current Price, Value, PnL, PnL%, Weight
- [x] "Add Position" FAB → modal (adapter, symbol, qty, avg price, date)

### 6.5 — Portfolio Frontend — Charts + Live Updates ✅
*Frontend. Requires 6.3 + 6.4.*
- [x] Allocation donut chart (Recharts) — hover segment highlights table row
- [x] Equity curve line chart (from transaction history)
- [x] Transaction history table (paginated)
- [x] Live PnL updates via WebSocket
- [x] Interaction tests (donut + table)

---

## Phase 7 — News, Events & Alerts 🔲

### 7.1 — News Aggregator 🔲
*Backend. No dependencies.*
- [ ] `internal/news/` — runs every 15 min
- [ ] CryptoPanic API + NewsAPI + RSS fallback (`gofeed`: Reuters, CoinDesk)
- [ ] Dedup by URL, tag with symbols from title + summary
- [ ] Tests: dedup logic, symbol tagging

### 7.2 — News API 🔲
*Backend. Requires 7.1.*
- [ ] `GET /api/v1/news?symbols=&limit=&offset=&from=&to=`
- [ ] `GET /api/v1/news/symbols/:symbol`
- [ ] `GET /api/v1/candles?...&include_news=true` — news flags in candle response

### 7.3 — Alert Evaluator + Notification System 🔲
*Backend. No dependencies.*
- [ ] `internal/alert/` — runs every 60s
- [ ] Types: `price_above`, `price_below`, `price_change_pct`, `rsi_overbought`, `rsi_oversold`
- [ ] On fire: create Notification, publish to Redis `notifications:new`, mark triggered/recurring
- [ ] Alert API: `POST/GET/DELETE /api/v1/alerts`, `PATCH /api/v1/alerts/:id/toggle`
- [ ] Notification API: `GET /api/v1/notifications`, `PATCH /:id/read`, `POST /read-all`
- [ ] `WS /ws/notifications` — real-time push
- [ ] Tests: all 5 condition types, WS delivery

### 7.4 — Frontend News Panel + Chart Markers 🔲
*Frontend. Requires 7.2.*
- [ ] Chart page: collapsible news side panel, scrollable feed (source badge, headline, time ago, sentiment dot)
- [ ] Chart timeline: triangular flag icons for news events → popover (headline + "Read more")

### 7.5 — Frontend Alerts + Notification Bell 🔲
*Frontend. Requires 7.3.*
- [ ] Alerts page: table of active rules, "+" modal (type, symbol, threshold, channels), status + last-fired
- [ ] Notification bell: unread badge, dropdown with 5 recent, "View all" → `/notifications`
- [ ] Bell animation + count increment on new WS notification

---

## Phase 8 — Live Market Monitor & Signal Alerts 🔲

### 8.1 — Monitor Manager — Streaming Path 🔲
*Backend. Requires Phase 2 + 3 complete.*
- [ ] `internal/monitor/` singleton, load active monitors on boot
- [ ] Subscribe to `StreamTicker`, detect candle completion per timeframe
- [ ] Call `strategy.OnCandle()`, warm-start with last 200 candles
- [ ] On signal: save `MonitorSignal`, publish to Redis `monitor:<id>:signal`

### 8.2 — Monitor Manager — Polling Path + Signal Routing 🔲
*Backend. Requires 8.1.*
- [ ] Poll `GetCandles` every N seconds (N = timeframe/10, min 30s) for non-streaming adapters
- [ ] Route signals to: in-app notification, Telegram (Phase 9), webhook (Phase 9)
- [ ] Tests: lifecycle (start/stop/pause/resume), candle-completion detection, signal routing

### 8.3 — Monitor API + WebSocket 🔲
*Backend. Requires 8.2.*
- [ ] `POST/GET/PUT/DELETE /api/v1/monitors`
- [ ] `PATCH /api/v1/monitors/:id/toggle`
- [ ] `GET /api/v1/monitors/:id/signals` (paginated)
- [ ] `WS /ws/monitors/signals` — multiplexed, client sends `{action:"subscribe", monitor_ids:[...]}`
- [ ] Tests: WS multiplexed subscriptions

### 8.4 — Frontend Monitor Page 🔲
*Frontend. Requires 8.3.*
- [ ] Monitor cards grid: name, status (green pulse/gray), symbol, strategy, notification icons, last signal
- [ ] Card actions: Edit, Pause/Resume, Delete
- [ ] Click card → expand: signal history table + mini chart
- [ ] "Create Monitor" modal (strategy selector + params + symbol/timeframe + notifications)

### 8.5 — Frontend Live Signals 🔲
*Frontend. Requires 8.4.*
- [ ] Live signal toast: animated, 8s, dismiss button, bottom-right
- [ ] WS subscription to monitor signals
- [ ] Settings page warning if Telegram not configured

---

## Phase 9 — Telegram Bot & Social Card Generator 🔲

### 9.1 — Telegram + Webhook Senders 🔲
*Backend. No dependencies.*
- [ ] `internal/notification/telegram.go`
  - [ ] `SendText(chatID, text)` — Bot API `sendMessage`
  - [ ] `SendPhoto(chatID, imageBytes, caption)` — Bot API `sendPhoto`
  - [ ] Startup message if token configured
- [ ] `WebhookSender` — POST with HMAC-SHA256 (`X-StratosMarket-Signature`), retry 3x with backoff
- [ ] Tests: mock Bot API, retry + signature verification

### 9.2 — Social Card Generator 🔲
*Backend. No dependencies (parallel with 9.1).*
- [ ] `internal/social/` using `gg` library
- [ ] `GenerateBacktestCard` — dark/light bg, strategy + symbol, 4 metrics, equity sparkline, branding
- [ ] `GenerateSignalCard` — symbol, BUY/SELL colored, price, strategy, timestamp
- [ ] Tests: output dimensions, non-blank pixels

### 9.3 — Social + Settings API 🔲
*Backend. Requires 9.1 + 9.2.*
- [ ] `POST /api/v1/social/backtest-card/:runId?theme=dark|light` → PNG
- [ ] `POST /api/v1/social/signal-card/:signalId` → PNG
- [ ] `POST /api/v1/social/send-telegram`
- [ ] `GET/POST /api/v1/settings/notifications`
- [ ] `POST /api/v1/settings/notifications/test`

### 9.4 — Frontend Share Modal + Settings UI 🔲
*Frontend. Requires 9.3.*
- [ ] "Share" button on backtest results → modal: card preview, dark/light toggle, "Download PNG", "Send to Telegram", "Copy formatted text"
- [ ] Settings page "Notifications" section: Telegram token + chat ID inputs, "Test Connection" button

---

## Phase 10 — AI Assistant Chatbot 🔲

### 10.1 — AI Providers 🔲
*Backend. No dependencies.*
- [ ] `AIProvider` interface (`internal/ai/`)
- [ ] `OpenAIProvider` — `gpt-4o-mini` (configurable model), standard chat completions
- [ ] `OllamaProvider` — local Ollama REST API, configurable model
- [ ] Tests: mock provider interface

### 10.2 — System Prompt Builder + AI API 🔲
*Backend. Requires 10.1.*
- [ ] Dynamic system prompt from `page_context` (symbol, timeframe, indicators, metrics, etc.)
- [ ] `POST /api/v1/ai/chat` — `{messages, page_context}` → `{reply, suggested_questions[3]}`
- [ ] `GET/POST /api/v1/settings/ai` — provider, API key, Ollama URL, model
- [ ] `POST /api/v1/settings/ai/test`
- [ ] Tests: prompt builder (all page contexts), suggested questions format

### 10.3 — Frontend Chat Panel 🔲
*Frontend. Requires 10.2.*
- [ ] Floating AI button (bottom-right, sparkle icon) on every page
- [ ] Slide-up chat panel (~50% height, app visible above)
- [ ] Header: "AI Assistant" + context chip + close
- [ ] Message list: user right-aligned, assistant left-aligned, `react-markdown` rendering
- [ ] Input: text + send (Enter to send, Shift+Enter newline)
- [ ] 3 suggested question chips below input, typing indicator
- [ ] Context auto-captured from Zustand stores on panel open
- [ ] Tests: panel open/close, context capture

### 10.4 — Frontend AI Settings 🔲
*Frontend. Requires 10.2.*
- [ ] Settings page "AI" section: provider selector, API key, Ollama URL, model, "Test Connection"

---

## Phase 11 — Advanced Analytics 🔲

### 11.1 — Param Heatmap 🔲
*Backend. Requires Phase 3 complete.*
- [ ] `GET /api/v1/backtest/runs/:id/param-heatmap?x_param=&y_param=`
- [ ] Up to 20×20 grid, parallel via worker pool
- [ ] Tests: parallel execution, grid correctness

### 11.2 — Monte Carlo Simulation 🔲
*Backend. Requires Phase 3 complete.*
- [ ] `POST /api/v1/backtest/runs/:id/monte-carlo`
- [ ] 1000 simulations → min/max/median equity curves, probability of ruin
- [ ] Tests: distribution shape, ruin probability bounds

### 11.3 — Walk-Forward + Compare Runs 🔲
*Backend. Requires Phase 3 complete.*
- [ ] `GET /api/v1/backtest/runs/:id/walk-forward` — N windows, per-window metrics
- [ ] `POST /api/v1/backtest/compare` — `{run_ids:[...]}` → side-by-side metrics
- [ ] Tests: window splitting logic

### 11.4 — Frontend Analytics Tab 🔲
*Frontend. Requires 11.1–11.3.*
- [ ] "Advanced Analytics" tab added to Backtest results
- [ ] Param Heatmap: axis selectors + color-coded grid (red→green Sharpe), hover tooltip
- [ ] Monte Carlo: "Run Simulation" button + fan chart (5th–95th percentile shaded) + ruin stat
- [ ] Walk-Forward: bar chart of Sharpe per window
- [ ] Compare Runs: multi-select runs + comparison table + combined equity chart

---

## Phase 12 — Open Source Polish & CI 🔲

### 12.1 — Repository Documentation 🔲
- [ ] `README.md` — banner, features, Mermaid architecture diagram, quickstart, env vars table, adding-a-market/strategy guides, screenshots, contributing, license badge
- [ ] `CONTRIBUTING.md` — code style, branch naming, PR template, test/lint instructions
- [ ] `.claude/docs/adding-a-market.md` — step-by-step with code example
- [ ] `.claude/docs/adding-a-strategy.md` — step-by-step with code example

### 12.2 — GitHub CI/CD 🔲
- [ ] `.github/ISSUE_TEMPLATE/bug_report.md` + `feature_request.md`
- [ ] `.github/pull_request_template.md`
- [ ] `.github/workflows/ci.yml` — on PR: `go vet`, `golangci-lint`, `go test ./...`, Docker build, `eslint`, `tsc --noEmit`, `vitest run`

### 12.3 — Seed Data 🔲
*Requires Phase 2 + 3 + 6 + 7 + 8 complete.*
- [ ] `make seed` script
- [ ] 1 year daily candles: BTCUSDT, ETHUSDT, SOLUSDT (Binance) + AAPL, MSFT, SPY (Yahoo)
- [ ] Demo portfolio (3 positions), 1 example monitor, 2 alert rules, sample news items

### 12.4 — Paper Trading Mode (Bonus) 🔲
*Requires Phase 8 complete.*
- [ ] Monitor mode flag: `live_alert` | `paper_trade`
- [ ] In paper trade mode: auto-create transaction in paper portfolio on signal
- [ ] Show paper trade results on monitor detail page

---

## Cross-Cutting Requirements (Every Phase)

- [ ] `go test ./...` must pass after each backend sub-phase
- [ ] `vitest run` must pass after each frontend sub-phase
- [ ] All config via env vars, no hardcoded values
- [ ] UI: card/island design, responsive (mobile + desktop)
- [ ] WebSocket-first for all real-time updates (no frontend polling)
- [ ] Schema-driven UI for strategy params, indicator params, alert conditions
- [ ] `user_id` on every model (single-user now, multi-tenant ready)
- [ ] `make backend-fmt` + `make backend-lint` pass before every commit
- [ ] `make frontend-fmt` + `make frontend-lint` pass before every commit
