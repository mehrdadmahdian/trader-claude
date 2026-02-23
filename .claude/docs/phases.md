# Implementation Phases

> Legend: ✅ Done · 🔲 Todo · 🚧 In Progress

---

## Phase 1 — Project Scaffold & Infrastructure ✅ COMPLETE

### Backend ✅
- [x] Config loader (`godotenv` + `Config` struct with all env vars + defaults)
- [x] Fiber app with CORS, request logger, error handler, graceful shutdown
- [x] `GET /health` → `{status, db, redis, version}`
- [x] MySQL connection via GORM with retry logic (10 attempts, 3s intervals)
- [x] GORM AutoMigrate on startup (all 9 models)
- [x] Redis connection with go-redis, ping on startup
- [x] `AdapterRegistry` + `StrategyRegistry` singletons (`internal/registry/`)
- [x] `MarketAdapter` + `Strategy` interfaces defined (`internal/registry/interfaces.go`)
- [x] Worker pool with configurable concurrency + panic recovery (`internal/worker/pool.go`)
- [x] WebSocket hub singleton with channel subscriptions (`internal/ws/hub.go`)
- [x] All stub API routes registered (`internal/api/routes.go`)

### Frontend ✅
- [x] React 18 + TypeScript + Vite 5
- [x] TailwindCSS v3 + shadcn/ui initialized
- [x] React Router v6 with all 9 page routes
- [x] Zustand stores: theme, sidebar, market, backtest, portfolio, alert, notification
- [x] Axios instance configured (`api/client.ts`)
- [x] TanStack Query provider wrapping app
- [x] Layout shell: collapsible sidebar, top bar with theme toggle + notification bell
- [x] Dark/light mode (Tailwind `dark` class, persisted to `localStorage: trader-theme`)
- [x] All TypeScript interfaces mirroring backend models (`types/index.ts`)
- [x] 9 stub pages (Dashboard, Chart, Backtest, Monitor, Portfolio, Alerts, Notifications, News, Settings)

### Infrastructure ✅
- [x] Docker Compose: MySQL 8.0, Redis 7, Go backend, React frontend
- [x] Health checks on all services
- [x] Air hot-reload for backend, Vite HMR for frontend
- [x] Makefile with all dev targets
- [x] `.env.example` with every env var

---

## Phase 2 — Market Data Layer 🔲

**Goal:** Fetch, store, and serve OHLCV candle data with smart gap-filling.

### Backend 🔲
- [ ] `BinanceAdapter` (`internal/adapter/binance.go`)
  - [ ] `FetchOHLCV` — Binance `/api/v3/klines`, rate-limit + exponential backoff
  - [ ] `FetchSymbols` — Binance `/api/v3/exchangeInfo`, all USDT pairs
  - [ ] `StreamTicker` — Binance WS `wss://stream.binance.com/ws/<symbol>@kline_1m`
  - [ ] `IsStreamingSupported` → true
  - [ ] Register adapter in `main.go`
- [ ] `YahooFinanceAdapter` (`internal/adapter/yahoo.go`)
  - [ ] `FetchOHLCV` — Yahoo unofficial API, map intervals
  - [ ] `FetchSymbols` — hardcoded curated list (~100 symbols: SP500, ETFs, forex)
  - [ ] `IsStreamingSupported` → false
  - [ ] Register adapter in `main.go`
- [ ] `DataService` (`internal/adapter/dataservice.go`)
  - [ ] `GetCandles` — query DB → find gaps → fetch gaps → upsert → return merged
  - [ ] `SyncRecent` — last 500 candles upsert (called by background worker)
  - [ ] Background sync worker: every 5 min, sync symbols accessed in last 24h (track in Redis)
- [ ] API endpoints
  - [ ] `GET /api/v1/markets` — list all registered adapters + metadata
  - [ ] `GET /api/v1/markets/:adapterID/symbols` — list symbols for adapter
  - [ ] `GET /api/v1/candles?adapter=&symbol=&timeframe=&from=&to=` — OHLCV data
  - [ ] `GET /api/v1/candles/timeframes` — `[1m, 5m, 15m, 30m, 1h, 4h, 1d, 1w]`

