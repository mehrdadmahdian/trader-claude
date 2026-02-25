# Phase 6 — Portfolio Tracker Design

**Date:** 2026-02-25
**Status:** Approved
**Scope:** Multi-asset portfolio tracker with live PnL, transaction history, allocation donut, and equity curve.

---

## Overview

Phase 6 adds a full portfolio tracker to trader-claude. Users create portfolios, add positions (stocks + crypto), log transactions, and see live PnL via WebSocket. The existing `Portfolio` model is extended (not replaced) to support multi-asset tracking while remaining compatible with future live trading in Phase 8.

---

## Architecture Decision

**Approach B — Split packages** was chosen over all-in-one or adapter reuse.

- `internal/portfolio/` — CRUD service + PnL recalculator + equity curve builder
- `internal/price/` — shared `PriceService` (Binance + Yahoo, Redis 30s TTL)

Rationale: `PriceService` will be reused by Phase 7 alert evaluator and Phase 8 monitor manager. Isolating it now avoids duplication later.

---

## Data Models

### Portfolio (extended)

Existing model is extended. Fields added/changed:

| Field | Type | Notes |
|---|---|---|
| `Description` | `text` | Optional free-text |
| `Type` | `enum(manual,paper,live)` | Replaces nothing — new field |
| `Currency` | `varchar(10)` | Default `USD` |
| `Symbol`, `StrategyName`, `Timeframe`, `Params` | kept | Made nullable/optional for backward compat with future live trading |

### Position (new model)

```
portfolios → positions (1:N)
```

| Field | Type | Notes |
|---|---|---|
| `ID` | BIGINT PK | |
| `PortfolioID` | BIGINT FK | index |
| `AdapterID` | varchar(20) | `binance` or `yahoo` |
| `Symbol` | varchar(20) | e.g. `BTCUSDT`, `AAPL` |
| `Market` | varchar(20) | e.g. `crypto`, `stock` |
| `Quantity` | DECIMAL(30,8) | |
| `AvgCost` | DECIMAL(20,8) | Weighted average cost basis |
| `CurrentPrice` | DECIMAL(20,8) | Updated by PriceService |
| `CurrentValue` | DECIMAL(20,8) | Quantity × CurrentPrice |
| `UnrealizedPnL` | DECIMAL(20,8) | CurrentValue − (Quantity × AvgCost) |
| `UnrealizedPnLPct` | DECIMAL(10,4) | |
| `OpenedAt` | DATETIME | First buy date |
| `CreatedAt`, `UpdatedAt` | DATETIME | |

### Transaction (new model)

```
portfolios → transactions (1:N)
positions  → transactions (1:N, nullable — portfolio-level cash events)
```

| Field | Type | Notes |
|---|---|---|
| `ID` | BIGINT PK | |
| `PortfolioID` | BIGINT FK | index |
| `PositionID` | BIGINT FK nullable | null for deposit/withdrawal |
| `Type` | `enum(buy,sell,deposit,withdrawal)` | |
| `AdapterID` | varchar(20) | |
| `Symbol` | varchar(20) | empty for deposit/withdrawal |
| `Quantity` | DECIMAL(30,8) | |
| `Price` | DECIMAL(20,8) | Execution price |
| `Fee` | DECIMAL(20,8) | Commission/fee |
| `Notes` | text | Optional |
| `ExecutedAt` | DATETIME | User-specified execution time |
| `CreatedAt` | DATETIME | |

---

## Backend Services

### `internal/price/service.go`

```go
type PriceService interface {
    GetPrice(ctx context.Context, adapterID, symbol string) (float64, error)
}
```

- Binance: `GET /api/v3/ticker/price?symbol={symbol}`
- Yahoo: `GET /v8/finance/chart/{symbol}?range=1d&interval=1m` → last close
- Redis cache key: `price:{adapterID}:{symbol}`, TTL 30s
- Returns `ErrPriceUnavailable` on miss from both cache and source

### `internal/portfolio/service.go`

```go
type PortfolioService interface {
    // CRUD
    CreatePortfolio(ctx, req) (*Portfolio, error)
    GetPortfolio(ctx, id) (*Portfolio, error)
    ListPortfolios(ctx) ([]*Portfolio, error)
    UpdatePortfolio(ctx, id, req) (*Portfolio, error)
    DeletePortfolio(ctx, id) error
    GetSummary(ctx, id) (*PortfolioSummary, error)

    // Positions
    AddPosition(ctx, portfolioID, req) (*Position, error)
    UpdatePosition(ctx, positionID, req) (*Position, error)
    DeletePosition(ctx, positionID) error

    // Transactions
    AddTransaction(ctx, portfolioID, req) (*Transaction, error)
    ListTransactions(ctx, portfolioID, page, limit) ([]*Transaction, int64, error)

    // Analytics
    RecalculatePortfolio(ctx, id) error  // fetch current prices → update all positions + portfolio
    GetEquityCurve(ctx, id) ([]*EquityPoint, error)  // replay transactions → {timestamp, value}
}
```

