# Phase 4 — Slow-Motion Replay Engine: Design

**Date:** 2026-02-25
**Status:** Approved

---

## Goal

Re-watch a completed backtest candle-by-candle in real time, with play/pause/seek/speed controls and a bookmark system for saving annotated moments to MySQL for future research.

---

## Architecture Overview

```
Browser (Backtest Page)
  │
  │  POST /api/v1/backtest/runs/:id/replay  → { replay_id }
  │  WS  /ws/replay/:replay_id  (bidirectional)
  │
  ▼
Backend Replay Handler
  ├── sync.Map[replay_id → *ReplaySession]   (in-memory, lives with WS)
  │     └── goroutine: ticks at speed, sends candles → WS client
  │          ← receives control msgs (pause/resume/step/seek/set_speed)
  │
  └── MySQL  replay_bookmarks                (persistent)
        run_id · candle_index · label · note · chart_snapshot (base64 PNG)
```

**Key decisions:**
- Sessions are in-memory only — they are ephemeral and live only while the WS is connected. No value in persisting them.
- Bookmarks are MySQL — they need to survive server restarts and are the research artifact.
- Seek delivers a single `seek_snapshot` message (all candles/trades/signals up to N) rather than re-streaming N individual messages. This keeps chart rebuild fast.
- Chart snapshot is captured client-side via `canvas.toDataURL('image/png')` and POSTed as base64. No server-side rendering needed.

---

## Backend

### New GORM Model

```go
// internal/models/models.go
type ReplayBookmark struct {
    ID            uint64    `gorm:"primaryKey;autoIncrement"`
    UserID        uint64    `gorm:"index;default:1"`
    BacktestRunID uint64    `gorm:"index;not null"`
    CandleIndex   int       `gorm:"not null"`
    Label         string    `gorm:"size:255"`
    Note          string    `gorm:"type:text"`
    ChartSnapshot string    `gorm:"type:longtext"` // base64 PNG
    CreatedAt     time.Time
}
```

### In-Memory Session

```go
// internal/replay/session.go
type ReplaySession struct {
    RunID       uint64
    Candles     []registry.Candle
    Trades      map[int][]*models.Trade   // keyed by candle index
    Signals     map[int][]*models.Signal  // keyed by candle index
    CurrentIdx  int
    Speed       float64                   // 1.0 = 300ms/candle
    State       string                    // "idle" | "playing" | "paused" | "complete"
    ControlChan chan ControlMsg
}
```

### WebSocket Protocol

**Client → Server (control messages):**

| `type` | Extra fields | Description |
|---|---|---|
| `start` | — | Begin playback from current index |
| `pause` | — | Pause playback |
| `resume` | — | Resume playback |
| `step` | — | Advance exactly one candle |
| `set_speed` | `speed: float64` | Set multiplier (0.25–10.0) |
| `seek` | `index: int` | Jump to candle N |

**Server → Client (emit messages):**

| `type` | Payload | Description |
|---|---|---|
| `candle` | OHLCV + timestamp | Next candle in sequence |
| `signal` | direction, price, timestamp | Strategy signal at this candle |
| `trade_open` | entry price, qty, timestamp | Trade opened |
| `trade_close` | exit price, PnL, PnL%, timestamp | Trade closed |
| `equity_update` | value, timestamp | Current equity curve point |
| `seek_snapshot` | `{ candles[], trades[], signals[], equity[] }` | All history up to seek index |
| `status` | `{ state, index, total }` | Playback state update |

### New API Routes

```
POST   /api/v1/backtest/runs/:id/replay       → { replay_id }
WS     /ws/replay/:replay_id
POST   /api/v1/replay/bookmarks               → ReplayBookmark
GET    /api/v1/replay/bookmarks?run_id=N      → []ReplayBookmark
GET    /api/v1/replay/bookmarks/:id           → ReplayBookmark
DELETE /api/v1/replay/bookmarks/:id           → 204
```

### New Files

```
backend/internal/replay/
  session.go      ReplaySession struct + goroutine control logic
  manager.go      sync.Map registry, CreateSession / GetSession / DeleteSession
  handler.go      WS upgrade + read-loop (control msgs) + write-loop (candle stream)
backend/internal/api/
  replay.go       HTTP handlers: CreateReplay, CreateBookmark, ListBookmarks, GetBookmark, DeleteBookmark
```

### Replay Goroutine Logic

```
goroutine loop:
  for {
    select {
    case ctrl := <-session.ControlChan:
      handle: pause/resume toggles ticker, step sends one candle, seek emits seek_snapshot, set_speed resets ticker interval
    case <-ticker.C:
      if state == playing:
        emit candle[currentIdx] + any signals/trades at currentIdx + equity_update
        currentIdx++
        if currentIdx >= len(candles): emit status{complete}, return
    }
  }
```

---

## Frontend

### Layout

Full-screen overlay that mounts on top of the Backtest page (no route change).

