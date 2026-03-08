# Bloomberg Terminal Clone — Design Document

**Date:** 2026-03-06
**Status:** Approved
**Scope:** Full replacement of the existing page-based UI with a Bloomberg-style panel workspace

---

## 1. Vision

Replace the current sidebar-nav + page paradigm with a Bloomberg Terminal-style workspace where:

- The entire screen is a **resizable, draggable grid of panels**
- Each panel independently loads any feature module (chart, news, screener, fundamentals, etc.)
- Navigation is driven by a **command bar** at the top: `<TICKER> <FUNCTION>` (e.g. `BTC GP`, `AAPL FA`)
- Users save/load multiple **workspace layouts** as tabs
- Panels can be **linked** so changing the ticker in one panel updates all linked panels

**Asset classes covered:** Crypto, Equities, Commodities (no FX)
**Data sources:** Free APIs only (Yahoo Finance, Alpha Vantage, FRED, CoinGecko, Polygon.io free)

---

## 2. Architecture Decision

**Approach chosen: Additive — Bloomberg alongside existing app**

Two distinct tools in one codebase. Existing routes are fully preserved:

```
/              → Old app: trading workbench (Layout.tsx, sidebar nav)
/chart         → Chart page (unchanged)
/backtest      → Backtest page (unchanged)
/portfolio     → Portfolio page (unchanged)
/monitor       → Monitor page (unchanged)
/news          → News page (unchanged)
/alerts        → Alerts page (unchanged)
/terminal      → Bloomberg workspace (WorkspaceLayout — NEW)
/login         → authentication (unchanged)
/register      → registration (unchanged)
```

**Why two apps:**
- **Old app** = active trading workbench: running backtests, configuring strategies, setting alerts, managing monitors
- **Bloomberg terminal** = research & market viewer: panel-based data exploration, fundamentals, screener, charting, news

A "Terminal" link is added to the existing sidebar nav to switch between modes. Existing Layout.tsx and all page components are untouched.

---

## 3. Panel Builder System

### Grid Engine

- Library: `react-grid-layout` (MIT, 12-column responsive grid)
- Panels are positioned by `{x, y, w, h}` grid coordinates
- Drag, resize, and rearrange panels freely
- Layout serializes to JSON and persists to backend DB

### Panel Anatomy

```
+------------------------------------------+
| [TICKER BADGE] [FUNCTION LABEL] ........  [🔗][⛶][✕] |
+------------------------------------------+
|                                          |
|           Panel Content Area            |
|                                          |
+------------------------------------------+
```

**Header controls:**
- `🔗` — Link group selector (color-coded: red, blue, green, yellow). Panels in the same group share ticker context.
- `⛶` — Maximize panel to full workspace (press again to restore)
- `✕` — Close panel

### Panel Linking

Panels share a link color group. When the ticker changes in any panel of a group, all other panels in that group re-render with the new ticker. Allows e.g.: chart + news + fundamentals all tracking `AAPL` simultaneously.

### Workspaces

Multiple workspace tabs appear across the top of the screen:

```
[WS: Market Overview ▾] [WS: Trader ▾] [WS: Crypto ▾] [+]
```

- Each workspace stores: panel layout JSON + panel configs (function, ticker, params)
- Workspaces saved per-user in backend DB (`Workspace` model)
- Pre-built starter templates: **Market Overview**, **Trader**, **Crypto**, **Analyst**
- Workspaces restore fully on login

---

## 4. Command Bar

Single input at the top of the workspace. Always visible. Format:

```
>_ BTC GP
```

### Function Codes

| Code | Panel | Description |
|------|-------|-------------|
| `GP` | Chart | Candlestick chart with indicators |
| `HM` | Heatmap | Market heatmap (treemap, color = % change) |
| `FA` | Fundamentals | P/E, EPS, revenue, balance sheet; crypto supply/dominance |
| `NEWS` | News | Asset-specific news feed |
| `PORT` | Portfolio | Portfolio positions, PnL, transactions |
| `WL` | Watchlist | Multi-column watchlist with real-time prices |
| `SCR` | Screener | Filter builder across all asset classes |
| `CAL` | Calendar | Earnings dates + macro events (Fed, CPI, GDP) |
| `OPT` | Options Chain | Put/call chain with Greeks and IV for stocks |
| `YCRV` | Yield Curves | US Treasury yield curves + sovereign spreads |
| `RISK` | Risk Analytics | VaR, Sharpe/Sortino, drawdown, stress tests |
| `BT` | Backtest | Strategy backtest runner + results |
| `ALRT` | Alerts | Alert rules manager |
| `MON` | Monitor | Live strategy monitor + signals |
| `AI` | AI Chat | AI assistant panel |

### Behavior