**`RecalculatePortfolio`:** For each position, calls `PriceService.GetPrice(adapterID, symbol)`, updates `CurrentPrice`, `CurrentValue`, `UnrealizedPnL`, `UnrealizedPnLPct`. Then sums all position values to update `Portfolio.CurrentValue`.

**`GetEquityCurve`:** Replays all transactions in chronological order. At each transaction, compute running portfolio value = current cash + sum of (qty × price at that time for each position). Returns `[{timestamp, value}]`.

---

## API Routes

All under `/api/v1/`:

```
POST   /portfolios                         Create portfolio
GET    /portfolios                         List all
GET    /portfolios/:id                     Get with positions
PUT    /portfolios/:id                     Update
DELETE /portfolios/:id                     Soft delete

GET    /portfolios/:id/summary             Summary cards (TotalValue, PnL, PnL%, DayChange%)

POST   /portfolios/:id/positions           Add position
PUT    /portfolios/:id/positions/:posId    Edit position
DELETE /portfolios/:id/positions/:posId    Remove position

POST   /portfolios/:id/transactions        Log transaction
GET    /portfolios/:id/transactions        Paginated list (?page=&limit=)

GET    /portfolios/:id/equity-curve        {points:[{timestamp,value}]}

WS     /ws/portfolio/:id/live             PnL updates every 5s
```

---

## WebSocket Protocol

`/ws/portfolio/:id/live` — server pushes every 5s:

```json
{
  "type": "portfolio_update",
  "portfolio_id": 1,
  "total_value": 12540.50,
  "total_pnl": 540.50,
  "total_pnl_pct": 4.50,
  "positions": [
    {"id": 1, "symbol": "BTCUSDT", "current_price": 42000.0, "unrealized_pnl": 300.0, "unrealized_pnl_pct": 3.5}
  ]
}
```

Server calls `RecalculatePortfolio` on each tick, then broadcasts. Goroutine per active WS connection, exits on disconnect.

---

## Frontend Layout

**Page: `/portfolio`** → `Portfolio.tsx`

```
┌─────────────────────────────────────────────────────┐
│ [Portfolio ▼]  [+ New Portfolio]                    │
├──────────┬──────────┬──────────┬───────────────────┤
│ Total    │ Total    │ PnL%     │ Day Change%        │
│ Value    │ PnL      │          │                    │
├──────────┴──────────┴──────────┴───────────────────┤
│                                                     │
│  Positions Table (60%)  │  Allocation Donut (40%)  │
│  Asset|Qty|AvgCost|     │  (Recharts, hover        │
│  Price|Value|PnL|Wt%    │   highlights table row)  │
│  [+ Add Position]       │                          │
│                                                     │
├─────────────────────────────────────────────────────┤
│  [Equity Curve] [Transactions]                      │
│  (tab content below)                                │
└─────────────────────────────────────────────────────┘
```

**Components:**
- `PortfolioSelector` — dropdown + "New Portfolio" button
- `PortfolioSummaryCards` — 4 stat cards (TotalValue, TotalPnL, PnL%, DayChange%)
- `PositionsTable` — sortable, row highlight on donut hover, PnL cells green/red
- `AllocationDonut` — Recharts PieChart, hover emits event to highlight table row
- `EquityCurveChart` — Recharts LineChart from equity-curve endpoint
- `TransactionTable` — paginated table
- `NewPortfolioModal` — name, description, type, initial cash, currency
- `AddPositionModal` — adapter, symbol, qty, avg cost, date
- `AddTransactionModal` — type, adapter, symbol, qty, price, fee, notes, date

**Live updates:** `usePortfolioLive(id)` hook opens WS on mount, patches Zustand `portfolioStore` on each message. Position rows flash green/red on price change.

---

## Testing Plan

**Backend:**
- `internal/price/` — unit tests: Redis cache hit/miss, Binance mock, Yahoo mock
- `internal/portfolio/` — unit tests: CRUD, `RecalculatePortfolio` PnL math, `GetEquityCurve` transaction replay
- API integration tests: all 12 endpoints, WS stream delivery

**Frontend:**
- Donut + table interaction (hover donut → row highlight)
- WS mock → position row updates
- Modal form validation

---

## Sub-phase Order

1. **6.1** — Models (Position, Transaction) + Portfolio extension + GORM migration
2. **6.2** — `internal/price/` PriceService (Binance + Yahoo + Redis cache) + tests
3. **6.3** — `internal/portfolio/` service (CRUD + RecalculatePortfolio + GetEquityCurve) + tests
4. **6.4** — API endpoints (all 12 routes) + integration tests
5. **6.5** — WebSocket `/ws/portfolio/:id/live` + tests
6. **6.6** — Frontend: layout + positions table + donut chart
7. **6.7** — Frontend: equity curve + transaction table + live WS hook
8. **6.8** — All modals (New Portfolio, Add Position, Add Transaction)
