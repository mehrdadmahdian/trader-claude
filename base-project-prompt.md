> ## Project Overview
>
> Build a full-stack market backtesting, live monitoring, and research platform. This will be published as an open-source GitHub project under MIT license. It must be production-quality code, well-structured, and easy for other developers to contribute to. After you completed everything please extract claude code based on what you impelented which would be usefull for rest of works in a standard way.
>
> **Core Philosophy:**
> - Everything is pluggable. New markets, new strategies, new notification channels = implement one interface, register it, done.
> - The UI drives itself from backend metadata. Strategy params, market lists, indicator configs are all served as JSON schema — the frontend renders them dynamically, no hardcoded forms.
> - Data is local-first. Historical OHLCV is persisted in TimescaleDB. APIs are only called for delta updates. The app must be fully functional offline for historical data once synced.
> - One command start. `docker compose up` brings everything online.
>
> ---
>
> ## Tech Stack (Non-Negotiable)
>
> | Layer | Choice |
> |---|---|
> | Backend language | Go 1.22+ |
> | Backend framework | Fiber v2 |
> | ORM / DB driver | GORM + pgx |
> | DB | mysql is preferred|
> | Cache / PubSub | Redis 7 |
> | WebSocket | Gorilla WebSocket (via Fiber) |
> | Background jobs | Custom Go worker pool (no external queue needed at this stage) |
> | Frontend framework | React 18 + TypeScript + Vite |
> | UI component library | shadcn/ui (Radix primitives + Tailwind CSS) |
> | State management | Zustand |
> | Server state / API calls | TanStack Query (React Query v5) |
> | Charts | TradingView Lightweight Charts v4 (open source npm package) |
> | HTTP client | Axios |
> | Styling | Tailwind CSS v3 |
> | Containerization | Docker |
>
> ---
>
> ## Monorepo Structure
>
> ```
> trader-claude/
> ├── backend/
> │   ├── cmd/
> │   │   └── server/
> │   │       └── main.go
> │   ├── internal/
> │   │   ├── adapter/          # MarketAdapter implementations
> │   │   ├── strategy/         # Strategy implementations
> │   │   ├── backtest/         # Backtesting engine
> │   │   ├── monitor/          # Live monitor manager
> │   │   ├── portfolio/        # Portfolio service
> │   │   ├── news/             # News aggregation service
> │   │   ├── alert/            # Alert routing service
> │   │   ├── ai/               # AI assistant proxy
> │   │   ├── notification/     # Telegram, webhook senders
> │   │   ├── social/           # Social card generator
> │   │   ├── ws/               # WebSocket hub
> │   │   ├── api/              # Route handlers (one file per domain)
> │   │   ├── models/           # GORM model structs
> │   │   ├── registry/         # Strategy and adapter registries
> │   │   └── config/           # Env config loader
> │   ├── migrations/           # SQL migration files
> │   ├── Dockerfile
> │   └── go.mod
> ├── frontend/
> │   ├── src/
> │   │   ├── components/       # Reusable UI components
> │   │   │   ├── chart/
> │   │   │   ├── strategy/
> │   │   │   ├── portfolio/
> │   │   │   ├── monitor/
> │   │   │   ├── news/
> │   │   │   ├── ai/
> │   │   │   └── shared/
> │   │   ├── pages/            # One folder per route
> │   │   ├── hooks/            # Custom React hooks
> │   │   ├── stores/           # Zustand stores
> │   │   ├── api/              # Axios API call functions
> │   │   ├── types/            # TypeScript interfaces mirroring backend models
> │   │   ├── lib/              # Utilities
> │   │   └── main.tsx
> │   ├── Dockerfile
> │   └── package.json
> ├── docker/
> │   ├── timescaledb/
> │   │   └── init.sql
> │   └── redis/
> │       └── redis.conf
> ├── docker-compose.yml
> ├── .env.example
> ├── Makefile
> └── README.md
> ```
>
> ---
>
> ## Database Schema
>
> Design these tables in TimescaleDB. Write proper SQL migration files.
>
> ```sql
> -- Candles (hypertable partitioned by time)
> candles (id, symbol, market, timeframe, open, high, low, close, volume, timestamp)
>
> -- Strategies (metadata only, logic is in Go)
> strategies (id, name, description, version, params_schema jsonb, created_at)
>
> -- Backtest runs
> backtest_runs (id, strategy_id, symbol, market, timeframe, from_time, to_time, params jsonb,
>                status, equity_curve jsonb, metrics jsonb, trades jsonb, created_at, completed_at)
>
> -- Portfolios
> portfolios (id, name, description, base_currency, created_at)
>
> -- Portfolio positions
> positions (id, portfolio_id, symbol, market, quantity, avg_price, opened_at, closed_at, status)
>
> -- Transactions
> transactions (id, portfolio_id, position_id, type [buy/sell], symbol, quantity, price, fee, executed_at)
>
> -- Live monitors
> monitors (id, name, symbol, market, timeframe, strategy_id, params jsonb,
>           notify_inapp bool, notify_telegram bool, notify_webhook bool,
>           webhook_url, telegram_chat_id, status [active/paused], created_at)
>
> -- Monitor signal history
> monitor_signals (id, monitor_id, signal [buy/sell/none], price, triggered_at, delivered bool, metadata jsonb)
>
> -- News items
> news_items (id, source, title, url, summary, symbols jsonb, sentiment, published_at, fetched_at)
>
> -- Alert rules
> alert_rules (id, name, symbol, market, condition_type, condition_params jsonb,
>              notify_inapp bool, notify_telegram bool, status, created_at)
>
> -- Notifications
> notifications (id, type, title, body, metadata jsonb, read bool, created_at)
>
> -- User settings (single-user for now, id always = 1)
> user_settings (id, telegram_bot_token, telegram_chat_id, openai_api_key,
>                ollama_url, ai_provider [openai/ollama], theme, created_at, updated_at)
> ```
>
> ---
>
> ## Core Interfaces (Go)
>
> These are the two foundational interfaces everything else builds on. Define them in `internal/registry/interfaces.go`.
>
> ```go
> // MarketAdapter — implement this to add a new market/exchange
> type MarketAdapter interface {
>     ID() string                          // e.g. "binance", "yahoo"
>     Name() string                        // e.g. "Binance", "Yahoo Finance"
>     SupportedMarkets() []string          // e.g. ["crypto"], ["stocks", "etf"]
>     FetchOHLCV(ctx context.Context, symbol, timeframe string, from, to time.Time) ([]Candle, error)
>     FetchSymbols(ctx context.Context) ([]Symbol, error)
>     StreamTicker(ctx context.Context, symbol string) (<-chan Tick, error)
>     IsStreamingSupported() bool
> }
>
> // Strategy — implement this to add a new trading strategy
> type Strategy interface {
>     ID() string                          // unique slug, e.g. "ema_crossover"
>     Name() string                        // display name
>     Description() string
>     Version() string
>     Params() []ParamDefinition           // defines the UI controls
>     Init(params map[string]any) error    // called before first candle
>     OnCandle(candle Candle, state *StrategyState) Signal
>     Reset()                              // called between backtest runs
> }
>
> // ParamDefinition — drives auto-rendered UI controls
> type ParamDefinition struct {
>     Key          string      // used as map key in params
>     Label        string      // display name in UI
>     Description  string      // tooltip text
>     Type         string      // "int" | "float" | "bool" | "enum" | "string"
>     Default      any
>     Min          *float64    // for int/float
>     Max          *float64    // for int/float
>     Step         *float64    // for sliders
>     Options      []EnumOption // for enum type
> }
>
> // Signal returned by OnCandle
> type Signal struct {
>     Action    string  // "buy" | "sell" | "hold"
>     Price     float64
>     StopLoss  *float64
>     TakeProfit *float64
>     Reason    string  // human-readable explanation for the AI and logs
> }
> ```
>
> ---
>
> ## Phase 1 — Project Scaffold & Infrastructure
>
> **Goal:** Running skeleton with all services communicating. No business logic yet.
>
> **Docker Compose** must define these services:
> - `timescaledb`: Init script creates the DB, runs migrations, enables the extension. Healthcheck included.
> - `redis`: Redis 7 with persistence enabled. Healthcheck included.
> - `backend`: Go app, watches for rebuild in dev mode (use Air for hot reload). Depends on timescaledb and redis being healthy.
> - `frontend`: Vite dev server, hot reload enabled. Proxies `/api` and `/ws` to backend.
>
> **Backend scaffold:**
> - Config loader using `godotenv` + a `Config` struct that reads all env vars with defaults and validation
> - Fiber app setup with: CORS (allow localhost:5173), request logger, error handler middleware, graceful shutdown
> - Health check endpoint: `GET /health` returns `{ status: "ok", db: "ok", redis: "ok", version: "0.1.0" }`
> - Database connection with GORM + TimescaleDB. Run auto-migrations on startup.
> - Redis connection with go-redis. Ping on startup.
> - Registry pattern: `AdapterRegistry` and `StrategyRegistry` are singletons that hold registered implementations. Adapters and strategies register themselves via `init()` functions in their respective packages.
> - Worker pool: simple goroutine pool with configurable concurrency, used for background data sync and backtest runs
>
> **Frontend scaffold:**
> - Vite + React 18 + TypeScript
> - Tailwind CSS configured
> - shadcn/ui initialized with all base components installed
> - React Router v6 with routes defined for every page (even if pages are empty stubs)
> - Zustand store stubs for: `useMarketStore`, `useBacktestStore`, `usePortfolioStore`, `useMonitorStore`, `useNotificationStore`, `useSettingsStore`
> - Axios instance configured with base URL, interceptors for error handling
> - TanStack Query provider wrapping the app
> - Layout shell: sidebar with nav links (Dashboard, Chart, Backtest, Portfolio, Monitor, News, Settings), top bar with theme toggle and notification bell, main content area
> - Dark/light mode: Tailwind `dark` class strategy, persisted to localStorage, toggle in top bar
>
> **Deliverables:** `docker compose up` starts all 4 services. Frontend loads and shows the empty shell. `/health` returns 200.
>
> ---
>
> ## Phase 2 — Market Data Layer
>
> **Goal:** Fetch, store, and serve OHLCV candle data. Smart caching so APIs are only called for missing ranges.
>
> **Backend — MarketAdapter implementations:**
>
> Implement two adapters:
>
> `BinanceAdapter` (in `internal/adapter/binance.go`):
> - ID: `"binance"`, supports `["crypto"]`
> - `FetchOHLCV`: calls Binance public REST API (`/api/v3/klines`). Maps their response to the internal `Candle` struct. Handles rate limiting with exponential backoff.
> - `FetchSymbols`: calls `/api/v3/exchangeInfo`, returns all USDT pairs as `Symbol` structs with name, base asset, quote asset, market type
> - `StreamTicker`: connects to Binance WebSocket stream (`wss://stream.binance.com/ws/<symbol>@kline_1m`), returns a channel of `Tick` events
> - `IsStreamingSupported`: returns true
>
> `YahooFinanceAdapter` (in `internal/adapter/yahoo.go`):
> - ID: `"yahoo"`, supports `["stocks", "etf", "forex", "commodities"]`
> - `FetchOHLCV`: uses the Yahoo Finance unofficial API (`https://query1.finance.yahoo.com/v8/finance/chart/<symbol>`). Map intervals: `1m→1m, 5m→5m, 1h→60m, 1d→1d, 1w→1wk`
> - `FetchSymbols`: return a curated hardcoded list of ~100 popular symbols (SP500 components, major ETFs, major forex pairs) — no need to call an API for this
> - `IsStreamingSupported`: returns false (Yahoo doesn't support streaming)
>
> **Backend — Data service (`internal/adapter/dataservice.go`):**
> - `GetCandles(ctx, adapterID, symbol, timeframe, from, to)`:
>   1. Query TimescaleDB for candles in the requested range
>   2. Identify gaps (missing date ranges) in the stored data
>   3. For each gap, call the adapter's `FetchOHLCV`
>   4. Store fetched candles in DB (upsert, ignore duplicates)
>   5. Return the complete merged dataset
> - `SyncRecent(ctx, adapterID, symbol, timeframe)`: fetches the last 500 candles and upserts them. Called by the background sync worker.
> - Background sync worker: runs every 5 minutes, syncs all symbols that have been accessed in the last 24 hours (track access time in Redis)
>
> **Backend — API endpoints:**
> ```
> GET  /api/markets                          → list all registered adapters and their metadata
> GET  /api/markets/:adapterID/symbols       → list available symbols for that adapter
> GET  /api/candles?adapter=&symbol=&timeframe=&from=&to=  → get OHLCV data
> GET  /api/candles/timeframes               → list supported timeframes [1m,5m,15m,30m,1h,4h,1d,1w]
> ```
>
> **Frontend — Chart page (`/chart`):**
> - Top bar: adapter selector dropdown (Binance / Yahoo), symbol search with debounced autocomplete, timeframe buttons (1m 5m 15m 30m 1h 4h 1d 1w)
> - TradingView Lightweight Charts: render candlestick series from API data. Implement proper chart initialization, data loading, and cleanup on symbol/timeframe change.
> - Loading skeleton while data fetches. Error state with retry button.
> - The chart container must be a reusable `<CandlestickChart>` component that accepts candle data as props and handles all chart lifecycle internally.
> - On timeframe or symbol change, show a loading overlay on the existing chart rather than destroying and recreating it.
>
> ---
>
> ## Phase 3 — Strategy Engine & Backtesting
>
> **Goal:** Run a strategy over historical data, track every trade, compute professional metrics.
>
> **Backend — Built-in strategies (each in their own file in `internal/strategy/`):**
>
> `EMA Crossover` (`ema_crossover.go`):
> - Params: `fast_period` (int, 5–50, default 9), `slow_period` (int, 10–200, default 21), `signal_on_close` (bool, default true)
> - Logic: buy when fast EMA crosses above slow EMA, sell when fast crosses below slow
> - Uses a proper EMA calculation (not SMA approximation)
>
> `RSI Strategy` (`rsi.go`):
> - Params: `period` (int, 5–30, default 14), `overbought` (float, 60–90, default 70), `oversold` (float, 10–40, default 30), `use_divergence` (bool, default false)
> - Logic: buy when RSI crosses above oversold level, sell when RSI crosses below overbought level
>
> `MACD Signal` (`macd.go`):
> - Params: `fast` (int, default 12), `slow` (int, default 26), `signal` (int, default 9), `histogram_threshold` (float, default 0)
> - Logic: buy when MACD line crosses above signal line and histogram > threshold, sell on the opposite cross
>
> **Backend — Backtesting engine (`internal/backtest/engine.go`):**
>
> The engine takes: strategy instance, candle slice, initial capital, commission rate, slippage model.
>
> It iterates candles one by one, calling `strategy.OnCandle()`. It must:
> - Never look ahead — the strategy only sees candles up to the current index
> - Track open and closed positions
> - Apply commission on each trade (default 0.1% per side)
> - Apply slippage (default 0.05% price impact)
> - Record every trade: entry price, exit price, entry time, exit time, PnL, PnL%, direction
> - Build an equity curve (one data point per candle: timestamp + portfolio value)
> - On completion, calculate:
>   - Total return %
>   - Annualized return %
>   - Sharpe ratio (using 0% risk-free rate)
>   - Sortino ratio
>   - Max drawdown % and drawdown duration
>   - Win rate %
>   - Profit factor (gross profit / gross loss)
>   - Average win / average loss
>   - Total trades, winning trades, losing trades
>   - Largest win, largest loss
>
> The engine runs inside a worker goroutine. Status updates (progress %) are published to Redis pub/sub channel `backtest:<run_id>:progress` so the frontend can show a progress bar via WebSocket.
>
> **Backend — API endpoints:**
> ```
> GET  /api/strategies                          → list all registered strategies with their param schemas
> GET  /api/strategies/:id                      → get single strategy detail
> POST /api/backtest/run                        → start a backtest run (async, returns run_id immediately)
>      body: { strategy_id, symbol, adapter, timeframe, from, to, params, initial_capital, commission }
> GET  /api/backtest/runs                       → list all past runs
> GET  /api/backtest/runs/:id                   → get full run result (equity curve, metrics, trades)
> DELETE /api/backtest/runs/:id                 → delete a run
> WS   /ws/backtest/:id/progress                → stream progress updates (0–100%) and final result
> ```
>
> **Frontend — Backtest page (`/backtest`):**
>
> Layout: two-column. Left panel (30% width) is the configuration panel. Right panel (70%) is results.
>
> Left panel contains:
> - Strategy selector: card-style grid showing all registered strategies with name, description. Click to select.
> - Once selected: param form auto-generated from the strategy's `params` schema. Int/float params render as a slider with a number input next to it. Bool params render as a toggle switch. Enum params render as a segmented control. Each param shows its label and description as a tooltip.
> - Symbol and adapter selector (reuse the component from the Chart page)
> - Timeframe selector
> - Date range picker (from / to)
> - Initial capital input, commission rate input
> - "Run Backtest" button. Disabled while a run is in progress.
>
> Right panel contains (tabs):
> - **Overview tab**: key metrics displayed as stat cards in a grid (Total Return, Sharpe Ratio, Max Drawdown, Win Rate, Profit Factor, Total Trades). Below the stats: the equity curve rendered with TradingView Lightweight Charts as an area chart.
> - **Trades tab**: table of all trades with columns: #, Direction, Entry Time, Entry Price, Exit Time, Exit Price, PnL, PnL%, Duration. Sortable columns. Clicking a trade highlights it on the equity curve.
> - **Chart tab**: the full candlestick chart for the tested symbol/timeframe with buy/sell signal markers overlaid at the exact candle where the trade was entered and exited.
>
> While a backtest is running, show a progress bar at the top of the right panel with the current % and "Running…" label. When complete, animate the results appearing.
>
> ---
>
> ## Phase 4 — Slow-Motion Replay Engine
>
> **Goal:** Re-watch a completed backtest playing out candle by candle in real time, as if watching the market unfold live.
>
> **Backend:**
> - `POST /api/backtest/runs/:id/replay` — creates a replay session, returns a `replay_id`
> - `WS /ws/replay/:replay_id` — WebSocket connection for the replay session
> - On WebSocket connect, the server loads the backtest's candle data and waits for a START command
> - The client sends control messages as JSON:
>   - `{ "action": "start" }`
>   - `{ "action": "pause" }`
>   - `{ "action": "resume" }`
>   - `{ "action": "step" }` — advance exactly one candle
>   - `{ "action": "set_speed", "value": 1.0 }` — multiplier, 0.25x to 10x
>   - `{ "action": "seek", "index": 450 }` — jump to candle index
> - The server emits these message types:
>   - `{ "type": "candle", "data": Candle, "index": N, "total": M }`
>   - `{ "type": "signal", "data": Signal }` — when the strategy fires at this candle
>   - `{ "type": "trade_open", "data": Trade }`
>   - `{ "type": "trade_close", "data": Trade }`
>   - `{ "type": "equity_update", "data": { "value": 10523.4, "timestamp": "..." } }`
>   - `{ "type": "status", "data": { "state": "playing|paused|complete", "index": N } }`
> - Speed is implemented as a `time.Sleep` between emitting candles. At 1x speed, sleep = 300ms. At 10x = 30ms. At 0.25x = 1200ms.
>
> **Frontend — Replay view (accessible from the Backtest Results page):**
> - "Replay" button on any completed backtest run opens the replay view
> - The replay view replaces the static chart with a live-updating chart
> - Chart starts empty and builds up candle by candle as the server streams them
> - Buy/sell markers appear on the chart exactly when the trade was signaled
> - Below the chart: replay control bar with:
>   - Play/Pause button (shows current state)
>   - Step Forward button (one candle at a time when paused)
>   - Speed control: segmented control with options [0.25x, 0.5x, 1x, 2x, 5x, 10x]
>   - Progress bar showing current candle index / total candles. Clickable to seek.
>   - Current candle timestamp display
> - Live equity mini-chart updates in real time in the bottom-right corner during replay
> - When a trade signal fires, show a brief animated toast notification ("BUY signal at $42,150")
>
> ---
>
> ## Phase 5 — Technical Indicators on Chart
>
> **Goal:** Users can add professional indicators to the chart with full parameter control.
>
> **Backend:**
> - `GET /api/indicators` — returns metadata for all available indicators: id, name, description, params schema (same ParamDefinition pattern as strategies), output type (overlay or panel), output series definitions
> - `POST /api/indicators/calculate` — body: `{ indicator_id, params, candles: [...] }` — calculates and returns the indicator values as time-series arrays
> - Implement these indicators server-side in pure Go (no external TA libraries required for these):
>   - Overlays (on main chart): SMA, EMA, WMA, Bollinger Bands (upper/mid/lower), VWAP, Ichimoku Cloud (9/26/52 lines + cloud), Parabolic SAR
>   - Panels (below chart): RSI, MACD (line + signal + histogram bars), Stochastic (%K and %D lines), ATR, OBV, Volume with colored bars (green/red based on candle direction)
>
> **Frontend — Indicator system:**
> - "Indicators" button in chart top bar opens an indicator selector modal
> - Modal shows a searchable list of all indicators grouped by category (Trend, Momentum, Volatility, Volume)
> - Clicking an indicator opens its param configuration form (auto-generated from schema, same as strategy params)
> - Each active indicator appears as a chip/badge in the chart toolbar showing its name and params summary. Click the chip to edit params or remove.
> - Overlay indicators render as line series directly on the candlestick chart pane. Bollinger Bands render as three lines with the area between upper and lower filled with a low-opacity color. Ichimoku cloud renders the Kumo (cloud) with appropriate coloring.
> - Panel indicators render in separate panes below the main chart. The chart layout expands vertically to accommodate them. Each panel has a small header with the indicator name and a close button.
> - Indicator data is fetched from the backend after the candle data loads. On param change, re-fetches only that indicator's data.
> - Active indicators and their params are persisted to localStorage so they survive page refresh.
>
> ---
>
> ## Phase 6 — Portfolio Tracker
>
> **Goal:** Track a multi-asset portfolio with real-time PnL.
>
> **Backend:**
>
> Services in `internal/portfolio/`:
> - `PortfolioService` handles CRUD for portfolios and positions
> - `PriceService` fetches current prices: for crypto uses Binance `/api/v3/ticker/price`, for stocks uses Yahoo Finance. Caches prices in Redis with 30s TTL.
> - `PnLCalculator` computes unrealized PnL for each position using current price vs avg cost
>
> API endpoints:
> ```
> POST   /api/portfolios                              → create portfolio
> GET    /api/portfolios                              → list portfolios
> GET    /api/portfolios/:id                          → get portfolio with positions and PnL
> PUT    /api/portfolios/:id                          → update portfolio
> DELETE /api/portfolios/:id                          → delete portfolio
>
> POST   /api/portfolios/:id/positions                → add position
>        body: { symbol, adapter, quantity, avg_price, opened_at }
> PUT    /api/portfolios/:id/positions/:posId         → update position
> DELETE /api/portfolios/:id/positions/:posId         → remove position
>
> POST   /api/portfolios/:id/transactions             → record a transaction
> GET    /api/portfolios/:id/transactions             → list transactions (paginated)
>
> WS     /ws/portfolio/:id/live                       → streams live PnL updates every 5 seconds
>        emits: { positions: [{ symbol, current_price, pnl, pnl_pct }], total_value, total_pnl, total_pnl_pct }
> ```
>
> **Frontend — Portfolio page (`/portfolio`):**
>
> Top section: portfolio selector (dropdown to switch between portfolios, "+ New Portfolio" button).
>
> Summary cards row: Total Value, Total PnL (with color — green/red), Total PnL %, Day Change %.
>
> Two-column layout below:
> - Left (60%): positions table with columns: Asset, Quantity, Avg Cost, Current Price, Value, PnL, PnL%, Weight %. Each row has a colored PnL cell (green positive, red negative). Rows update live via WebSocket.
> - Right (40%): allocation donut chart using Recharts, showing % allocation per asset. Hovering a segment highlights the corresponding table row.
>
> Below the two columns: equity curve chart (line chart of portfolio total value over time, computed from transactions history).
>
> Transaction history section at the bottom: table with Date, Type (buy/sell badge), Symbol, Quantity, Price, Total Value. Paginated.
>
> "+ Add Position" floating action button opens a modal: adapter selector, symbol search, quantity input, avg price input, date picker.
>
> ---
>
> ## Phase 7 — News, Events & In-App Alerts
>
> **Goal:** Surface relevant market news, mark events on the chart, and let users create price/indicator alerts.
>
> **Backend — News service (`internal/news/`):**
> - `NewsAggregator` runs every 15 minutes as a background job
> - Sources:
>   - CryptoPanic API (free tier): `https://cryptopanic.com/api/v1/posts/?auth_token=<token>&public=true` — parses posts, extracts associated currencies as symbols
>   - NewsAPI: `https://newsapi.org/v2/everything?q=<symbol>&sortBy=publishedAt` — for stock symbols
>   - If neither API key is configured, still works: fetches from a few public RSS feeds (Reuters Markets, CoinDesk) as fallback using `gofeed` library
> - Deduplicates by URL, stores in `news_items` table
> - Tags each item with relevant symbols by scanning title + summary for known symbol names
>
> API endpoints:
> ```
> GET /api/news?symbols=BTC,ETH&limit=20&offset=0&from=&to=   → paginated news feed
> GET /api/news/symbols/:symbol                                 → news for a specific symbol
> ```
>
> News event markers on chart: `GET /api/candles` response optionally includes news events within the requested time range when `?include_news=true` is passed. The frontend renders these as small flag markers on the chart. Hovering a flag shows the news headline in a tooltip.
>
> **Backend — Alert system (`internal/alert/`):**
> - `AlertRule` types supported:
>   - `price_above`: fires when current price > threshold
>   - `price_below`: fires when current price < threshold
>   - `price_change_pct`: fires when price changes by X% within a timeframe
>   - `rsi_overbought`: fires when RSI(period) > level
>   - `rsi_oversold`: fires when RSI(period) < level
> - `AlertEvaluator` runs every 60 seconds for all active rules, checking prices from the `PriceService` cache
> - When a rule fires: creates a `Notification` record, publishes to Redis `notifications:new` channel, marks the rule as `triggered` (one-shot) or keeps it active (recurring)
>
> API endpoints:
> ```
> POST   /api/alerts                  → create alert rule
> GET    /api/alerts                  → list alert rules
> DELETE /api/alerts/:id              → delete alert rule
> PATCH  /api/alerts/:id/toggle       → enable/disable
>
> GET    /api/notifications           → list notifications (paginated, filter by read/unread)
> PATCH  /api/notifications/:id/read  → mark as read
> POST   /api/notifications/read-all  → mark all as read
> WS     /ws/notifications            → real-time push of new notifications
> ```
>
> **Frontend — News & Alerts:**
> - News panel: collapsible side panel on the Chart page. Shows a scrollable feed of news items relevant to the currently viewed symbol. Each item: source badge, headline, time ago, sentiment indicator (bullish/bearish/neutral colored dot). Clicking opens the full article in a new tab.
> - Chart event markers: small triangular flag icons on the price chart timeline when news events exist. Clicking a flag opens a small popover with the headline and a "Read more" link.
> - Alerts page (`/alerts`): table of active alert rules. "+" button opens a create-alert modal with: alert type selector, symbol search, threshold/params inputs, notification channel checkboxes (in-app, Telegram). Each row shows status (active/triggered), created time, and last-fired time.
> - Notification bell in the top bar shows unread count badge. Clicking opens a dropdown showing the 5 most recent notifications. "View all" link goes to `/notifications` page which shows the full history.
> - New notifications received via WebSocket cause the bell icon to animate and the count to increment.
>
> ---
>
> ## Phase 8 — Live Market Monitor & Trade Signal Alerts
>
> **Goal:** Run a strategy continuously on live market data and alert the user when the strategy signals a trade.
>
> **Backend — Monitor system (`internal/monitor/`):**
>
> `MonitorManager` is a singleton service that manages the lifecycle of all active monitors. It starts on app boot and loads all monitors with `status = active` from the DB.
>
> For each active monitor:
> - If the adapter supports streaming (`IsStreamingSupported() == true`): subscribe to the adapter's `StreamTicker` channel. On each tick, check if a new candle has completed (using the monitor's timeframe). When a new candle completes, run `strategy.OnCandle()`.
> - If the adapter does not support streaming: poll `GetCandles` every N seconds (N = timeframe duration / 10, minimum 30s) to check for new candles.
> - Each monitor maintains its own `StrategyState` (indicator buffers, last signal, etc.). The state is warm-started by feeding it the last 200 candles on monitor startup so indicators are primed.
> - When `OnCandle()` returns a signal with `Action != "hold"`:
>   1. Store a `MonitorSignal` record in DB
>   2. Publish to Redis channel `monitor:<id>:signal`
>   3. Route to notification channels based on monitor config (in-app notification, Telegram message, webhook POST)
>
> Telegram message format when a signal fires:
> ```
> 🚨 StratosMarket Signal
> Strategy: EMA Crossover
> Symbol: BTCUSDT (Binance)
> Signal: 📈 BUY
> Price: $42,150.00
> Time: 2024-01-15 14:30:00 UTC
> Reason: Fast EMA(9) crossed above Slow EMA(21)
> ```
>
> Webhook POST body: full signal JSON including monitor ID, strategy params, symbol, price, timestamp, reason.
>
> A `WebSocketHub` in `internal/ws/` manages all active WebSocket connections. When a signal is published to Redis, a Redis subscriber goroutine picks it up and broadcasts it to all connected clients subscribed to that monitor's channel.
>
> API endpoints:
> ```
> POST   /api/monitors                      → create monitor
>        body: { name, symbol, adapter, timeframe, strategy_id, params,
>                notify_inapp, notify_telegram, notify_webhook, webhook_url }
> GET    /api/monitors                      → list all monitors with status
> GET    /api/monitors/:id                  → get monitor detail
> PUT    /api/monitors/:id                  → update monitor config (restarts the monitor goroutine)
> DELETE /api/monitors/:id                  → delete monitor and stop goroutine
> PATCH  /api/monitors/:id/toggle           → pause or resume the monitor
> GET    /api/monitors/:id/signals          → paginated signal history for this monitor
>
> WS     /ws/monitors/signals               → multiplexed stream of all live signals across all monitors
>        client sends: { "action": "subscribe", "monitor_ids": [1, 2, 3] }
>        server sends: { "monitor_id": 1, "signal": "buy", "price": 42150, "timestamp": "...", "reason": "..." }
> ```
>
> **Frontend — Monitor page (`/monitor`):**
>
> Header: "Live Monitors" title + "Create Monitor" button.
>
> Monitor cards grid: each card shows:
> - Monitor name and colored status badge (Active = green pulse dot, Paused = gray)
> - Symbol + adapter badge, timeframe badge
> - Strategy name + key params summary
> - Notification channels icons (bell for in-app, Telegram logo, webhook icon)
> - Last signal: direction badge + price + time ago. "No signals yet" if none.
> - Card actions: Edit (pencil), Pause/Resume toggle, Delete (trash)
>
> Clicking a monitor card expands it or navigates to a detail view showing:
> - Signal history table: timestamp, signal direction (BUY/SELL colored badge), price, reason
> - Mini chart of the monitored symbol for context
>
> Live signal toast: when a signal arrives via WebSocket, show a prominent animated toast in the bottom-right corner of the screen with the monitor name, symbol, signal direction (colored), and price. The toast stays for 8 seconds with a dismiss button. Also plays a subtle sound if enabled in settings.
>
> "Create Monitor" modal: same strategy/param selector as the backtest panel, plus symbol/timeframe, plus notification channel configuration (toggles for in-app, Telegram — shows a warning if Telegram isn't configured yet with a link to Settings).
>
> ---
>
> ## Phase 9 — Telegram Bot & Social Card Generator
>
> **Goal:** Deliver alerts to Telegram and generate shareable visual cards.
>
> **Backend — Notification service (`internal/notification/`):**
>
> `TelegramSender`:
> - Uses the Telegram Bot API (`https://api.telegram.org/bot<token>/sendMessage` and `sendPhoto`)
> - Configured via `user_settings` table (bot token + chat ID)
> - `SendText(chatID, text string)` — for alert messages
> - `SendPhoto(chatID string, imageBytes []byte, caption string)` — for social cards
> - On startup, if a token is configured, send a startup message to the configured chat ID: "✅ StratosMarket is online and monitoring your strategies."
>
> `WebhookSender`:
> - `POST` to the configured URL with JSON body
> - Includes a `X-StratosMarket-Signature` header (HMAC-SHA256 of body + a shared secret) for webhook verification
> - Retries up to 3 times with exponential backoff on failure
>
> **Backend — Social card generator (`internal/social/`):**
> - Uses the `gg` Go library (2D drawing) to generate PNG images
> - `GenerateBacktestCard(run BacktestRun) ([]byte, error)`:
>   - Dark background (or light, based on request param)
>   - Strategy name and tested symbol as headline
>   - Key metrics displayed as large numbers: Total Return, Sharpe Ratio, Max Drawdown, Win Rate
>   - A sparkline of the equity curve drawn as a line chart
>   - StratosMarket branding in the corner
>   - Returns PNG bytes
> - `GenerateSignalCard(signal MonitorSignal, monitor Monitor) ([]byte, error)`:
>   - Shows symbol, signal direction (BUY/SELL in large colored text), price, strategy name, timestamp
>   - Clean minimal design suitable for social sharing
>
> API endpoints:
> ```
> POST /api/social/backtest-card/:runId          → returns PNG image (Content-Type: image/png)
>      query param: ?theme=dark|light
> POST /api/social/signal-card/:signalId         → returns PNG image
> POST /api/social/send-telegram                 → sends a card to Telegram
>      body: { type: "backtest"|"signal", id: "...", theme: "dark" }
>
> GET  /api/settings/notifications               → get Telegram config status
> POST /api/settings/notifications               → save Telegram bot token + chat ID
> POST /api/settings/notifications/test          → send a test Telegram message
> ```
>
> **Frontend:**
> - On any backtest result, show a "Share" button that opens a modal with:
>   - Preview of the generated card (fetched from `/api/social/backtest-card/:id`)
>   - Toggle for dark/light card theme
>   - "Download PNG" button
>   - "Send to Telegram" button (disabled with tooltip if Telegram not configured)
>   - "Copy formatted text" button (generates a text summary for manual posting)
> - Settings page (`/settings`) has a "Notifications" section: Telegram bot token input, chat ID input, "Test Connection" button that sends a test message and shows success/error feedback.
>
> ---
>
> ## Phase 10 — AI Assistant Chatbot
>
> **Goal:** A contextual AI chatbot accessible from every page that understands what the user is looking at and can explain it intelligently.
>
> **Backend — AI service (`internal/ai/`):**
>
> `AIService` with a provider interface:
> ```go
> type AIProvider interface {
>     Complete(ctx context.Context, messages []Message, systemPrompt string) (string, error)
> }
> ```
> Implement two providers: `OpenAIProvider` (uses `gpt-4o-mini` by default, configurable) and `OllamaProvider` (calls local Ollama REST API, model configurable, e.g. `llama3.2`). Provider is selected from `user_settings.ai_provider`.
>
> API endpoint:
> ```
> POST /api/ai/chat
> body: {
>   messages: [{ role: "user"|"assistant", content: "..." }],
>   page_context: {
>     page: "chart"|"backtest"|"portfolio"|"monitor"|"news"|"alerts",
>     symbol: "BTCUSDT",
>     adapter: "binance",
>     timeframe: "1h",
>     active_indicators: ["EMA(9)", "EMA(21)", "RSI(14)"],
>     strategy_name: "EMA Crossover",
>     backtest_metrics: { ... },    // included when on backtest page
>     portfolio_summary: { ... },   // included when on portfolio page
>   }
> }
> → returns: { reply: "...", suggested_questions: ["...", "...", "..."] }
> ```
>
> The backend builds a rich system prompt based on `page_context`. For example, on the chart page:
> ```
> You are a professional trading assistant embedded in StratosMarket, a market analysis platform.
> The user is currently viewing the BTCUSDT chart on Binance, 1-hour timeframe.
> Active indicators: EMA(9), EMA(21), RSI(14).
> Help the user understand technical analysis, interpret what they're seeing, and make sense of the indicators.
> Be concise, educational, and honest about uncertainty. Never give financial advice.
> ```
>
> The `suggested_questions` field returns 3 follow-up questions relevant to the context, helping users who don't know what to ask.
>
> **Frontend — AI chat widget:**
> - Floating circular button in the bottom-right of every page (above the "Create Monitor" FAB if present). Shows a small sparkle/brain icon.
> - Clicking opens a slide-up chat panel (not a full modal — it overlays the bottom ~50% of the screen, the rest of the app remains visible)
> - Chat panel header: "AI Assistant" title + current page context chip (e.g., "BTCUSDT · 1h · Chart") + close button
> - Message list: user messages right-aligned, assistant messages left-aligned. Markdown rendering for assistant messages (use `react-markdown`). Smooth scroll to bottom on new message.
> - Input area: text input + send button. "Enter" to send, "Shift+Enter" for newline.
> - Below the input: 3 suggested question chips. Clicking a chip fills the input with that question.
> - Loading state: typing indicator animation while waiting for response.
> - The page context is automatically captured from the relevant Zustand store when the panel opens. No manual input needed from the user.
> - Chat history is kept in component state for the session (not persisted).
> - Settings page has an "AI" section: provider selector (OpenAI / Ollama), API key input (for OpenAI), Ollama URL input (for Ollama), model name input, "Test Connection" button.
>
> ---
>
> ## Phase 11 — Advanced Analytics
>
> **Goal:** Prevent overfitting and give power users deeper performance insight.
>
> **Backend:**
>
> `GET /api/backtest/runs/:id/param-heatmap`
> - Takes two param names as query params: `?x_param=fast_period&y_param=slow_period`
> - Automatically determines the range (min to max in the param schema, or user-supplied range)
> - Runs the backtest for every combination of the two params (up to 20x20 = 400 combinations)
> - Returns a 2D array of results: `{ x_values, y_values, metric_grid }` where each cell is the Sharpe ratio (default) or user-selected metric
> - Runs combinations in parallel using the worker pool
>
> `POST /api/backtest/runs/:id/monte-carlo`
> - Takes the trade list from a completed run
> - Runs N simulations (default 1000) by randomizing trade order and recomputing equity curves
> - Returns: min/max/median final equity, 5th and 95th percentile equity curves, probability of ruin (equity < 50% of initial)
>
> `GET /api/backtest/runs/:id/walk-forward`
> - Divides the backtest period into N equal windows (default: 5)
> - Runs the strategy on each window independently
> - Returns per-window metrics to reveal if performance is consistent or if it only works in certain periods
>
> `POST /api/backtest/compare`
> - Body: `{ run_ids: [1, 2, 3] }` — list of existing backtest run IDs
> - Returns all their metrics side by side in a single response for easy comparison
>
> **Frontend — Advanced Analytics tab (added to Backtest results):**
> - **Param Heatmap**: two axis-param selectors, then a color-coded grid (red = bad Sharpe, green = good Sharpe). Hovering a cell shows exact params and metric value. Helps users find the "island of good performance" vs. overfit peaks.
> - **Monte Carlo**: "Run Simulation" button triggers the analysis. Shows a fan chart of simulated equity curves (5th–95th percentile shaded, median highlighted). Shows probability of ruin as a stat card.
> - **Walk-Forward**: bar chart of Sharpe ratio per time window. If all bars are positive and roughly equal, the strategy is robust. If only some windows are good, it's overfit.
> - **Compare Runs**: multi-select from previous run history, renders a comparison table with all metrics side by side and a combined equity curve chart with one line per run.
>
> ---
>
> ## Phase 12 — Open Source Polish & CI
>
> **Goal:** Make the repo welcoming, professional, and easy to set up for new contributors.
>
> **Repository files to create:**
> - `README.md`: project banner image (placeholder), description, feature list with checkmarks, architecture diagram (Mermaid), prerequisites, quickstart (`git clone && cp .env.example .env && docker compose up`), environment variables table, adding a new market guide, adding a new strategy guide, screenshots section (placeholders), contributing section, license badge
> - `CONTRIBUTING.md`: code style guide, branch naming, PR template, how to run tests, how to run linter
> - `.github/ISSUE_TEMPLATE/`: bug report template, feature request template
> - `.github/pull_request_template.md`
> - `.github/workflows/ci.yml`: on PR to main — run `go vet`, `golangci-lint`, `go test ./...`, build Docker images, run `eslint`, `tsc --noEmit`, `vitest run`
> - `Makefile` with targets: `make up`, `make down`, `make logs`, `make test`, `make lint`, `make seed`, `make migrate`
> - `docs/architecture.md`: detailed explanation of the adapter pattern, strategy pattern, backtest engine, monitor system, WebSocket architecture
> - `docs/adding-a-market.md`: step-by-step guide with code example
> - `docs/adding-a-strategy.md`: step-by-step guide with code example
>
> **Seed data script** (`make seed`):
> - Fetches 1 year of daily candles for: BTCUSDT, ETHUSDT, SOLUSDT (Binance) and AAPL, MSFT, SPY (Yahoo)
> - Creates a demo portfolio with 3 positions
> - Creates one example monitor (EMA Crossover on BTCUSDT, 1h, in-app notifications only)
> - Creates 2 example alert rules
> - Inserts sample news items
>
> **Paper trading mode** (bonus, implement if time allows):
> - A mode flag on monitors: `mode: "live_alert" | "paper_trade"`
> - In paper trade mode, when a signal fires, it automatically creates a transaction in a designated paper portfolio at the current price
> - Tracks virtual PnL of the strategy running forward in time
> - Shows paper trade results on the monitor detail page
>
> ---
>
> ## Key Constraints (Repeat For Emphasis)
>
> 1. **Pluggable markets**: New exchange = one Go struct implementing `MarketAdapter`, one `init()` registration. Zero other files change.
> 2. **Pluggable strategies**: New strategy = one Go struct implementing `Strategy`. The param schema drives the UI automatically. Zero frontend changes.
> 3. **Local-first data**: TimescaleDB is the source of truth. APIs only fill gaps.
> 4. **WebSocket-first real-time**: Replay, live monitor signals, portfolio PnL, notifications — all over WebSocket. No polling on the frontend except where explicitly noted.
> 5. **Schema-driven UI**: Strategy params, indicator params, alert conditions all render their configuration UI from JSON schema. No hardcoded forms.
> 6. **One command**: `docker compose up` starts everything. No manual DB setup, no separate install steps.
> 7. **AI is context-aware**: The chatbot always knows what page the user is on and what they're looking at. The backend builds the system prompt dynamically.
> 8. **Single user for now**: No authentication system. `user_settings` table has a single row (id=1). Design with future multi-tenancy in mind (every model has a `user_id` column even if unused now).
> 9. **Frontend and UI**: should be very creative island like deisgn which is a card. responsive is a must. monbile and desktop friemdly alwyays 

> ---
>
> **Begin with Phase 1. Generate the complete project scaffold: full folder structure with all files (even if empty stubs), Docker Compose with all four services and health checks, Go module with all dependencies in `go.mod`, React+Vite+TypeScript setup with all npm dependencies in `package.json`, Tailwind and shadcn/ui configured, base layout shell with sidebar and dark mode toggle, and the `.env.example` file with every environment variable the app will ever need. Do not proceed to Phase 2 until confirming Phase 1 is complete and `docker compose up` successfully starts all services.**