### Frontend 🔲
- [ ] `<CandlestickChart>` reusable component (lightweight-charts, full lifecycle management)
- [ ] Chart page (`/chart`)
  - [ ] Adapter selector dropdown
  - [ ] Symbol search with debounced autocomplete
  - [ ] Timeframe buttons (1m 5m 15m 30m 1h 4h 1d 1w)
  - [ ] Loading skeleton while data fetches
  - [ ] Error state with retry button
  - [ ] Loading overlay on symbol/timeframe change (no chart destroy/recreate)
- [ ] React Query hooks for candles + symbols + markets

### Tests 🔲
- [ ] BinanceAdapter unit tests (mock HTTP)
- [ ] YahooFinanceAdapter unit tests
- [ ] DataService gap-detection unit tests
- [ ] CandlestickChart render tests

---

## Phase 3 — Strategy Engine & Backtesting 🔲

**Goal:** Run strategies over historical data, compute professional performance metrics.

### Backend 🔲
- [ ] Strategy implementations (`internal/strategy/`)
  - [ ] `EMA Crossover` (`ema_crossover.go`) — params: fast_period, slow_period, signal_on_close
  - [ ] `RSI Strategy` (`rsi.go`) — params: period, overbought, oversold, use_divergence
  - [ ] `MACD Signal` (`macd.go`) — params: fast, slow, signal, histogram_threshold
  - [ ] Register all strategies in `main.go`
- [ ] Backtesting engine (`internal/backtest/engine.go`)
  - [ ] Iterate candles one-by-one (no look-ahead)
  - [ ] Track open/closed positions
  - [ ] Commission (default 0.1% per side) + slippage (default 0.05%)
  - [ ] Full trade record: entry/exit price, time, PnL, PnL%
  - [ ] Equity curve: `{timestamp, value}` per candle
  - [ ] Metrics: total return, annualized return, Sharpe, Sortino, max drawdown + duration, win rate, profit factor, avg win/loss, total/winning/losing trades, largest win/loss
  - [ ] Progress published to Redis `backtest:<run_id>:progress` (0–100%)
  - [ ] Run inside worker pool goroutine
- [ ] API endpoints
  - [ ] `GET /api/v1/strategies` — list all with param schemas
  - [ ] `GET /api/v1/strategies/:id`
  - [ ] `POST /api/v1/backtest/run` — async, returns run_id immediately
  - [ ] `GET /api/v1/backtest/runs` — list past runs
  - [ ] `GET /api/v1/backtest/runs/:id` — full result (equity curve + metrics + trades)
  - [ ] `DELETE /api/v1/backtest/runs/:id`
  - [ ] `WS /ws/backtest/:id/progress` — stream 0–100% + final result

### Frontend 🔲
- [ ] Backtest page (`/backtest`) — two-column layout
  - [ ] Left panel (30%): strategy card grid selector
  - [ ] Auto-generated param form from schema (slider+number for int/float, toggle for bool, segmented for enum, tooltips)
  - [ ] Symbol/adapter/timeframe/date-range/capital/commission inputs
  - [ ] "Run Backtest" button (disabled while running)
  - [ ] Progress bar with % while running
  - [ ] Right panel (70%) — tabs:
    - [ ] Overview tab: metric stat cards + equity area chart
    - [ ] Trades tab: sortable table, click trade highlights equity curve
    - [ ] Chart tab: candlestick with buy/sell markers overlaid

### Tests 🔲
- [ ] EMA/RSI/MACD unit tests (known input → expected output)
- [ ] Backtest engine unit tests (known candle sequence → expected metrics)
- [ ] Backtest API integration tests
- [ ] Param form render tests (each param type)

---

## Phase 4 — Slow-Motion Replay Engine 🔲

**Goal:** Re-watch a completed backtest candle-by-candle in real time.

