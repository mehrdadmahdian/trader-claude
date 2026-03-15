# Frontend Testing Guide

Step-by-step interactive guide to test every feature after logging in. No curl — frontend only.
Work through each section in order. After each step, verify the expected behavior matches what you see.

---

## SECTION 0 — Starting the App

**Step 0.1** — Copy the environment file (first time only).

```bash
make setup
```

**Expect:** A `.env` file is created from `.env.example`. Directories are created. No errors.

---

**Step 0.2** — Start all 4 Docker services (MySQL, Redis, backend, frontend).

```bash
make up
```

**Expect:** Docker pulls/builds images and starts 4 containers. The command returns without error. Run `docker ps` to confirm all 4 are `Up`.

---

**Step 0.3** — Wait ~10 seconds, then verify the backend is healthy.

```bash
make health
```

**Expect:** A JSON response like:

```json
{"status":"ok","db":"ok","redis":"ok","version":"1.0.0"}
```

All three values must be `"ok"`. If `db` or `redis` is not ok, wait a few more seconds and retry — MySQL takes time on first boot.

---

**Step 0.4** — Open the frontend in your browser.

Navigate to: **http://localhost:5173**

**Expect:** The login page loads — a centered card with email + password fields and a Submit button.

---

**Step 0.5** — Register a new account (first time only).

Click **"Don't have an account? Register"** and fill in:
- Display name: any name
- Email: any valid email format
- Password: min 8 characters with uppercase, lowercase, and a digit (e.g. `Test1234`)

Click **Register**.

**Expect:** You are redirected to the Dashboard (`/`).

---

**Step 0.6** — On subsequent runs, just log in.

Enter your email and password, click **Login**.

**Expect:** Redirected to the Dashboard.

---

**Stopping the app** (when done):

```bash
make down        # stop containers, keep data volumes
make down-v      # stop containers AND delete all data (irreversible)
```

**Tailing logs** (for debugging during testing):

```bash
make logs        # all services
```

---

---

## SECTION 1 — Global UI & Navigation

**Step 1.1** — Look at the top header bar.

**Expect:** A fixed horizontal bar with:
- Logo/brand on the left
- Navigation links: Dashboard, Chart, Backtest, Portfolio, Monitor, News, Alerts, Settings, Terminal
- A symbol search/picker (showing something like `BTC/USDT`)
- Timeframe buttons or dropdown
- A theme toggle icon (sun/moon)
- A bell icon for notifications
- A user avatar or name with a dropdown

---

**Step 1.2** — Click the **theme toggle** (sun/moon icon).

**Expect:** The entire page background switches between dark (near-black) and light (white/gray). Click again to toggle back. The preference persists — if you refresh the page, the theme stays the same.

---

**Step 1.3** — Click the **bell icon** (notifications).

**Expect:** A dropdown appears showing recent notifications. Each has a title, body text, type badge (alert/trade/system/backtest), and relative time (e.g. "2 hours ago"). There may be an unread count badge on the bell.

---

**Step 1.4** — Click your **username/avatar** in the top right.

**Expect:** A dropdown with your display name, role, and a **Logout** button.

---

## SECTION 2 — Dashboard (`/`)

**Step 2.1** — Navigate to **Dashboard** (first nav item or `/`).

**Expect:** A 3-column layout:
- Left rail: Watchlist panel + Portfolio summary panel
- Center: A candlestick chart area (empty or with data)
- Right rail: News feed panel + Alerts feed panel

---

**Step 2.2** — In the CommandBar symbol picker, click on the symbol input. Type `ETH`.

**Expect:** A dropdown list of symbols containing "ETH" appears (filtered in real time). Options like `ETH/USDT`, `ETH/BTC`, etc.

---

**Step 2.3** — Select **ETH/USDT** (or any symbol from the list).

**Expect:** The center candlestick chart loads and displays candle data for that symbol. The chart shows green/red candlesticks on a price axis with a time axis at the bottom.

---

**Step 2.4** — Change the **timeframe** (e.g. click `1h` then `1d`).

**Expect:** The chart reloads with the new timeframe. Candle sizes and the time range change (1d shows fewer, wider candles; 1m shows many narrow ones).

---

## SECTION 3 — Chart Page (`/chart`)

**Step 3.1** — Click **Chart** in the navigation.

