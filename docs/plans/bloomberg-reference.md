# Bloomberg Terminal — Reference

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement each phase plan.

**Two apps, one codebase:**
- `/` → Old app (Layout.tsx) — trading workbench: backtest, alerts, strategies, portfolio management
- `/terminal` → Bloomberg workspace (WorkspaceLayout) — research & market viewer: panels, data, fundamentals, screener

**Asset classes:** Crypto, Equities, Commodities (no FX)
**Data sources:** Free only — Yahoo Finance, Alpha Vantage, FRED, CoinGecko

---

## Vision

A Bloomberg Terminal-style workspace at `/terminal` where:

- The screen is a **resizable, draggable grid of panels**
- Each panel independently loads any feature module (chart, news, screener, fundamentals, etc.)
- Navigation driven by a **command bar**: `<TICKER> <FUNCTION>` (e.g. `BTC GP`, `AAPL FA`)
- Users save/load multiple **workspace layouts** as tabs
- Panels can be **linked** — changing ticker in one panel updates all linked panels

A "Terminal" link in the existing sidebar nav switches between the two apps.

---

## Panel Builder

- Library: `react-grid-layout` (12-column, row height 30px)
- Each panel: min 3 cols wide, 4 rows tall — drag, resize, rearrange freely
- Panel IDs: `crypto.randomUUID()`
- Layout persists to backend DB; optimistic local update via Zustand

**Panel anatomy:**
```
+------------------------------------------+
| [TICKER] [FUNCTION LABEL]  ..  [🔗][⛶][✕] |
+------------------------------------------+
|           Panel Content Area             |
+------------------------------------------+
```
- `🔗` Link group (red/blue/green/yellow) — linked panels share ticker context
- `⛶` Maximize / restore
- `✕` Close

**Workspace tabs:**
```
[WS: Market Overview ▾] [WS: Trader ▾] [WS: Crypto ▾] [+]
```
Each workspace stores layout JSON + panel configs. 4 pre-built templates:
1. **Market Overview** — HM left, NEWS top-right, WL bottom-right
2. **Trader** — GP large top, PORT bottom-left, ALRT bottom-right
3. **Crypto** — GP BTC top-left, GP ETH top-right, NEWS bottom-left, WL bottom-right
4. **Analyst** — FA top-left, CAL top-right, SCR bottom

---

## Command Bar

Format: `>_ BTC GP`

| Code | Widget | Description |
|------|--------|-------------|
| `GP` | Chart | Candlestick chart with indicators |
| `HM` | Heatmap | Market treemap, color = % change |
| `FA` | Fundamentals | P/E, EPS, revenue, balance sheet; crypto supply/dominance |
| `NEWS` | News | Asset-specific news feed |
| `PORT` | Portfolio | Positions, PnL, transactions |
| `WL` | Watchlist | Multi-column watchlist with real-time prices |
| `SCR` | Screener | Filter builder across all asset classes |
| `CAL` | Calendar | Earnings dates + macro events (Fed, CPI, GDP) |
| `OPT` | Options Chain | Put/call chain with Greeks and IV |
| `YCRV` | Yield Curves | US Treasury curves + sovereign spreads |
| `RISK` | Risk Analytics | VaR, Sharpe/Sortino, drawdown, stress tests |
| `BT` | Backtest | Strategy backtest runner (read-only view) |
| `ALRT` | Alerts | Alert rules (read-only view) |
| `MON` | Monitor | Live strategy monitor (read-only view) |
| `AI` | AI Chat | AI assistant panel |

**Behavior:** `Enter` = active panel · `Shift+Enter` = new panel · `↑↓` = history · `Esc` = dismiss

---

## Widget Catalog

### Adapted from existing (7)
| Widget | Source file |
|--------|------------|
| GP | `components/chart/CandlestickChart.tsx` |
| NEWS | `components/dashboard/NewsFeedPanel.tsx` |
| PORT | `pages/Portfolio.tsx` |
| WL | `components/dashboard/WatchlistPanel.tsx` |
| ALRT | `pages/Alerts.tsx` |
| BT | `pages/Backtest.tsx` |
| MON | `pages/Monitor.tsx` |

### New widgets (8)
| Widget | Data source |
|--------|------------|
| HM | Binance + Yahoo Finance + CoinGecko |
| FA | Yahoo Finance `/quoteSummary` + CoinGecko |
| SCR | Yahoo Finance + Binance |
| CAL | Alpha Vantage EARNINGS + FRED API |
| OPT | Yahoo Finance v7 options endpoint |
| YCRV | FRED API (DGS2, DGS5, DGS10, DGS30) |
| RISK | Internal Go calculation engine |
| AI | Existing ChatPanel (context-aware of ticker) |

---

## New Frontend Files

