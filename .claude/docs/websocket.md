# WebSocket Protocol

**Endpoint:** `ws://localhost:6060/ws`

All real-time communication (price ticks, candle updates, signals, alerts, notifications) flows through a single WebSocket connection per client.

---

## Connection Lifecycle

```
1. Client: GET /ws (HTTP Upgrade)
2. Server: 101 Switching Protocols
3. Client: subscribe to channels
4. Server: streams events to subscribed channels
5. Client: ping/pong heartbeat (every 30s recommended)
6. Client: unsubscribe or disconnect
```

---

## Message Format

All messages are JSON objects with a `type` field.

### Client → Server

**Subscribe to a channel:**
```json
{
  "type": "subscribe",
  "channel": "ticks:BTC/USDT"
}
```

**Unsubscribe:**
```json
{
  "type": "unsubscribe",
  "channel": "ticks:BTC/USDT"
}
```

**Ping (heartbeat):**
```json
{ "type": "ping" }
```

---

### Server → Client

**Tick (real-time price):**
```json
{
  "type": "tick",
  "channel": "ticks:BTC/USDT",
  "data": {
    "symbol": "BTC/USDT",
    "price": "43200.50000000",
    "volume": "1.23400000",
    "timestamp": 1700000000
  }
}
```

**Candle (completed bar):**
```json
{
  "type": "candle",
  "channel": "candles:BTC/USDT:1h",
  "data": {
    "symbol": "BTC/USDT",
    "timeframe": "1h",
    "timestamp": 1700000000,
    "open": "43000.00000000",
    "high": "43500.00000000",
    "low": "42800.00000000",
    "close": "43200.00000000",
    "volume": "1500.00000000"
  }
}
```

**Signal (strategy signal):**
```json
{
  "type": "signal",
  "channel": "signals:1",
  "data": {
    "strategy_id": 1,
    "symbol": "BTC/USDT",
    "action": "buy",
    "price": "43200.00000000",
    "confidence": 0.85,
    "timestamp": 1700000000
  }
}
```

**Alert triggered:**
```json
{
  "type": "alert",
  "channel": "alerts:5",
  "data": {
    "alert_id": 42,
    "portfolio_id": 5,
    "symbol": "BTC/USDT",
    "type": "price_above",
    "threshold": "50000.00000000",
    "current_price": "50100.00000000",
    "message": "BTC above 50k!",
    "triggered_at": "2024-01-01T00:00:00Z"
  }
}
```

**Notification:**
```json
{
  "type": "notification",
  "channel": "notifications",
  "data": {
    "id": 10,
    "type": "backtest_completed",
    "title": "Backtest complete",
    "body": "EMA Crossover on BTC/USDT finished with +34.2% return",
    "metadata": { "backtest_id": 7 }
  }
}
```

**Pong:**
```json
{ "type": "pong" }
```

**Error:**
```json
{
  "type": "error",
  "data": { "message": "unknown channel format" }
}
```

---

## Channels

| Channel Pattern | Description |
|---|---|
| `ticks:{symbol}` | Real-time price ticks for a symbol |
| `candles:{symbol}:{timeframe}` | Completed candle bars |
| `signals:{strategy_id}` | Strategy signals |
| `alerts:{portfolio_id}` | Alerts scoped to a portfolio |
| `notifications` | All system notifications |

**Examples:**
- `ticks:BTC/USDT`
- `candles:BTC/USDT:1h`
- `candles:ETH/USDT:15m`
- `signals:1`
- `alerts:3`
- `notifications`

---

## Hub Implementation

The WebSocket hub is a singleton (`ws.GetHub()`) with:
- Per-client buffered send channel (256 messages)
- Channel subscription map: `channel → set<client>`
- Goroutine-based send loop per client (drops slow clients after buffer full)
- Thread-safe register/unregister/subscribe/broadcast

**Broadcast flow:**
```go
hub.Broadcast("ticks:BTC/USDT", payload)
  → finds all clients subscribed to "ticks:BTC/USDT"
  → non-blocking send to each client's channel
  → client's write goroutine serializes to WebSocket
```

---

## Frontend Usage

```typescript
// In App.tsx or a custom hook
const ws = new WebSocket(import.meta.env.VITE_WS_URL + '/ws')

ws.onopen = () => {
  ws.send(JSON.stringify({ type: 'subscribe', channel: 'ticks:BTC/USDT' }))
}

ws.onmessage = (event) => {
  const msg = JSON.parse(event.data)
  if (msg.type === 'tick') {
    useMarketStore.getState().updateTick(msg.data)
  }
}

// Heartbeat
setInterval(() => ws.send(JSON.stringify({ type: 'ping' })), 30_000)
```
