# API Reference

**Base URL:** `http://localhost:8080`
**API Prefix:** `/api/v1`
**Content-Type:** `application/json`

---

## Health

### GET /health

Returns service health status.

**Response 200:**
```json
{
  "status": "ok",
  "db": "ok",
  "redis": "ok",
  "version": "0.1.0"
}
```

---

## WebSocket

### GET /ws

Upgrade to WebSocket. All real-time events flow through this endpoint.

**Client → Server messages:**
```json
{ "type": "subscribe",   "channel": "ticks:BTC/USDT" }
{ "type": "unsubscribe", "channel": "ticks:BTC/USDT" }
{ "type": "ping" }
```

**Server → Client messages:**
```json
{ "type": "tick",         "channel": "ticks:BTC/USDT",           "data": { ... } }
{ "type": "candle",       "channel": "candles:BTC/USDT:1m",       "data": { ... } }
{ "type": "signal",       "channel": "signals:{strategy_id}",     "data": { ... } }
{ "type": "alert",        "channel": "alerts:{portfolio_id}",      "data": { ... } }
{ "type": "notification", "channel": "notifications",             "data": { ... } }
{ "type": "pong" }
{ "type": "error",        "data": { "message": "..." } }
```

See `docs/websocket.md` for full protocol details.

---

## Symbols

### GET /api/v1/symbols
List all symbols.

**Query params:** `market` (crypto|stock|forex), `active` (true|false)

**Response 200:**
```json
[
  {
    "id": 1,
    "symbol": "BTC/USDT",
    "name": "Bitcoin",
    "market": "crypto",
    "exchange": "binance",
    "base_currency": "BTC",
    "quote_currency": "USDT",
    "active": true,
    "metadata": {}
  }
]
```

### GET /api/v1/symbols/:id
Get symbol by ID.

### POST /api/v1/symbols
Create symbol.

### PUT /api/v1/symbols/:id
Update symbol.

---

## Candles

### GET /api/v1/candles
Query candle (OHLCV) data.

**Query params:**
| Param | Required | Description |
|---|---|---|
| `symbol` | yes | e.g. `BTC/USDT` |
| `market` | yes | `crypto`, `stock`, `forex` |
| `timeframe` | yes | `1m`, `5m`, `15m`, `1h`, `4h`, `1d` |
| `from` | no | Unix timestamp (seconds) |
| `to` | no | Unix timestamp (seconds) |
| `limit` | no | Max rows (default 500) |

**Response 200:**
```json
[
  {
    "id": 1,
    "symbol": "BTC/USDT",
    "market": "crypto",
    "timeframe": "1h",
    "timestamp": 1700000000,
    "open": "43000.00000000",
    "high": "43500.00000000",
    "low": "42800.00000000",
    "close": "43200.00000000",
    "volume": "1500.00000000"
  }
]
```

### POST /api/v1/candles
Bulk insert candles (used by adapters).

---

## Strategies

### GET /api/v1/strategies
List registered strategy definitions.

**Response 200:**
```json
[
  {
    "id": 1,
    "name": "EMA Crossover",
    "key": "ema_crossover",
    "description": "Crosses of fast/slow EMAs",
    "params_schema": {
      "fast_period": { "type": "int", "default": 9 },
      "slow_period": { "type": "int", "default": 21 }
    },
    "version": "1.0.0"
  }
]
```

### GET /api/v1/strategies/:id

---

## Backtests

### GET /api/v1/backtests
List backtest runs.

**Query params:** `status` (pending|running|completed|failed), `limit`, `offset`

**Response 200:**
```json
[
  {
    "id": 1,
    "strategy_id": 1,
    "symbol": "BTC/USDT",
    "market": "crypto",
    "timeframe": "1h",
    "start_date": "2023-01-01T00:00:00Z",
    "end_date": "2023-12-31T00:00:00Z",
    "initial_capital": "10000.00000000",
    "status": "completed",
    "params": {},
    "metrics": {
      "total_return": "0.342",
      "sharpe_ratio": "1.87",
      "max_drawdown": "-0.142",
      "win_rate": "0.58",
      "total_trades": 142
    },
    "equity_curve": [ ... ],
    "created_at": "2024-01-01T00:00:00Z"
  }
]
```

### GET /api/v1/backtests/:id
Get backtest with full trade list.

### POST /api/v1/backtests
Create and queue a new backtest run.

**Body:**
```json
{
  "strategy_id": 1,
  "symbol": "BTC/USDT",
  "market": "crypto",
  "timeframe": "1h",
  "start_date": "2023-01-01T00:00:00Z",
  "end_date": "2023-12-31T00:00:00Z",
  "initial_capital": "10000",
  "params": { "fast_period": 9, "slow_period": 21 }
}
```

**Response 201:** Backtest object with `status: "pending"`

### DELETE /api/v1/backtests/:id
Cancel or delete a backtest.

---

## Trades

### GET /api/v1/trades
List trades.

**Query params:** `backtest_id`, `portfolio_id`, `symbol`, `limit`, `offset`

### GET /api/v1/trades/:id

---

## Portfolios

### GET /api/v1/portfolios
List portfolios.

### GET /api/v1/portfolios/:id
Get portfolio with positions.

### POST /api/v1/portfolios
Create portfolio.

**Body:**
```json
{
  "name": "My Paper Portfolio",
  "type": "paper",
  "initial_capital": "10000",
  "currency": "USDT"
}
```

### PUT /api/v1/portfolios/:id
Update portfolio.

---

## Alerts

### GET /api/v1/alerts
List alerts.

**Query params:** `active` (true|false), `symbol`

### GET /api/v1/alerts/:id

### POST /api/v1/alerts
Create alert.

**Body:**
```json
{
  "symbol": "BTC/USDT",
  "market": "crypto",
  "type": "price_above",
  "threshold": "50000",
  "message": "BTC above 50k!"
}
```

### PUT /api/v1/alerts/:id
Update alert (e.g. deactivate).

### DELETE /api/v1/alerts/:id

---

## Notifications

### GET /api/v1/notifications
List notifications.

**Query params:** `unread` (true|false), `limit`, `offset`

### PUT /api/v1/notifications/:id/read
Mark as read.

### PUT /api/v1/notifications/read-all
Mark all as read.

---

## Watchlists

### GET /api/v1/watchlists
List watchlists.

### GET /api/v1/watchlists/:id
Get watchlist with symbols.

### POST /api/v1/watchlists
Create watchlist.

### PUT /api/v1/watchlists/:id
Update watchlist (rename, update symbols).

### DELETE /api/v1/watchlists/:id

---

## Error Responses

All errors follow:
```json
{ "error": "human-readable message" }
```

| Code | Meaning |
|---|---|
| 400 | Bad request / validation error |
| 404 | Resource not found |
| 409 | Conflict (duplicate) |
| 422 | Unprocessable entity |
| 500 | Internal server error |