- Typing `BTC` alone → autocomplete shows all BTC pairs + function codes
- Typing `BTC G` → narrows to functions starting with G (shows `GP`)
- `Enter` → opens result in the active (focused) panel
- `Shift+Enter` → opens result in a new panel
- `↑` / `↓` → navigate command history
- `Ctrl+1..9` → focus panel by number
- `Esc` → dismiss autocomplete

---

## 5. Panel Widget Catalog (15 Widgets)

### Adapted from Existing Features (7)

| Widget | Source | Changes needed |
|--------|--------|----------------|
| GP — Chart | `CandlestickChart.tsx` | Wrap in panel container, accept ticker prop |
| NEWS — News | `NewsFeedPanel.tsx` | Filter by panel ticker context |
| PORT — Portfolio | Portfolio page components | Embed in panel, compact layout |
| WL — Watchlist | `WatchlistPanel.tsx` | Add multi-column sorting, custom column picker |
| ALRT — Alerts | Alerts page | Embed in panel |
| BT — Backtest | Backtest page | Embed in panel |
| MON — Monitor | Monitor page | Embed in panel |

### New Widgets (8)

| Widget | Description | Data source |
|--------|-------------|-------------|
| HM — Heatmap | Treemap: size=market cap, color=% change. Supports crypto, US sectors, commodities. Drill-down by sector. | Binance, Yahoo Finance, CoinGecko |
| FA — Fundamentals | Stocks: P/E, EPS, revenue, gross margin, debt/equity, earnings history chart. Crypto: market cap, circulating supply, dominance, all-time high. | Yahoo Finance `/quoteSummary`, CoinGecko |
| SCR — Screener | Multi-asset filter builder. Filters: price, % change, volume, market cap, P/E, EPS growth, RSI, MA crossover. Save/load filter presets. | Yahoo Finance, Binance |
| CAL — Calendar | Tabbed: Earnings (company, EPS est, EPS actual, surprise%) + Macro (event name, country, actual vs forecast). Date range filter. | Alpha Vantage EARNINGS, FRED API |
| OPT — Options Chain | Put/call table: strike, expiry, bid, ask, IV, delta, gamma, theta, open interest. IV surface chart (3D). Expiry selector. | Yahoo Finance v7 options endpoint |
| YCRV — Yield Curves | US Treasury yield curve line chart (2Y, 5Y, 10Y, 30Y) with historical overlay. Inversion indicator. Date scrubber. | FRED API series: DGS2, DGS5, DGS10, DGS30 |
| RISK — Risk Analytics | Portfolio-level: VaR (95/99%), CVaR, Sharpe ratio, Sortino ratio, Calmar ratio, max drawdown, stress scenarios (2008, 2020, custom). | Internal calculation engine (Go) |
| AI — AI Chat | Existing AI chat panel as dockable widget. Context-aware of active panel's ticker. | OpenAI / Ollama (existing) |

---

## 6. Data Layer Expansion

### New Backend Adapters

| Adapter | Purpose | API | Auth |
|---------|---------|-----|------|
| `YahooFundamentalsAdapter` | Fundamentals, options, earnings calendar | Yahoo Finance v8 (no key) | None |
| `AlphaVantageAdapter` | Earnings calendar, income statements, balance sheets | Alpha Vantage REST | Free API key |
| `FREDAdapter` | Yield curves, macro events, economic indicators | FRED REST | Free API key |
| `CoinGeckoAdapter` | Crypto fundamentals, global market data, heatmap data | CoinGecko v3 (free) | None |

### New Backend Routes

```
GET  /api/v1/fundamentals/:symbol           → FA panel data
GET  /api/v1/options/:symbol                → Options chain
GET  /api/v1/options/:symbol/iv-surface     → IV surface data
GET  /api/v1/calendar/earnings              → Upcoming earnings
GET  /api/v1/calendar/macro                 → Macro events (Fed, CPI, GDP)
GET  /api/v1/yield-curves                   → Treasury yield curve data
GET  /api/v1/heatmap/:market                → Heatmap data (crypto|us_equities|commodities)
POST /api/v1/screener/run                   → Execute screener filter
GET  /api/v1/screener/presets               → List saved screener presets
POST /api/v1/screener/presets               → Save screener preset
GET  /api/v1/workspaces                     → List workspaces
POST /api/v1/workspaces                     → Create workspace
GET  /api/v1/workspaces/:id                 → Get workspace (layout + panel configs)
PUT  /api/v1/workspaces/:id                 → Update workspace
DELETE /api/v1/workspaces/:id               → Delete workspace
```

### New DB Models