**Expect:** Full-width page with:
- Toolbar at top: adapter selector, symbol search input, timeframe buttons (1m 5m 15m 30m 1h 4h 1d 1w), Indicators button, News toggle button, Refresh button
- Large candlestick chart below

---

**Step 3.2** — Click the **adapter dropdown** (should show "Binance" or "Yahoo").

**Expect:** A dropdown with available adapters (Binance, Yahoo). Switching adapter resets the symbol.

---

**Step 3.3** — Click the **symbol search input**, type `BTC`.

**Expect:** A dropdown list appears with matching symbols (ticker + description). Click one to select it — the chart loads data for that symbol.

---

**Step 3.4** — Click the **Indicators** button.

**Expect:** A modal opens showing available technical indicators (e.g. SMA, EMA, RSI, MACD, Bollinger Bands). Each has a name and configurable parameters.

---

**Step 3.5** — Inside the Indicators modal, select **SMA**. Set a period (e.g. 20). Click Add.

**Expect:** The modal closes. A chip appears in the toolbar showing "SMA(20)" with an X. On the chart, an overlay line appears tracking the moving average over the candles.

---

**Step 3.6** — Click the **X** on the SMA chip.

**Expect:** The SMA line disappears from the chart. The chip is removed from the toolbar.

---

**Step 3.7** — Open Indicators again and add **RSI** (period 14).

**Expect:** RSI is a panel indicator — a separate chart panel appears *below* the main candlestick chart, showing the RSI oscillating between 0 and 100, with overbought/oversold lines at 70 and 30.

---

**Step 3.8** — Click the **News toggle** button in the toolbar.

**Expect:** A right-side panel slides open showing news articles related to the selected symbol. Each article has a headline, source, and time. Click it again to hide the panel.

---

**Step 3.9** — Click the **Refresh** button.

**Expect:** The chart briefly shows a loading state and candle data reloads from the API.

---

## SECTION 4 — Backtest Page (`/backtest`)

**Step 4.1** — Click **Backtest** in the navigation.

**Expect:** A multi-section page with:
- Left: A form to configure a new backtest + list of past runs below it
- Right: Metrics panel + equity curve chart + status badge

---

**Step 4.2** — In the backtest form, click the **Strategy** dropdown.

**Expect:** A list of available strategies appears (e.g. SMA Cross, RSI Strategy, etc.).

---

**Step 4.3** — Select a strategy. Fill in: symbol, market, timeframe, start date, end date.

**Expect:** When you select a strategy, parameter fields appear below (e.g. "Fast Period", "Slow Period" for SMA Cross). Fill them with valid numbers.

---

**Step 4.4** — Click **Run Backtest**.

**Expect:** A new entry appears in the backtest runs list below the form. The status badge shows "running" or "pending" with a progress indicator. After a few seconds it transitions to "completed".

---

**Step 4.5** — Click on the **completed backtest run** in the list.

**Expect:** The right panel populates with:
- Metrics table: Sharpe Ratio, Total Return %, Max Drawdown %, Win Rate, etc.
- An equity curve chart (area/line chart showing portfolio value over time)

---

**Step 4.6** — Click the **Play** button (replay control).

**Expect:** A replay mode activates. The chart replays candles one by one. A control bar appears with Play/Pause, Step Forward, and Speed controls. Trade markers appear on the chart at entry/exit points.

---

**Step 4.7** — Click **Pause**, then **Step** forward one candle at a time.

**Expect:** Each Step click advances the chart by one candle. The equity mini-chart updates to reflect the current equity at that point in time.

---

**Step 4.8** — While replaying, click the **Bookmark** button (if visible).

**Expect:** A modal opens asking for a label and optional note. After saving, the bookmark is stored and associated with that candle index in the replay.

---

## SECTION 5 — Portfolio Page (`/portfolio`)

**Step 5.1** — Click **Portfolio** in the navigation.

**Expect:** A page with:
- A portfolio selector dropdown at the top
- If no portfolio: an empty state with a "Create Portfolio" button
- If portfolio exists: Summary cards (Total Value, P&L, Cash) + positions table + allocation donut chart

---

**Step 5.2** — If no portfolio exists, create one. Otherwise select your portfolio from the dropdown.

**Expect:** After selecting, summary cards load with numeric values. The left side shows a table of positions. The right side shows a donut chart with each symbol's allocation %.