### Backend 🔲
- [ ] `POST /api/v1/backtest/runs/:id/replay` — create replay session, return `replay_id`
- [ ] `WS /ws/replay/:replay_id` — WebSocket replay session
  - [ ] Control messages: start, pause, resume, step, set_speed (0.25x–10x), seek
  - [ ] Emit: candle, signal, trade_open, trade_close, equity_update, status
  - [ ] Speed as `time.Sleep` between candles (1x = 300ms)

### Frontend 🔲
- [ ] "Replay" button on completed backtest results
- [ ] Replay view: chart builds up candle-by-candle
- [ ] Buy/sell markers appear at signal candles
- [ ] Replay control bar: Play/Pause, Step Forward, speed selector (0.25x–10x), seekable progress bar, timestamp display
- [ ] Live equity mini-chart in bottom-right corner
- [ ] Signal toast ("BUY signal at $42,150") for each trade signal

### Tests 🔲
- [ ] Replay session lifecycle tests
- [ ] Speed control timing tests
- [ ] WS message sequence tests

---

## Phase 5 — Technical Indicators on Chart 🔲

**Goal:** Professional indicator overlay system with dynamic param forms.

### Backend 🔲
- [ ] `GET /api/v1/indicators` — metadata + params schema for all indicators
- [ ] `POST /api/v1/indicators/calculate` — body: `{indicator_id, params, candles}` → time-series arrays
- [ ] Implement in pure Go (`internal/indicator/`):
  - [ ] Overlays: SMA, EMA, WMA, Bollinger Bands, VWAP, Ichimoku Cloud, Parabolic SAR
  - [ ] Panels: RSI, MACD (line + signal + histogram), Stochastic (%K/%D), ATR, OBV, Volume (colored)

### Frontend 🔲
- [ ] "Indicators" button in chart toolbar → searchable modal (grouped: Trend, Momentum, Volatility, Volume)
- [ ] Auto-generated param config form per indicator
- [ ] Active indicator chips in toolbar (click to edit/remove)
- [ ] Overlay indicators as line series on main chart pane
  - [ ] Bollinger Bands: 3 lines + filled area
  - [ ] Ichimoku: full cloud with Kumo coloring
- [ ] Panel indicators in separate panes below chart (with header + close button)
- [ ] Persist active indicators + params to `localStorage`
- [ ] Re-fetch on param change (single indicator only)

### Tests 🔲
- [ ] Indicator calculation unit tests (SMA, EMA, RSI, MACD, Bollinger Bands)
- [ ] Indicator API integration tests
- [ ] localStorage persistence tests

---

## Phase 6 — Portfolio Tracker 🔲

**Goal:** Multi-asset portfolio with real-time PnL via WebSocket.

### Backend 🔲
- [ ] `PortfolioService` (`internal/portfolio/`) — CRUD for portfolios + positions
- [ ] `PriceService` — current prices: Binance `/api/v3/ticker/price` + Yahoo Finance, Redis cache 30s TTL
- [ ] `PnLCalculator` — unrealized PnL per position (current price vs avg cost)
- [ ] API endpoints
  - [ ] `POST/GET/PUT/DELETE /api/v1/portfolios`
  - [ ] `GET /api/v1/portfolios/:id` — with positions + live PnL
  - [ ] `POST/PUT/DELETE /api/v1/portfolios/:id/positions`
  - [ ] `POST/GET /api/v1/portfolios/:id/transactions` (paginated)
  - [ ] `WS /ws/portfolio/:id/live` — PnL updates every 5s

### Frontend 🔲
- [ ] Portfolio page (`/portfolio`)
  - [ ] Portfolio selector dropdown + "New Portfolio" button
  - [ ] Summary cards: Total Value, Total PnL (green/red), PnL %, Day Change %
  - [ ] Positions table (60%): Asset, Qty, Avg Cost, Current Price, Value, PnL, PnL%, Weight — live WS updates
  - [ ] Allocation donut chart (40%) with Recharts — hover segment highlights table row
  - [ ] Equity curve line chart (from transactions history)
  - [ ] Transaction history table (paginated)
  - [ ] "Add Position" FAB → modal (adapter, symbol, qty, avg price, date)
  - [ ] Live PnL updates via WebSocket