```go
// Workspace — user-saved panel layout
Workspace {
    ID          uint64
    UserID      uint64
    Name        string
    IsTemplate  bool
    Layout      JSON  // react-grid-layout serialized grid
    PanelStates JSON  // map[panelID]{ function, ticker, params }
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

// ScreenerPreset — saved screener filter configs
ScreenerPreset {
    ID          uint64
    UserID      uint64
    Name        string
    AssetClass  string  // crypto|equity|commodity
    Filters     JSON    // array of filter rules
    CreatedAt   time.Time
}
```

---

## 7. Frontend File Changes

### Removed
- `components/layout/Sidebar.tsx` — replaced by command bar
- `components/layout/TopBar.tsx` — replaced by terminal top bar
- `components/layout/Layout.tsx` — replaced by WorkspaceLayout.tsx

### New Files
```
frontend/src/
  components/terminal/
    WorkspaceLayout.tsx        # Root layout: command bar + workspace tabs + panel grid
    CommandBar.tsx             # Command input with autocomplete
    WorkspaceTabs.tsx          # Multi-workspace tab switcher
    PanelGrid.tsx              # react-grid-layout wrapper
    PanelSlot.tsx              # Individual panel container (header + content)
    PanelHeader.tsx            # Ticker badge, function label, link/max/close controls
    PanelLinkGroup.tsx         # Link group color selector
    WidgetRegistry.tsx         # Maps function codes → widget components
  components/widgets/
    ChartWidget.tsx            # GP — wraps existing CandlestickChart
    HeatmapWidget.tsx          # HM — new
    FundamentalsWidget.tsx     # FA — new
    NewsWidget.tsx             # NEWS — wraps existing NewsFeedPanel
    PortfolioWidget.tsx        # PORT — wraps existing portfolio components
    WatchlistWidget.tsx        # WL — enhanced existing WatchlistPanel
    ScreenerWidget.tsx         # SCR — new
    CalendarWidget.tsx         # CAL — new
    OptionsWidget.tsx          # OPT — new
    YieldCurveWidget.tsx       # YCRV — new
    RiskWidget.tsx             # RISK — new
    BacktestWidget.tsx         # BT — wraps existing backtest components
    AlertsWidget.tsx           # ALRT — wraps existing alerts
    MonitorWidget.tsx          # MON — wraps existing monitor
    AIChatWidget.tsx           # AI — wraps existing chat panel
  stores/
    workspaceStore.ts          # New store: workspaces, active workspace, panel states
  types/
    terminal.ts                # Terminal-specific types (WorkspaceLayout, PanelConfig, etc.)
  api/
    terminal.ts                # API calls: workspaces, fundamentals, options, calendar, etc.
```

---

## 8. Phase Roadmap

| Phase | Name | Deliverables | New panels |
|-------|------|-------------|------------|
| **A** | Terminal Foundation | WorkspaceLayout replaces Layout; react-grid-layout grid; CommandBar with autocomplete + routing; workspace tabs; workspace save/load DB + API; port 7 existing features as panel widgets; workspace templates (4) | GP, NEWS, PORT, WL, ALRT, BT, MON |
| **B** | Market Data Expansion | CoinGecko adapter; Yahoo Finance commodity tickers; Market Heatmap panel; enhanced Watchlist with custom columns | HM, enhanced WL |
| **C** | Screener | Backend screener query engine; multi-asset filter builder; saved presets; Screener panel | SCR |
| **D** | Fundamentals | Yahoo Finance `/quoteSummary` adapter; Fundamentals panel for stocks + crypto | FA |
| **E** | Earnings & Macro Calendar | Alpha Vantage earnings; FRED macro events; Calendar panel | CAL |
| **F** | Fixed Income & Yield Curves | FRED yield curve data adapter; Yield Curve panel with historical scrubber | YCRV |
| **G** | Options Chain | Yahoo Finance options adapter; Options Chain panel with IV surface | OPT |
| **H** | Risk Analytics | VaR/CVaR/Sharpe calculation engine in Go; stress test scenarios; Risk Analytics panel | RISK |
| **I** | Polish | Panel linking, workspace templates, keyboard shortcuts, Bloomberg hotkeys, panel snapshot export, AI panel context-awareness | AI enhanced |

---

## 9. Success Criteria

- User can load the app and immediately use the command bar to navigate to any view
- Panels resize and drag fluidly with layout persisted across sessions
- All 15 panel widgets are functional with real data
- Workspace templates give immediate value out of the box
- Command bar autocomplete covers all tickers (crypto + stock + commodity) and all function codes
- Panel linking allows synchronized multi-asset analysis
- Options chain shows real data from Yahoo Finance (no paid API required)
- Yield curves pulled live from FRED
- Screener returns filtered results in < 2s for standard filters

---

*Design approved 2026-03-06. Next step: implementation plan (writing-plans).*