---

**Step 5.3** — Hover over a **slice in the donut chart**.

**Expect:** That slice highlights (grows/pops out), and the corresponding row in the positions table highlights as well.

---

**Step 5.4** — Click **Add Position** button.

**Expect:** A modal opens with fields: symbol, quantity, average cost, market. Fill these in and submit.

**Expect after submit:** The modal closes, the new position appears in the table, and the donut chart updates to include the new allocation.

---

**Step 5.5** — Click the **edit icon** on an existing position.

**Expect:** A modal opens pre-filled with the position's current data. Edit a value and save.

**Expect after save:** The position row updates with new values.

---

**Step 5.6** — At the bottom, click the **Equity Curve** tab.

**Expect:** A line chart showing your portfolio's total value over time.

---

**Step 5.7** — Click the **Transactions** tab.

**Expect:** A table listing all transactions: type (Buy/Sell/Deposit/Withdrawal), symbol, amount, price, date.

---

## SECTION 6 — Monitor Page (`/monitor`)

**Step 6.1** — Click **Monitor** in the navigation.

**Expect:** A grid of monitor cards (or an empty state with "Create Monitor" CTA).

---

**Step 6.2** — Click **Create Monitor**.

**Expect:** A modal with fields: adapter (Binance/Yahoo), symbol, market, timeframe, strategy, mode (Live/Paper), notify in-app toggle.

---

**Step 6.3** — Fill the form (e.g. Binance, BTC/USDT, spot, 1h, any strategy, Paper mode, notify enabled). Submit.

**Expect:** Modal closes. A new monitor card appears in the grid showing:
- Green pulsing dot (active)
- Monitor name + strategy badge
- Symbol/timeframe/adapter
- "Last polled" time

---

**Step 6.4** — Click the **Pause** button on the monitor card.

**Expect:** The green dot disappears or turns gray. The status changes to "paused". The Pause button becomes a Play button.

---

**Step 6.5** — Click the **Play** button to resume.

**Expect:** Status returns to active with green dot.

---

**Step 6.6** — Click the **expand/signals** button on the monitor card.

**Expect:** The card expands to show a signal history table with columns: Time, Direction (Buy/Sell), Price, Strength %. If no signals yet, the table is empty.

---

**Step 6.7** — Click **Delete** on the monitor.

**Expect:** A confirmation dialog appears. Confirm it. The monitor card is removed from the grid.

---

## SECTION 7 — Alerts Page (`/alerts`)

**Step 7.1** — Click **Alerts** in the navigation.

**Expect:** A page with a table of alerts (or empty state). Table columns: Name, Symbol, Condition, Threshold, Status, Last Fired, Actions.

---

**Step 7.2** — Click **New Alert**.

**Expect:** A modal with fields: name, adapter (Binance/Yahoo), symbol, condition dropdown, threshold, recurring toggle, cooldown minutes.

---

**Step 7.3** — Change the **Condition** dropdown between options (price_above, price_below, price_change_pct).

**Expect:** A hint text below the condition field updates to explain what each condition means.

---

**Step 7.4** — Fill the form (e.g. name="BTC High Alert", Binance, BTC/USDT, price_above, threshold=100000, non-recurring). Submit.

**Expect:** Modal closes. The new alert appears in the table with status "active".

---

**Step 7.5** — Click the **toggle** (enable/disable) on the alert.

**Expect:** The status badge changes between "active" (green) and "disabled" (gray).

---

**Step 7.6** — Click **Delete** on the alert.

**Expect:** The alert row is removed from the table.

---

## SECTION 8 — Notifications Page (`/notifications`)

**Step 8.1** — Click the bell icon in the header, then navigate to `/notifications`.

**Expect:** A paginated list of notifications. Each item shows:
- Blue filled dot (unread) or empty circle (read)
- Title + type badge (alert/trade/system/backtest in different colors)
- Body text
- Relative time ("2 hours ago")
- A "Read" button if unread

---

**Step 8.2** — Click the **Read** button on an unread notification.

**Expect:** The blue dot changes to an empty circle. The "Read" button disappears. The unread count in the header bell decreases by 1.

---

**Step 8.3** — Click **Mark all read** at the top.