### Tests 🔲
- [ ] PriceService cache tests
- [ ] PnLCalculator tests
- [ ] Portfolio WebSocket stream tests
- [ ] Donut chart + table interaction tests

---

## Phase 7 — News, Events & Alerts 🔲

**Goal:** Market news feed, chart event markers, and price/indicator alert rules.

### Backend 🔲
- [ ] `NewsAggregator` (`internal/news/`) — runs every 15 min
  - [ ] CryptoPanic API source (free tier)
  - [ ] NewsAPI source (stock symbols)
  - [ ] RSS fallback (Reuters Markets, CoinDesk) using `gofeed` if no API keys
  - [ ] Dedup by URL, tag with symbols by scanning title + summary
- [ ] `GET /api/v1/news?symbols=&limit=&offset=&from=&to=`
- [ ] `GET /api/v1/news/symbols/:symbol`
- [ ] `GET /api/v1/candles?...&include_news=true` — include news flags in response
- [ ] `AlertEvaluator` (`internal/alert/`) — runs every 60s
  - [ ] Types: `price_above`, `price_below`, `price_change_pct`, `rsi_overbought`, `rsi_oversold`
  - [ ] On fire: create Notification, publish to Redis `notifications:new`, mark triggered/recurring
- [ ] Alert API: `POST/GET/DELETE /api/v1/alerts`, `PATCH /api/v1/alerts/:id/toggle`
- [ ] Notification API: `GET /api/v1/notifications`, `PATCH /:id/read`, `POST /read-all`
- [ ] `WS /ws/notifications` — real-time new notification push

### Frontend 🔲
- [ ] Chart page news panel: collapsible side panel, scrollable feed for active symbol, source badge + headline + time ago + sentiment dot, click → open article in new tab
- [ ] Chart event markers: triangular flag icons on timeline for news events, click → popover with headline + "Read more"
- [ ] Alerts page (`/alerts`): table of active rules, "+" modal (type, symbol, threshold, notification channels), status + created + last-fired
- [ ] Notification bell: unread badge, dropdown with 5 recent, "View all" → `/notifications`
- [ ] Bell animation + count increment on new WS notification

### Tests 🔲
- [ ] NewsAggregator dedup + symbol tagging tests
- [ ] AlertEvaluator condition tests (all 5 types)
- [ ] Notification WS delivery tests

---

## Phase 8 — Live Market Monitor & Signal Alerts 🔲

**Goal:** Run strategies live on streaming market data, alert on trade signals.

### Backend 🔲
- [ ] `MonitorManager` singleton (`internal/monitor/`)
  - [ ] Load all `status=active` monitors on boot
  - [ ] Streaming path: subscribe to `StreamTicker`, detect candle completion per timeframe, call `strategy.OnCandle()`
  - [ ] Polling path: poll `GetCandles` every N seconds (N = timeframe/10, min 30s) for non-streaming adapters
  - [ ] Warm-start each monitor with last 200 candles to prime indicators
  - [ ] On signal: save `MonitorSignal`, publish to Redis `monitor:<id>:signal`, route to notification channels
- [ ] Telegram message on signal fire (formatted with emoji + strategy name + symbol + price + reason)
- [ ] Webhook POST on signal fire (with `X-StratosMarket-Signature` HMAC header)
- [ ] API endpoints
  - [ ] `POST/GET/GET/PUT/DELETE /api/v1/monitors`
  - [ ] `PATCH /api/v1/monitors/:id/toggle`
  - [ ] `GET /api/v1/monitors/:id/signals` — paginated history
  - [ ] `WS /ws/monitors/signals` — multiplexed, client sends `{action:"subscribe", monitor_ids:[...]}`

