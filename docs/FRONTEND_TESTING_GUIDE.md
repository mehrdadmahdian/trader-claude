# trader-claude — Frontend UI Testing Guide

> Step-by-step browser walkthrough for every implemented page.
> Open **http://localhost:5173** after `make up`.

---

## Prerequisites

Services must be running:
```bash
make up
make health   # should return {"status":"ok","db":"ok","redis":"ok",...}
```

Browser: any modern browser. Open DevTools Console (F12) and keep it visible — network errors and WS messages appear there.

---

## Page 1 — Register (`/register`)

**Get there:** Go to http://localhost:5173/register (you are redirected here automatically if not logged in)

**Steps:**
1. Fill in **Email**, **Password** (min 8 chars, must include uppercase + digit + symbol), and **Display Name**
2. Click **Register**
3. You are redirected to `/` (Dashboard) automatically

**Check:**
- No error message shown → registration succeeded
- You are logged in (sidebar and top bar are visible)

**Error cases to verify:**
- Leave email blank and click Register → form validation prevents submission
- Enter a weak password (e.g. `abc`) → error message appears
- Register again with the same email → error `email already registered` appears

---

## Page 2 — Login (`/login`)

**Get there:** Click **Sign out** in the sidebar (or visit http://localhost:5173/login directly)

**Steps:**
1. Enter registered email and password
2. Click **Sign In**
3. You are redirected to `/` (Dashboard)

**Check:**
- Dashboard loads with sidebar showing navigation items
- Top bar shows your display name or email

**Error cases to verify:**
- Wrong password → error `invalid email or password`
- Hit Login 6 times in under 60 seconds → `too many login attempts` error (rate limiter)
- Leave email blank → browser validation fires before network request

---

## Page 3 — Dashboard (`/`)

**Get there:** Click the logo or **Dashboard** in the sidebar

**Layout:**
- **Left rail** — Watchlist panel (top) + Portfolio summary panel (bottom)
- **Center** — Candlestick chart (empty state until a symbol is selected)
- **Right rail** — News feed panel (top) + Alerts feed panel (bottom)

**Steps:**
1. The chart shows an empty state: *"Select a symbol to view chart"*
2. In the **Top Bar**, find the symbol picker (e.g. `BTC/USDT` or `Search symbol…`) and select any symbol
3. The center chart loads with candlestick data
4. In the **Watchlist panel** (left), click **+ Add** to add a symbol to your watchlist
5. Confirm the added symbol appears in the list
6. The **Portfolio summary panel** shows totals (all zeros if no portfolio exists yet)
7. The **News feed panel** (right) shows news headlines (may be empty if no news ingested)
8. The **Alerts feed panel** (right bottom) shows recent fired alerts

**Check:**
- Chart renders candles when a symbol is selected
- Timeframe selector in the top bar changes the visible candle range
- Dark/light mode toggle (top-right) switches the theme; refreshing the page persists the theme

---

## Page 4 — Chart (`/chart`)

**Get there:** Click **Chart** in the sidebar

**Toolbar (left to right):**
- Adapter dropdown (`Binance`, `Yahoo Finance`)
- Symbol search button
- Timeframe pill buttons (`1m 5m 15m 30m 1h 4h 1d 1w`)
- **Indicators** button
- **News** button
- **Refresh** button (right end)

**Steps — Basic chart:**
1. Click the **adapter dropdown** and confirm Binance is selected
2. Click the **symbol search button** → a dropdown appears with a text input
3. Type `BTC` → list filters to matching symbols
4. Click a symbol (e.g. `BTCUSDT`) → chart loads
5. Click each **timeframe button** in sequence (1h, 4h, 1d) → chart range updates each time
6. Click **Refresh** → spinner animates and chart re-fetches

**Steps — Indicators:**
1. Click the **Indicators** button → modal opens listing available indicators (EMA, SMA, RSI, MACD, BB, etc.)
2. Click an **overlay** indicator (e.g. EMA) → configure period in the form → click Add
3. The indicator chip appears in the toolbar and the line is plotted on the chart
4. Click a **panel** indicator (e.g. RSI) → add it → a separate panel appears below the chart
5. Click the **×** on an indicator chip → it is removed from chart and panel disappears
6. Change timeframe → indicators are recalculated automatically for the new candle range
7. Reload the page → indicators for the selected symbol+timeframe are restored from `localStorage`

**Steps — News side panel:**
1. With a symbol selected, click the **News** button → a side panel slides in on the right
2. News articles for the selected symbol are listed
3. Click **News** again → panel closes

**Error states:**
- Select a symbol with no candle data → chart shows *"Failed to load candles"* with a **Retry** button
- Click Retry → re-fetches

---

## Page 5 — Backtest (`/backtest`)

**Get there:** Click **Backtest** in the sidebar

**Layout:**
- **Left panel** — Run configuration form + run history list
- **Right panel** — Results (metrics, equity chart, trades table) or Analytics tab

**Steps — Run a backtest:**
1. In the left panel:
   - Select an **Adapter** from the dropdown (e.g. Binance)
   - Select a **Market** (e.g. crypto)
   - Select a **Symbol** (e.g. BTCUSDT)
   - Select a **Timeframe** (e.g. 1h)
   - Choose a **Strategy** from the card grid (cards show strategy name and description)
   - Adjust any **strategy params** that appear below (sliders/inputs/toggles)
   - Set **Start date** and **End date**
   - Set **Initial capital**
2. Click **Run Backtest**
3. A new entry appears in the run history list below with a **Pending** badge (yellow clock icon)
4. Badge changes to **Running** (blue spinning loader)
5. A progress bar appears in the right panel, filling as the backtest executes
6. When done, badge changes to **Completed** (green checkmark)

**Steps — View results:**
1. Click on a **Completed** run in the history list (left)
2. Right panel shows:
   - Metric cards: Total Return, Sharpe Ratio, Max Drawdown, Win Rate, Total Trades
   - Equity curve area chart (time → capital value)
   - Trades table with columns: Entry, Exit, Direction, P&L
3. Verify metric cards show numeric values (colorized green/red for return/drawdown)

**Steps — Analytics tab:**
1. With a completed run selected, click the **Analytics** tab (right panel tabs)
2. Sub-tabs: **Heatmap**, **Monte Carlo**, **Walk-Forward**, **Compare**
3. **Heatmap tab** → grid of colored cells showing metric across param combinations
4. **Monte Carlo tab** → click **Run Monte Carlo** button → spinner while calculating → fan chart of 100+ equity curve simulations appears
5. **Walk-Forward tab** → chart showing in-sample vs out-of-sample periods
6. **Compare tab** → select another completed run from the dropdown → split metrics table appears

**Steps — Replay:**
1. With a completed run selected (right panel), click the **Replay** button (film icon)
2. A replay overlay appears on the chart with a control bar at the bottom:
   - Play / Pause
   - Speed selector
   - Progress scrubber
3. Press **Play** → candles animate forward in time; trade markers appear on the chart
4. Press **Pause** → animation freezes
5. Click the **Bookmark** icon → Bookmark modal opens → enter a label → Save
6. Bookmark appears in the bookmarks list below the control bar

**Steps — Share:**
1. With a completed run selected, click the **Share** icon (share/export icon)
2. Share modal opens with options: **Copy link**, **Telegram**
3. (Telegram requires settings configured — see Settings page)

**Steps — Delete a run:**
1. Hover over a run in the history list
2. Click the **trash** icon → confirm dialog → run is removed from the list

---

## Page 6 — Portfolio (`/portfolio`)

**Get there:** Click **Portfolio** in the sidebar

**Steps — Create a portfolio:**
1. Click the **portfolio selector** dropdown at the top right → click **+ New Portfolio**
2. Modal opens: enter **Name** (e.g. "My Paper Portfolio"), choose **Type** (paper/live), set **Initial Capital** and **Currency**
3. Click **Create** → modal closes, portfolio is selected in the dropdown

**Steps — Add a position:**
1. In the **Positions table** (center), click **+ Add Position**
2. Modal opens: enter **Symbol** (e.g. BTC/USDT), **Quantity**, **Avg Cost**, **Market**
3. Click **Save** → position row appears in the table

**Steps — Positions table interactions:**
1. Hover over a row in the positions table → row highlights
2. Click the **Edit** icon → modal opens pre-filled → change quantity → Save
3. Hover a row → hover state in the **Allocation Donut** chart (right) highlights that slice
4. Click the **Delete** icon on a position row → confirm → row is removed

**Steps — Allocation donut:**
1. The donut on the right shows each position as a proportional slice
2. Hover a slice → the corresponding row in the positions table highlights

**Steps — Summary cards:**
1. At the top of the page, 4 metric cards appear: **Total Value**, **Total Invested**, **P&L**, **Return %**
2. Positive P&L is green, negative is red
3. Cards refresh every 30 seconds (or immediately after a position change)

**Steps — Equity Curve tab:**
1. Click the **Equity Curve** tab below the positions area
2. A line chart shows portfolio value over time
3. If no transactions exist, the chart is flat at initial capital

**Steps — Transactions tab:**
1. Click the **Transactions** tab
2. A table lists all buy/sell/deposit transactions with date, type, symbol, quantity, price, fee
3. If empty, a placeholder message appears

---

## Page 7 — Monitor (`/monitor`)

**Get there:** Click **Monitor** in the sidebar

**Empty state:**
- Large lightning bolt icon with "No monitors yet" and a **Create your first monitor** button

**Steps — Create a monitor:**
1. Click **Create Monitor** (top right or empty-state button)
2. Modal opens with fields:
   - **Adapter** — select `Binance` or `Yahoo Finance`
   - **Symbol** — type `BTCUSDT` (or any valid symbol)
   - **Market** — select `Crypto`, `Stock`, `ETF`, `Forex`
   - **Timeframe** — click a pill: `1m 5m 15m 1h 4h 1d`
   - **Strategy** — grid of strategy cards; click one to select
   - **Name** — optional (auto-generated if left blank)
   - **Mode** — `Live Alert` (alerts only) or `Paper Trade` (auto-executes trades)
   - **In-app notifications** — checkbox
3. Click **Create Monitor** → modal closes, monitor card appears in the grid

**Steps — Monitor card interactions:**
1. Card shows: name, symbol/timeframe/adapter, mode badge (PAPER or LIVE), pulsing green dot (active) or grey dot (paused)
2. If a signal has fired: shows direction (LONG/SHORT), price, and "X time ago"
3. If no signal yet but monitor is active: shows last polled time
4. Click **Pause** button on the card → green dot becomes grey, button changes to **Resume**
5. Click **Resume** → monitor restarts
6. Click the **Signals** chevron at bottom right of card → signal history table expands below the card
7. Signal history shows: timestamp, direction badge (green LONG / red SHORT), price, strength %
8. If >20 signals, **Prev / Next** pagination appears
9. Click the **trash** icon → confirm dialog → card removed

**Real-time signals:**
- Active monitors subscribe to the signals WebSocket automatically
- When a strategy fires, a **toast notification** appears in the bottom-left of the screen (SignalToast component) showing: symbol, direction, price

---

## Page 8 — Alerts (`/alerts`)

**Get there:** Click **Alerts** in the sidebar

**Header shows:** "Price alerts — evaluated every 60 seconds"

**Steps — Create an alert:**
1. Click **New Alert** button (top right)
2. Modal opens with fields:
   - **Name** — e.g. "BTC above 100k"
   - **Adapter** — `Binance (Crypto)` or `Yahoo Finance (Stocks)`
   - **Symbol** — e.g. `BTCUSDT`
   - **Condition** — dropdown: `Price Above`, `Price Below`, `Price Change %`
   - **Threshold** — numeric input ($ amount or % depending on condition)
   - **Recurring** toggle — whether alert re-fires after cooldown
   - **Cooldown (minutes)** — visible only if Recurring is ON (default 60)
3. Click **Create Alert** → modal closes, alert row appears in the table

**Steps — Alerts table:**
1. Columns: Name, Symbol, Condition, Threshold, Status, Last Fired, Actions
2. Status badge: green `active`, yellow `triggered`, grey `disabled`
3. **Toggle icon** (right column): click to enable/disable alert → badge color changes
4. **Trash icon** (right column): click to delete alert → row removed immediately

**Note:** Alerts are evaluated every 60 seconds on the server. You won't see "triggered" in UI testing unless a real price feed is connected and the condition is actually met.

---

## Page 9 — News (`/news`)

**Get there:** Click **News** in the sidebar

**Layout:** Card grid of news articles

**Steps:**
1. Page loads a list of news articles (empty if none have been ingested)
2. Each card shows: headline, source, timestamp, symbol tag
3. Click a card → opens the article URL in a new tab (or shows detail, depending on implementation)
4. Use the symbol filter at the top (if present) to filter by symbol

**Note:** News data is populated by the backend's news ingestion. With a fresh database and no news adapter running, this page will show an empty state.

---

## Page 10 — Notifications (`/notifications`)

**Get there:** Click the bell icon in the sidebar (shows unread badge) or click **Notifications** nav item

**Steps:**
1. Page shows a list of notifications (type, message, timestamp, read/unread indicator)
2. Unread notifications have a highlighted background
3. Click a notification row → it is marked as read (background clears)
4. Click **Mark all read** button (top right) → all notifications clear their unread state
5. The badge counter in the sidebar updates to 0

**Real-time delivery:**
- Keep this page open in one browser tab
- In another tab, trigger an action that creates a notification (e.g. a backtest completes, or an alert fires)
- The new notification should appear in the list without a page refresh (WebSocket push)

---

## Page 11 — Settings (`/settings`)

**Get there:** Click **Settings** (gear icon) in the sidebar

**Two sections:**

### Notifications Section

**Telegram subsection:**
1. Toggle the **Telegram** button from Disabled → Enabled
2. Enter a **Bot Token** (format: `123456:ABC...`)
3. Enter a **Chat ID** (your Telegram user or group ID)
4. Click **Save Settings** → green *"Settings saved successfully!"* message appears for 3 seconds
5. Click **Test Connection** → green success message with bot name, or red error message

**Webhook subsection:**
1. Toggle **Webhook** → Enabled
2. Enter a **Webhook URL** (e.g. `https://example.com/hook`)
3. Enter an optional **Secret**
4. Click **Save Settings**

**Error states:**
- Click **Test Connection** without Telegram or Webhook enabled → button is disabled (greyed out)

### AI Assistant Section

1. **Provider** dropdown — select `OpenAI` or `Ollama`
2. If **OpenAI** selected: enter **API Key** (`sk-...`) and **Model** (e.g. `gpt-4`)
   - If a key is already saved, a `Key saved ✓` label appears next to the field; leave blank to keep existing key
3. If **Ollama** selected: enter **Ollama URL** (default `http://localhost:11434`) and **Model** (e.g. `llama2`)
4. Click **Save Settings** → green success or red error message
5. Click **Test Connection** → tests connectivity to the configured AI provider → result message appears

---

## Page 12 — AI Chat (floating button)

**Get there:** A floating **AI** button appears in the bottom-right of any page (Layout)

**Steps:**
1. Click the floating **AI** button → chat panel slides in from the right
2. AI settings must be configured first (see Settings page)
3. Type a message (e.g. *"What is EMA crossover strategy?"*) and press Enter or click Send
4. AI response streams in below the message
5. Click the X to close the chat panel

**Note:** If AI settings are not configured, the send button is disabled or an error is shown.

---

## Page 13 — Terminal / Bloomberg Workspace (`/terminal`)

**Get there:** Click **Terminal** in the sidebar

**Layout:**
- **Top bar** (CommandBar) — workspace actions, add panel button
- **Workspace tabs** — tab strip with workspace names, `+` to add workspace
- **Panel grid** — react-grid-layout canvas

**Steps — Workspace management:**
1. On first visit the grid is empty
2. Click **+ New Workspace** (or the `+` tab) → a new workspace tab appears
3. Double-click a workspace tab to rename it
4. Click **×** on a tab to delete that workspace

**Steps — Adding panels:**
1. Click **Add Panel** in the CommandBar → a panel selector dropdown or modal appears
2. Choose a panel type from the list:
   - `Chart` — candlestick chart widget
   - `Watchlist` — watchlist widget
   - `Portfolio` — portfolio summary widget
   - `News` — news feed widget
   - `Alerts` — alerts widget
   - `Backtest` — backtest run summary widget
   - `Monitor` — monitor signals widget
   - `AI Chat` — AI chat widget
3. The panel appears in the grid at a default position and size

**Steps — Panel interactions:**
1. **Drag** a panel by its header (drag handle) → moves to new position
2. **Resize** a panel by dragging the bottom-right corner handle → panel grows/shrinks
3. Click inside a panel (it activates with a highlighted border)
4. Click **×** on a panel header → panel is removed from the grid
5. **Chart panel** — select adapter/symbol/timeframe inside the panel just like the Chart page
6. **Watchlist panel** — add/remove symbols within the panel

**Steps — Layout persistence:**
1. Add and arrange several panels
2. Reload the page (`F5`)
3. The same workspace layout and panel positions should be restored (persisted to backend via `/api/v1/workspaces`)

**Steps — Multiple workspaces:**
1. Click `+` to add a second workspace
2. Add different panels to workspace 2
3. Click back to workspace 1 → its layout is shown
4. Click workspace 2 → its layout is shown
5. Each workspace is independent

---

## Global UX Checks (do these on any page)

### Dark / Light Mode
1. Click the sun/moon icon in the top bar
2. The entire UI switches theme (dark ↔ light)
3. Reload → theme is persisted

### Sidebar Collapse
1. Click the collapse arrow on the sidebar
2. Sidebar collapses to icon-only mode
3. Click again → expands back
4. State persists across page navigation

### Token Expiry
1. In localStorage (DevTools → Application → Local Storage), delete `access_token`
2. Reload any page
3. You are redirected to `/login`

### Protected Routes
1. While logged out, navigate directly to `http://localhost:5173/`
2. You are redirected to `/login`
3. After logging in, you are redirected back to the original page

---

## What Will Show Empty Without Live Data

These features require a market data adapter (Binance/Yahoo) to be actively ingesting data. With a fresh install, they will show empty or stub data:

| Page / Feature | Reason |
|---|---|
| Chart candles | No candles ingested — candle API returns empty array |
| Dashboard chart | Same as above |
| News page | No news adapter feeding data |
| Monitor signals | No live feed → no signals generated |
| Alerts "triggered" | Need real price ticks to evaluate conditions |
| Portfolio P&L | Need price feed for real-time mark-to-market |

This is expected. The backend scaffold and all UI are ready; data flows in once adapters are wired up in Phase 2.
