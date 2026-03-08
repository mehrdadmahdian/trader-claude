# Bloomberg Terminal Clone — Master Plan Index

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement each phase plan.

**Goal:** Replace page-based trader-claude UI with a Bloomberg Terminal-style multi-panel workspace.

**Design doc:** `docs/plans/2026-03-06-bloomberg-terminal-design.md`

---

## Phase Plans

| Phase | File | Status | Deliverables |
|-------|------|--------|-------------|
| **A** | `bloomberg-phase-A-plan.md` | ✅ Written | Panel grid, CommandBar, workspace CRUD, 7 widget wrappers, App routing |
| **B** | `bloomberg-phase-B-plan.md` | Pending | CoinGecko adapter, Market Heatmap (HM) widget |
| **C** | `bloomberg-phase-C-plan.md` | Pending | Screener engine (Go), Screener (SCR) widget |
| **D** | `bloomberg-phase-D-plan.md` | Pending | Yahoo Finance fundamentals adapter, Fundamentals (FA) widget |
| **E** | `bloomberg-phase-E-plan.md` | Pending | Alpha Vantage + FRED calendar adapter, Calendar (CAL) widget |
| **F** | `bloomberg-phase-F-plan.md` | Pending | FRED yield curve adapter, Yield Curve (YCRV) widget |
| **G** | `bloomberg-phase-G-plan.md` | Pending | Yahoo Finance options adapter, Options Chain (OPT) widget |
| **H** | `bloomberg-phase-H-plan.md` | Pending | VaR/Sharpe Go engine, Risk Analytics (RISK) widget |
| **I** | `bloomberg-phase-I-plan.md` | Pending | Panel linking, workspace templates, keyboard shortcuts, AI context |

---

## Cross-Cutting Conventions

### Backend (Go)
- Module: `github.com/trader-claude/backend`
- New handler files go in `backend/internal/api/` named `<feature>_handler.go`
- Register all routes in `backend/internal/api/routes.go` under `/api/v1/`
- New packages (adapters, engines) go in `backend/internal/<name>/`
- Error responses: `c.Status(xxx).JSON(fiber.Map{"error": "..."})`
- Protected routes use: `protected.Get(...)` (already has JWT middleware)
- Never use float64 for financial amounts in DB; use `DECIMAL(20,8)`
- GORM models: add to `backend/internal/models/models.go`, BIGINT PK, JSON columns

### Frontend (React/TypeScript)
- Path alias: `@/` = `frontend/src/`
- All new TS types go in `frontend/src/types/terminal.ts` (terminal-specific) or `frontend/src/types/index.ts` (shared)
- All new stores go in `frontend/src/stores/` (one file per store)
- API calls: use `frontend/src/api/client.ts` Axios instance
- Icons: `lucide-react` only
- Tailwind only — no inline styles
- Components use named exports: `export function MyComponent()`
- Panel widgets must accept `WidgetProps` interface (defined in terminal.ts)

### Widget Props Contract
Every widget component must accept this interface:
```typescript
interface WidgetProps {
  ticker: string        // active ticker for this panel (e.g. "BTCUSDT", "AAPL", "GC=F")
  market?: string       // adapter id: "binance" | "yahoo" | "coingecko"
  timeframe?: string    // candlestick timeframe (only for GP)
  params?: Record<string, unknown>  // widget-specific extra config
}
```

### Panel Grid
- Library: `react-grid-layout` (12-column, row height 30px)
- Each panel: minimum 3 cols wide, 4 rows tall
- Panel IDs: generated with `crypto.randomUUID()`
- Layout persists to backend; optimistic local update via Zustand

### Workspace Templates (4 pre-built)
Used as defaults when user first logs in:
1. **Market Overview** — HM (full left), NEWS (top right), WL (bottom right)
2. **Trader** — GP (large, top), PORT (bottom left), ALRT (bottom right)
3. **Crypto** — GP BTC (top left), GP ETH (top right), NEWS (bottom left), WL (bottom right)
4. **Analyst** — FA (top left), CAL (top right), SCR (bottom)

---

## New API Routes Summary

```
# Workspaces
GET    /api/v1/workspaces
POST   /api/v1/workspaces
GET    /api/v1/workspaces/:id
PUT    /api/v1/workspaces/:id
DELETE /api/v1/workspaces/:id

# Heatmap (Phase B)
GET    /api/v1/heatmap/:market         market = crypto|equities|commodities

# Screener (Phase C)
POST   /api/v1/screener/run
GET    /api/v1/screener/presets
POST   /api/v1/screener/presets
DELETE /api/v1/screener/presets/:id

# Fundamentals (Phase D)
GET    /api/v1/fundamentals/:symbol    ?market=yahoo|coingecko

# Calendar (Phase E)
GET    /api/v1/calendar/earnings       ?from=&to=
GET    /api/v1/calendar/macro          ?from=&to=

# Yield Curves (Phase F)
GET    /api/v1/yield-curves            ?series=2y,5y,10y,30y&from=&to=

# Options (Phase G)
GET    /api/v1/options/:symbol         ?expiry=
GET    /api/v1/options/:symbol/expirations

# Risk (Phase H)
POST   /api/v1/risk/analyze            body: { portfolio_id, scenarios[] }
```

---

## New DB Models Summary

```go
// Workspace (Phase A)
type Workspace struct {
    ID          int64      `gorm:"primaryKey;autoIncrement"`
    UserID      int64      `gorm:"not null;index"`
    Name        string     `gorm:"type:varchar(100);not null"`
    IsTemplate  bool       `gorm:"default:false"`
    Layout      datatypes.JSON
    PanelStates datatypes.JSON
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

// ScreenerPreset (Phase C)
type ScreenerPreset struct {
    ID         int64      `gorm:"primaryKey;autoIncrement"`
    UserID     int64      `gorm:"not null;index"`
    Name       string     `gorm:"type:varchar(100);not null"`
    AssetClass string     `gorm:"type:varchar(20);not null"`
    Filters    datatypes.JSON
    CreatedAt  time.Time
}
```

---

*Write the next phase plan before starting implementation of that phase.*