### Frontend 🔲
- [ ] Monitor page (`/monitor`)
  - [ ] Monitor cards grid with: name, status (green pulse / gray), symbol, strategy, notification icons, last signal
  - [ ] Card actions: Edit, Pause/Resume, Delete
  - [ ] Click card → expand/detail: signal history table + mini chart
  - [ ] Live signal toast: animated, 8s, dismiss button, bottom-right
  - [ ] "Create Monitor" modal (strategy selector + params + symbol/timeframe + notifications)
  - [ ] Settings page warning if Telegram not configured

### Tests 🔲
- [ ] MonitorManager lifecycle tests (start, stop, pause, resume)
- [ ] Candle-completion detection tests per timeframe
- [ ] Signal routing tests (in-app, Telegram, webhook)
- [ ] WS multiplexed signal subscription tests

---

## Phase 9 — Telegram Bot & Social Card Generator 🔲

**Goal:** Telegram delivery + shareable PNG performance cards.

### Backend 🔲
- [ ] `TelegramSender` (`internal/notification/telegram.go`)
  - [ ] `SendText(chatID, text)` — Bot API `sendMessage`
  - [ ] `SendPhoto(chatID, imageBytes, caption)` — Bot API `sendPhoto`
  - [ ] Startup message if token configured: "✅ StratosMarket is online…"
- [ ] `WebhookSender` — POST with HMAC-SHA256 signature, retry 3x with backoff
- [ ] Social card generator (`internal/social/`) using `gg` library
  - [ ] `GenerateBacktestCard` — dark/light background, strategy + symbol headline, 4 key metrics, equity sparkline, branding
  - [ ] `GenerateSignalCard` — symbol, BUY/SELL large colored text, price, strategy, timestamp
- [ ] API endpoints
  - [ ] `POST /api/v1/social/backtest-card/:runId?theme=dark|light` → PNG
  - [ ] `POST /api/v1/social/signal-card/:signalId` → PNG
  - [ ] `POST /api/v1/social/send-telegram`
  - [ ] `GET/POST /api/v1/settings/notifications`
  - [ ] `POST /api/v1/settings/notifications/test`

### Frontend 🔲
- [ ] "Share" button on backtest results → modal with: card preview, dark/light toggle, "Download PNG", "Send to Telegram", "Copy formatted text"
- [ ] Settings page "Notifications" section: Telegram token input, chat ID input, "Test Connection" button

### Tests 🔲
- [ ] Social card generator output tests (dimensions, not blank)
- [ ] TelegramSender mock API tests
- [ ] WebhookSender retry + signature tests

---

## Phase 10 — AI Assistant Chatbot 🔲

**Goal:** Contextual AI chatbot aware of what the user is currently viewing.

### Backend 🔲
- [ ] `AIProvider` interface (`internal/ai/`)
- [ ] `OpenAIProvider` — `gpt-4o-mini` (configurable model), standard chat completions
- [ ] `OllamaProvider` — local Ollama REST API, configurable model (e.g. `llama3.2`)
- [ ] Dynamic system prompt builder from `page_context` (symbol, timeframe, indicators, metrics, etc.)
- [ ] `POST /api/v1/ai/chat` — messages + page_context → `{reply, suggested_questions[3]}`
- [ ] `GET/POST /api/v1/settings/ai` — provider, API key, Ollama URL, model
- [ ] `POST /api/v1/settings/ai/test`

### Frontend 🔲
- [ ] Floating AI button (bottom-right, sparkle icon) on every page
- [ ] Slide-up chat panel (overlay bottom ~50%, app visible above)
- [ ] Panel header: "AI Assistant" + context chip (e.g. "BTCUSDT · 1h · Chart") + close
- [ ] Message list: user right-aligned, assistant left-aligned, `react-markdown` rendering
- [ ] Input: text + send button, Enter to send, Shift+Enter newline
- [ ] 3 suggested question chips below input
- [ ] Typing indicator while waiting for response
- [ ] Context auto-captured from Zustand stores on panel open
- [ ] Settings page "AI" section: provider selector, API key, Ollama URL, model, "Test Connection"