```
frontend/src/
  components/terminal/
    WorkspaceLayout.tsx     # Root shell: command bar + tabs + grid
    CommandBar.tsx          # Bloomberg-style TICKER FUNCTION input
    WorkspaceTabs.tsx       # Workspace tab switcher
    PanelGrid.tsx           # react-grid-layout wrapper
    PanelSlot.tsx           # Panel container (header + content)
    WidgetRegistry.tsx      # FunctionCode → React component map
  components/widgets/
    ChartWidget.tsx         # GP
    NewsWidget.tsx          # NEWS
    PortfolioWidget.tsx     # PORT
    WatchlistWidget.tsx     # WL
    AlertsWidget.tsx        # ALRT
    BacktestWidget.tsx      # BT
    MonitorWidget.tsx       # MON
    AIChatWidget.tsx        # AI
    HeatmapWidget.tsx       # HM (Phase B)
    FundamentalsWidget.tsx  # FA (Phase D)
    ScreenerWidget.tsx      # SCR (Phase C)
    CalendarWidget.tsx      # CAL (Phase E)
    YieldCurveWidget.tsx    # YCRV (Phase F)
    OptionsWidget.tsx       # OPT (Phase G)
    RiskWidget.tsx          # RISK (Phase H)
  stores/workspaceStore.ts  # Panel CRUD, workspace tabs, templates
  types/terminal.ts         # FunctionCode, PanelConfig, WidgetProps, etc.
  api/terminal.ts           # Workspace API calls
```

**Widget props contract** — every widget must accept:
```typescript
interface WidgetProps {
  ticker: string
  market?: string       // "binance" | "yahoo" | "coingecko"
  timeframe?: string    // GP only
  params?: Record<string, unknown>
}
```

---

## New Backend Routes

```
# Workspaces (Phase A)
GET    /api/v1/workspaces
POST   /api/v1/workspaces
GET    /api/v1/workspaces/:id
PUT    /api/v1/workspaces/:id
DELETE /api/v1/workspaces/:id

# Heatmap (Phase B)
GET    /api/v1/heatmap/:market          market = crypto|equities|commodities

# Screener (Phase C)
POST   /api/v1/screener/run
GET    /api/v1/screener/presets
POST   /api/v1/screener/presets
DELETE /api/v1/screener/presets/:id

# Fundamentals (Phase D)
GET    /api/v1/fundamentals/:symbol     ?market=yahoo|coingecko

# Calendar (Phase E)
GET    /api/v1/calendar/earnings        ?from=&to=
GET    /api/v1/calendar/macro           ?from=&to=

# Yield Curves (Phase F)
GET    /api/v1/yield-curves             ?series=2y,5y,10y,30y&from=&to=

# Options (Phase G)
GET    /api/v1/options/:symbol          ?expiry=
GET    /api/v1/options/:symbol/expirations

# Risk (Phase H)
POST   /api/v1/risk/analyze             body: { portfolio_id, scenarios[] }
```

## New DB Models

```go
// Phase A
type Workspace struct {
    ID          int64  `gorm:"primaryKey;autoIncrement"`
    UserID      int64  `gorm:"not null;index"`
    Name        string `gorm:"type:varchar(100);not null"`
    IsTemplate  bool   `gorm:"default:false"`
    Layout      JSON   // react-grid-layout grid items
    PanelStates JSON   // map[panelID]PanelConfig
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

// Phase C
type ScreenerPreset struct {
    ID         int64  `gorm:"primaryKey;autoIncrement"`
    UserID     int64  `gorm:"not null;index"`
    Name       string `gorm:"type:varchar(100);not null"`
    AssetClass string `gorm:"type:varchar(20);not null"`
    Filters    JSON
    CreatedAt  time.Time
}
```

---

## Coding Conventions

### Backend
- Module: `github.com/trader-claude/backend`
- Handler files: `backend/internal/api/<feature>_handler.go`
- New packages: `backend/internal/<name>/`
- All routes under `/api/v1/`, registered in `routes.go`
- Error format: `c.Status(xxx).JSON(fiber.Map{"error": "..."})`
- Protected routes: `protected.Get(...)` (JWT middleware already applied)
- Financial amounts: `DECIMAL(20,8)` in DB — never `float64`
- GORM models: `BIGINT AUTO_INCREMENT` PK, `JSON` columns (MySQL 8.0)

### Frontend
- Path alias: `@/` = `frontend/src/`
- Terminal types: `types/terminal.ts` · Shared types: `types/index.ts`
- API calls: `api/client.ts` Axios instance only
- Icons: `lucide-react` only · Styling: Tailwind only
- Named exports: `export function MyComponent()`

---

## Phase Status

| Phase | Plan file | Status | Delivers |
|-------|-----------|--------|----------|
| **A** | `bloomberg-phase-A-plan.md` | ⬜ Pending | Panel grid, CommandBar, workspace CRUD, 8 widget wrappers, App routing |
| **B** | `bloomberg-phase-B-plan.md` | ⬜ Pending | CoinGecko adapter, HM widget |
| **C** | `bloomberg-phase-C-plan.md` | ⬜ Pending | Screener engine (Go), SCR widget |
| **D** | `bloomberg-phase-D-plan.md` | ⬜ Pending | Yahoo Finance fundamentals, FA widget |
| **E** | `bloomberg-phase-E-plan.md` | ⬜ Pending | Alpha Vantage + FRED calendar, CAL widget |
| **F** | `bloomberg-phase-F-plan.md` | ⬜ Pending | FRED yield curves, YCRV widget |
| **G** | `bloomberg-phase-G-plan.md` | ⬜ Pending | Yahoo Finance options, OPT widget |
| **H** | `bloomberg-phase-H-plan.md` | ⬜ Pending | VaR/Sharpe Go engine, RISK widget |
| **I** | `bloomberg-phase-I-plan.md` | ⬜ Pending | Panel linking, keyboard shortcuts, AI context |

*Write the next phase plan before starting implementation of that phase. Update status: ⬜ Pending → 🔄 In Progress → ✅ Done*