```
+────────────────────────────────────────────────────────────+
│  REPLAY  ·  EMA Crossover / BTCUSDT / 1h        [⌘ Save] [✕]│
│                                                            │
│  ┌──────────────────────────────────────────────────────┐ │
│  │                                                      │ │
│  │              Candlestick Chart (builds up)           │ │
│  │          ▲ buy/sell markers appear as streamed       │ │
│  │                                                      │ │
│  │                          ┌─────────────────────┐    │ │
│  │                          │  Equity mini-chart  │    │ │
│  │                          │  $10,420  +4.2%     │    │ │
│  │                          └─────────────────────┘    │ │
│  └──────────────────────────────────────────────────────┘ │
│                                                            │
│  [|◀]  [◀◀]  [▶ Play]  ──●────────────────── 47 / 312    │
│         0.25x  0.5x  [1x]  2x  5x  10x    Jan 15 14:00   │
│                                                            │
│  ┌─ Signal toast (bottom-right, 8s auto-dismiss) ───────┐ │
│  │  🟢 BUY  ·  BTCUSDT  ·  $42,150                     │ │
│  └──────────────────────────────────────────────────────┘ │
+────────────────────────────────────────────────────────────+
```

### Component Tree

```
<BacktestPage>
  └── <ReplayOverlay runId={id} onClose={...}>
        ├── <ReplayChart />               lightweight-charts, append-only updates
        ├── <EquityMiniChart />           Recharts LineChart, bottom-right corner
        ├── <ReplayControlBar />          play/pause/step/seek slider/speed chips
        ├── <SignalToast />               stacked, 8s auto-dismiss, bottom-right
        └── <BookmarkModal />             label + note fields, captures canvas on confirm
```

### Store Changes (`backtestStore`)

New fields added:
```ts
replayActive: boolean
replayId: string | null
replayState: 'idle' | 'playing' | 'paused' | 'complete'
replayIndex: number
replayTotal: number
replaySpeed: number
replayCandles: Candle[]        // append-only as candles stream in
replayEquity: EquityPoint[]    // append-only
replaySignals: Signal[]        // accumulated
```

### New Hook: `useReplayWS(replayId)`

- Opens WS on mount, closes on unmount or overlay close
- Dispatches incoming messages to the store
- Exposes `sendControl(msg)` used by control bar buttons
- On `seek_snapshot`: bulk-updates store (replace candles/equity/signals up to index)

### Bookmark Flow

1. User clicks "Save" → `BookmarkModal` opens with label + note inputs
2. On confirm: `canvas.toDataURL('image/png')` captures current chart canvas
3. POST `/api/v1/replay/bookmarks` with `{ run_id, candle_index, label, note, chart_snapshot }`
4. Success toast shown; bookmark appears in new "Bookmarks" tab on Backtest results panel

### New Files

```
frontend/src/components/replay/
  ReplayOverlay.tsx
  ReplayChart.tsx
  EquityMiniChart.tsx
  ReplayControlBar.tsx
  SignalToast.tsx
  BookmarkModal.tsx
frontend/src/hooks/
  useReplayWS.ts
frontend/src/api/
  replay.ts          createReplay(), createBookmark(), listBookmarks(), deleteBookmark()
```

---

## Testing

### Backend

| Test | Verifies |
|---|---|
| `TestReplaySession_StepAdvancesIndex` | `step` increments `CurrentIdx` by 1, emits exactly one candle |
| `TestReplaySession_SeekJumpsToIndex` | `seek{index:N}` sets `CurrentIdx=N`, emits `seek_snapshot` with correct slice |
| `TestReplaySession_SpeedChangesInterval` | `set_speed{2.0}` halves the tick interval |
| `TestReplaySession_PauseStopsEmission` | No candle messages emitted while `State=paused` |
| `TestReplaySession_CompleteOnLastCandle` | After last candle, emits `status{state:"complete"}` |
| `TestBookmark_CreateAndFetch` | POST bookmark persists; GET returns correct fields |
| `TestBookmark_DeleteRemovesRecord` | DELETE returns 204, record gone from DB |

### Frontend (Vitest)

- `ReplayControlBar` renders correct disabled states (step disabled when complete, play disabled when playing)
- `useReplayWS` dispatches `candle` messages to store correctly
- `BookmarkModal` calls `canvas.toDataURL` and includes result in POST body

---

## Sub-Phase Breakdown (for phases.md)

| Sub-phase | Scope |
|---|---|
| 4.1 | Backend: `ReplaySession`, `ReplayManager`, goroutine control logic + unit tests |
| 4.2 | Backend: WS handler, HTTP handlers, routes wired + API tests |
| 4.3 | Backend: `ReplayBookmark` model, bookmark CRUD API + tests |
| 4.4 | Frontend: `ReplayOverlay`, `ReplayChart`, `useReplayWS` hook, store fields |
| 4.5 | Frontend: `ReplayControlBar`, `EquityMiniChart`, `SignalToast` |
| 4.6 | Frontend: `BookmarkModal`, bookmark API calls, Bookmarks tab on Backtest results |