### Tests 🔲
- [ ] AIProvider interface mock tests
- [ ] System prompt builder tests (all page contexts)
- [ ] Suggested questions format tests
- [ ] Chat panel open/close + context capture tests

---

## Phase 11 — Advanced Analytics 🔲

**Goal:** Anti-overfitting tools and deep performance analysis.

### Backend 🔲
- [ ] `GET /api/v1/backtest/runs/:id/param-heatmap?x_param=&y_param=` — up to 20×20 grid, parallel via worker pool
- [ ] `POST /api/v1/backtest/runs/:id/monte-carlo` — 1000 simulations, min/max/median equity, probability of ruin
- [ ] `GET /api/v1/backtest/runs/:id/walk-forward` — divide into N windows, per-window metrics
- [ ] `POST /api/v1/backtest/compare` — body: `{run_ids: [...]}` → side-by-side metrics

### Frontend 🔲
- [ ] Advanced Analytics tab added to Backtest results
  - [ ] Param Heatmap: axis selectors + color-coded grid (red→green Sharpe), hover shows params + value
  - [ ] Monte Carlo: "Run Simulation" button + fan chart (5th–95th percentile shaded) + probability of ruin stat
  - [ ] Walk-Forward: bar chart of Sharpe per window
  - [ ] Compare Runs: multi-select previous runs + comparison table + combined equity chart

### Tests 🔲
- [ ] Heatmap parallel execution tests
- [ ] Monte Carlo distribution tests
- [ ] Walk-forward window splitting tests

---

## Phase 12 — Open Source Polish & CI 🔲

**Goal:** Production-quality repo for public release.

### Repository Files 🔲
- [ ] `README.md` — banner, features, architecture diagram (Mermaid), quickstart, env vars table, adding-a-market guide, adding-a-strategy guide, screenshots, contributing, license badge
- [ ] `CONTRIBUTING.md` — code style, branch naming, PR template, test/lint instructions
- [ ] `.github/ISSUE_TEMPLATE/bug_report.md`
- [ ] `.github/ISSUE_TEMPLATE/feature_request.md`
- [ ] `.github/pull_request_template.md`
- [ ] `.github/workflows/ci.yml` — on PR to main: `go vet`, `golangci-lint`, `go test ./...`, Docker build, `eslint`, `tsc --noEmit`, `vitest run`
- [ ] `.claude/docs/adding-a-market.md` — step-by-step with code example
- [ ] `.claude/docs/adding-a-strategy.md` — step-by-step with code example

### Seed Data (`make seed`) 🔲
- [ ] 1 year daily candles: BTCUSDT, ETHUSDT, SOLUSDT (Binance) + AAPL, MSFT, SPY (Yahoo)
- [ ] Demo portfolio with 3 positions
- [ ] 1 example monitor (EMA Crossover on BTCUSDT 1h, in-app only)
- [ ] 2 example alert rules
- [ ] Sample news items

### Paper Trading Mode (Bonus) 🔲
- [ ] Monitor mode flag: `live_alert` | `paper_trade`
- [ ] In paper trade mode: auto-create transaction in paper portfolio on signal
- [ ] Show paper trade results on monitor detail page

---

## Cross-Cutting Requirements (Every Phase)

- [ ] Unit tests for every backend function (`go test ./...` must pass)
- [ ] Unit tests for every frontend component/hook (`vitest run` must pass)
- [ ] All config via env vars, no hardcoded values
- [ ] UI: card/island design style, responsive (mobile + desktop)
- [ ] WebSocket-first for all real-time updates (no frontend polling)
- [ ] Schema-driven UI for strategy params, indicator params, alert conditions
- [ ] `user_id` column on every model (single-user now, multi-tenant ready)
- [ ] `make backend-fmt` + `make backend-lint` pass before every commit
- [ ] `make frontend-fmt` + `make frontend-lint` pass before every commit