**Expect:** All blue dots become empty circles. All "Read" buttons disappear. The unread count badge on the bell icon resets to 0.

---

**Step 8.4** — If there are more than 20 notifications, click **Next** page.

**Expect:** A new set of notifications loads (the next 20). The page indicator shows "Page 2 of N". Click "Previous" to go back.

---

## SECTION 9 — Settings Page (`/settings`)

**Step 9.1** — Click **Settings** in the navigation.

**Expect:** Two sections:
1. **Notifications** — Telegram config + Webhook config
2. **AI Assistant** — Provider selector + credentials

---

**Step 9.2** — In the Notifications section, toggle **Telegram enabled**.

**Expect:** The Bot Token and Chat ID fields appear/disappear based on the toggle state.

---

**Step 9.3** — Enter a fake Bot Token and Chat ID. Click **Test Connection**.

**Expect:** A message appears (success showing bot name, or error message explaining the failure).

---

**Step 9.4** — In the AI Assistant section, click the **Provider** dropdown.

**Expect:** Options: OpenAI and Ollama. Selecting OpenAI shows an API Key field. Selecting Ollama shows an Ollama URL field.

---

**Step 9.5** — Select **OpenAI**, enter a fake API key, enter a model name (e.g. `gpt-4o`). Click **Save Settings**.

**Expect:** A success message appears briefly: "Settings saved" or similar.

---

**Step 9.6** — Click **Test Connection** for AI.

**Expect:** Either a success message (if key is valid) or an error message explaining the connection failure.

---

## SECTION 10 — AI Chat Panel

**Step 10.1** — Look for a floating **AI button** (bottom right corner of the screen, likely a chat bubble icon).

**Expect:** A circular button in the bottom-right corner.

---

**Step 10.2** — Click the AI button.

**Expect:** A side panel slides in from the right with a chat interface — an input box at the bottom and message history above.

---

**Step 10.3** — Type a message: `What is the current market trend?` and press Enter or click Send.

**Expect:** Your message appears in the chat. The AI responds (if AI settings are configured) or shows an error/placeholder if no provider is configured.

---

**Step 10.4** — Click the AI button again (or an X) to close.

**Expect:** The chat panel slides closed.

---

## SECTION 11 — Terminal Page (`/terminal`)

**Step 11.1** — Click **Terminal** in the navigation.

**Expect:** A completely different layout — no sidebar, full-screen Bloomberg-style workspace. Tabs at the top for different workspaces. A grid of draggable/resizable panels.

---

**Step 11.2** — Try **dragging a panel** by its header to a new position.

**Expect:** The panel moves and the layout updates. Other panels reflow around it.

---

**Step 11.3** — Try **resizing a panel** by dragging its corner/edge.

**Expect:** The panel resizes. Content inside adjusts.

---

## SECTION 12 — Auth Flow (final check)

**Step 12.1** — Click your **username dropdown** → **Logout**.

**Expect:** You are redirected to `/login`. The login form appears.

---

**Step 12.2** — Try navigating directly to `/dashboard` (type it in the URL bar).

**Expect:** You are immediately redirected back to `/login` — the protected route guard is working.

---

**Step 12.3** — Log back in with your credentials.

**Expect:** On successful login, you are redirected to `/` (Dashboard). Your theme preference is preserved.

---

## Coverage Summary

| Section | Feature | Steps |
|---------|---------|-------|
| 1 | Global UI & Navigation | 1.1 – 1.4 |
| 2 | Dashboard | 2.1 – 2.4 |
| 3 | Chart (indicators, news, refresh) | 3.1 – 3.9 |
| 4 | Backtest (run, replay, bookmark) | 4.1 – 4.8 |
| 5 | Portfolio (positions, donut, tabs) | 5.1 – 5.7 |
| 6 | Monitor (create, pause, signals, delete) | 6.1 – 6.7 |
| 7 | Alerts (create, toggle, delete) | 7.1 – 7.6 |
| 8 | Notifications (read, mark all, paginate) | 8.1 – 8.4 |
| 9 | Settings (Telegram, Webhook, AI) | 9.1 – 9.6 |
| 10 | AI Chat Panel | 10.1 – 10.4 |
| 11 | Terminal (drag, resize) | 11.1 – 11.3 |
| 12 | Auth flow (logout, guard, login) | 12.1 – 12.3 |
