# Phase 2 — Market Data Layer Design

**Date:** 2026-02-24
**Status:** Approved
**Scope:** Full end-to-end (backend + frontend + tests)

---

## Overview

Fetch, store, and serve OHLCV candle data using a DB-first, lazy gap-filling architecture. No streaming in Phase 2 — streaming is deferred to Phase 8 (Live Market Monitor).

---

## Architecture: DB-first REST Polling

```
GET /candles request
        │
        ▼
   DataService.GetCandles(ctx, adapter, symbol, tf, from, to)
        │
        ├─► Query MySQL for requested range
        │
        ├─► Detect missing time gaps
        │
        ├─► For each gap: fetch from exchange REST API
        │       BinanceAdapter.FetchOHLCV()  or
        │       YahooAdapter.FetchOHLCV()
        │
        ├─► Upsert gap candles to MySQL
        │
        └─► Return merged, sorted []models.Candle

Background worker (every 5 min):
  - Track "hot" symbols in Redis (SET with 24h TTL per access)
  - For each hot symbol: DataService.SyncRecent() → upsert last 500 candles
```

Redis is used only for hot-symbol tracking (simple SET/EXPIRE), not Streams.

---

## Backend Components

### New Files

```
backend/internal/adapter/
  binance.go          # BinanceAdapter implementation
  yahoo.go            # YahooAdapter implementation
  dataservice.go      # DataService: GetCandles, SyncRecent, gap detection
  binance_test.go
  yahoo_test.go
  dataservice_test.go

backend/internal/api/
  candles.go          # GET /candles, GET /candles/timeframes handlers
  markets.go          # GET /markets, GET /markets/:id/symbols handlers
```

### Modified Files

```
backend/internal/api/routes.go     # Register 4 new routes
backend/cmd/server/main.go         # Register Binance/Yahoo adapters + background worker
```

### Adapter Contracts

**BinanceAdapter** implements `registry.MarketAdapter`:
- `FetchOHLCV(symbol, tf, from, to)` — `GET /api/v3/klines`, batch 1000, exponential backoff on 429
- `FetchSymbols()` — `GET /api/v3/exchangeInfo`, all USDT pairs
- `StreamTicker()` — stub only (returns nil, Phase 8)
- `IsStreamingSupported()` → `false` (Phase 2)

**YahooAdapter** implements `registry.MarketAdapter`:
- `FetchOHLCV(symbol, tf, from, to)` — Yahoo unofficial endpoint, maps intervals
- `FetchSymbols()` — returns curated hardcoded list (~100 symbols: S&P500, ETFs, forex)
- `StreamTicker()` — stub (returns nil)
- `IsStreamingSupported()` → `false`

### DataService

```go
type DataService struct {
    db    *gorm.DB
    redis *redis.Client
    reg   *registry.AdapterRegistry
}

func (s *DataService) GetCandles(ctx, adapterID, symbol, tf, from, to) ([]Candle, error)
func (s *DataService) SyncRecent(ctx, adapterID, symbol, tf) error
func (s *DataService) markHot(ctx, adapterID, symbol, tf)
```

Gap detection logic:
1. Query DB: `SELECT * FROM candles WHERE adapter=? AND symbol=? AND timeframe=? AND timestamp BETWEEN ? AND ? ORDER BY timestamp`
2. Walk the expected timestamp series (from→to, step=tf duration)
3. Any expected timestamp not present in DB results is a gap
4. Group contiguous gaps into ranges; fetch each range from exchange
5. Upsert fetched candles; merge into sorted result

---

## API Contracts

Base path: `/api/v1/`

| Method | Path | Description |
|---|---|---|
| `GET` | `/markets` | List all registered adapters |
| `GET` | `/markets/:adapterID/symbols` | List symbols for an adapter |
| `GET` | `/candles` | Fetch OHLCV candles |
| `GET` | `/candles/timeframes` | List supported timeframes |

### GET /markets
```json
[
  { "id": "binance", "name": "Binance", "streaming_supported": false },
  { "id": "yahoo",   "name": "Yahoo Finance", "streaming_supported": false }
]
```

### GET /markets/:adapterID/symbols
```json
[
  { "symbol": "BTCUSDT", "name": "Bitcoin / USDT", "base_asset": "BTC", "quote_asset": "USDT" }
]
```

### GET /candles
Query params: `adapter` (required), `symbol` (required), `timeframe` (required), `from` (Unix ms, optional), `to` (Unix ms, optional, default=now)

```json
[
  {
    "symbol": "BTCUSDT",
    "timeframe": "1h",
    "timestamp": 1700000000000,
    "open": "42000.00000000",
    "high": "42500.00000000",
    "low": "41800.00000000",
    "close": "42200.00000000",
    "volume": "1234.56780000"
  }
]
```

### GET /candles/timeframes
```json
["1m", "5m", "15m", "30m", "1h", "4h", "1d", "1w"]
```

---

## Frontend Components

### New Files

```
frontend/src/
  components/chart/
    CandlestickChart.tsx    # lightweight-charts wrapper
    ChartToolbar.tsx        # adapter/symbol/timeframe selectors
  hooks/
    useCandles.ts           # useQuery for GET /candles
    useSymbols.ts           # useQuery for GET /markets/:id/symbols
    useMarkets.ts           # useQuery for GET /markets
  pages/
    Chart.tsx               # updated from stub to full chart page
```

### Chart Page Layout

```
┌───────────────────────────────────────────────────────┐
│ [Adapter▼]  [Symbol search (debounced 300ms)]  [1m 5m 15m 1h 4h 1d 1w] │
├───────────────────────────────────────────────────────┤
│                                                       │
│   CandlestickChart (lightweight-charts)               │
│   • Loading skeleton on initial load                  │
│   • Loading overlay on symbol/timeframe change        │
│     (no chart destroy/recreate — overlay preserves    │
│      layout while new data arrives)                   │
│   • Error state with retry button                     │
│                                                       │
└───────────────────────────────────────────────────────┘
```

### CandlestickChart Lifecycle

- Holds `IChartApi` and `ISeriesApi<"Candlestick">` in refs
- Initializes chart in `useEffect` on mount
- On data change: `series.setData(newData)` — no destroy/recreate
- `ResizeObserver` handles container size changes
- Cleans up chart on unmount

---

## Test Plan

### Backend

| File | Tests |
|---|---|
| `binance_test.go` | FetchOHLCV success (mock HTTP), rate-limit backoff (429 → retry), empty response, HTTP error |
| `yahoo_test.go` | FetchOHLCV success, unsupported interval returns error |
| `dataservice_test.go` | No gaps (returns DB data), full gap, partial gap (prefix/suffix/interior), empty DB |

### Frontend

| File | Tests |
|---|---|
| `Chart.test.tsx` | Renders loading skeleton, renders chart after data load, error state shows retry |
| `useCandles.test.ts` | Correct query key, refetches on symbol change |

---

## Implementation Order

1. `BinanceAdapter` + tests
2. `YahooAdapter` + tests
3. `DataService` + tests
4. API handlers (`candles.go`, `markets.go`)
5. Register adapters + background worker in `main.go`
6. `CandlestickChart` component
7. `ChartToolbar` component + hooks (`useMarkets`, `useSymbols`, `useCandles`)
8. Update `Chart.tsx` page
9. Frontend tests

---

## Out of Scope (Phase 2)

- Live ticker streaming (Phase 8)
- Redis Streams pipeline (Phase 8)
- Indicator overlays (Phase 5)
- Volume histogram pane (Phase 5)
