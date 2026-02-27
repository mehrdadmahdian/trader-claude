# Phase 8 — Live Market Monitor & Signal Alerts: Design Document

**Date:** 2026-02-27
**Status:** Approved

---

## Overview

Phase 8 adds a live strategy monitoring system: users define monitors (strategy + symbol + timeframe) and the backend polls for new candles, runs the strategy, and emits real-time signals. The frontend shows a Monitor page with card-based status, a signal history table, and live toast notifications.

---

## Decisions Made

| Topic | Decision |
|---|---|
| Execution path | Polling only — `GetCandles` every N seconds. Streaming (SubscribeTicks) deferred. |
| Scheduling | `monitor.Manager` singleton with `time.AfterFunc` per active monitor, submits work to existing worker pool |
| WS design | Multiplexed — client sends `{action:"subscribe", monitor_ids:[...]}`, server filters per connection |
| Strategy state | Persisted to Redis key `monitor:{id}:state` (JSON, no TTL) to survive server restarts |
| Warm-start | Load last 200 candles on first poll; subsequent polls fetch only since `LastPolledAt` |
| Signal routing | In-app notification only for Phase 8; Telegram/webhook deferred to Phase 9 |
| Expanded card | Signal history table only; mini chart deferred |
| New deps | None — reuses existing packages (adapter, worker, price, registry, ws) |

---

## Section 1: New DB Models

### `models.go` additions

```go
// MonitorStatus is the lifecycle state of a monitor
type MonitorStatus string

const (
    MonitorStatusActive  MonitorStatus = "active"
    MonitorStatusPaused  MonitorStatus = "paused"
    MonitorStatusStopped MonitorStatus = "stopped"
)

// Monitor stores a live strategy-monitoring configuration
type Monitor struct {
    ID              int64          `gorm:"primaryKey;autoIncrement" json:"id"`
    Name            string         `gorm:"type:varchar(200);not null" json:"name"`
    AdapterID       string         `gorm:"type:varchar(20);not null" json:"adapter_id"`
    Symbol          string         `gorm:"type:varchar(20);not null;index" json:"symbol"`
    Market          string         `gorm:"type:varchar(20);not null" json:"market"`
    Timeframe       string         `gorm:"type:varchar(10);not null" json:"timeframe"`
    StrategyName    string         `gorm:"type:varchar(100);not null;index" json:"strategy_name"`
    Params          JSON           `gorm:"type:json" json:"params"`
    Status          MonitorStatus  `gorm:"type:varchar(20);not null;default:'active';index" json:"status"`
    NotifyInApp     bool           `gorm:"default:true" json:"notify_in_app"`
    LastPolledAt    *time.Time     `json:"last_polled_at,omitempty"`
    LastSignalAt    *time.Time     `json:"last_signal_at,omitempty"`
    LastSignalDir   string         `gorm:"type:varchar(10)" json:"last_signal_dir"`
    LastSignalPrice float64        `gorm:"type:decimal(20,8)" json:"last_signal_price"`
    CreatedAt       time.Time      `json:"created_at"`
    UpdatedAt       time.Time      `json:"updated_at"`
    DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Monitor) TableName() string { return "monitors" }

// MonitorSignal stores a strategy signal emitted by a live monitor
type MonitorSignal struct {
    ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
    MonitorID int64     `gorm:"not null;index" json:"monitor_id"`
    Direction string    `gorm:"type:varchar(10);not null" json:"direction"` // "long","short","flat"
    Price     float64   `gorm:"type:decimal(20,8);not null" json:"price"`
    Strength  float64   `gorm:"type:decimal(5,4);not null" json:"strength"`
    Metadata  JSON      `gorm:"type:json" json:"metadata,omitempty"`
    CreatedAt time.Time `gorm:"index" json:"created_at"`
}

func (MonitorSignal) TableName() string { return "monitor_signals" }
```

Also add to the `NotificationType` constants:
```go
NotificationTypeSignal NotificationType = "signal"
```

---

## Section 2: Backend — `internal/monitor/` Package

### File layout

```
internal/monitor/
  manager.go        Manager struct, Start(ctx), Add, Remove, Pause, Resume
  poller.go         executePoll(ctx, monitorID) — runs inside worker pool
  interval.go       calcPollInterval(timeframe string) time.Duration
  manager_test.go   lifecycle + signal generation tests
```

### Poll interval table

| Timeframe | Poll every |
|---|---|
| 1m | 30s |
| 5m | 30s |
| 15m | 90s |
| 1h | 6m |
| 4h | 24m |
| 1d | 1h |

Default (unknown timeframe): 60s.

### Manager struct

```go
type Manager struct {
    db   *gorm.DB
    rdb  *redis.Client
    ds   *adapter.DataService
    pool *worker.WorkerPool
    mu   sync.Mutex
    timers map[int64]*time.Timer  // monitorID → pending next-poll timer
    active sync.Map               // monitorID → struct{} (dedup: skip if poll already running)
}

func NewManager(db *gorm.DB, rdb *redis.Client, ds *adapter.DataService, pool *worker.WorkerPool) *Manager

// Start loads all monitors with status="active" from DB and schedules each.
func (m *Manager) Start(ctx context.Context)

// Add schedules polling for a newly created monitor.
func (m *Manager) Add(ctx context.Context, monitorID int64)

// Remove cancels the timer and marks the monitor stopped.
func (m *Manager) Remove(monitorID int64)

// Pause cancels the timer without changing DB status (caller updates DB).
func (m *Manager) Pause(monitorID int64)

// Resume re-schedules a paused monitor (caller has set DB status to active).
func (m *Manager) Resume(ctx context.Context, monitorID int64)
```

### scheduleNext (internal)

```go
func (m *Manager) scheduleNext(ctx context.Context, id int64, interval time.Duration) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.timers[id] = time.AfterFunc(interval, func() {
        if _, alreadyRunning := m.active.LoadOrStore(id, struct{}{}); alreadyRunning {
            m.scheduleNext(ctx, id, interval) // reschedule, skip this tick
            return
        }
        m.pool.Submit(func() {
            defer m.active.Delete(id)
            m.executePoll(ctx, id)
            m.scheduleNext(ctx, id, interval)
        })
    })
}
```

### executePoll flow (poller.go)

```
1. Load monitor from DB (return if not found or status != active)
2. Load strategy via registry.Strategies().Get(monitor.StrategyName)
3. Init strategy with monitor.Params
4. Load StrategyState from Redis key "monitor:{id}:state" (JSON unmarshal)
5. If no state (first poll):
   a. Fetch last 200 candles via ds.GetCandles(ctx, adapterID, symbol, market, timeframe, now-200*interval, now)
   b. Run strategy.OnCandle on each candle (warm-start); only save signals from the last candle
   c. Create empty StrategyState
   Else:
   a. Fetch candles since monitor.LastPolledAt (or last 5m ago as minimum window)
   b. Run strategy.OnCandle on each new candle
6. If final candle yields signal with Direction != "flat":
   a. Insert MonitorSignal into DB
   b. Update monitor.LastSignalAt, LastSignalDir, LastSignalPrice
   c. If monitor.NotifyInApp: Insert Notification{Type:"signal", Title:"[Direction] [Symbol]", Body:"Strategy [name] on [symbol] [timeframe]: [direction] @ $[price]"}
   d. Publish MonitorSignalEvent{MonitorID, Signal} JSON to Redis pubsub "monitor:signals"
7. Update monitor.LastPolledAt = time.Now()
8. Save StrategyState to Redis "monitor:{id}:state" (json.Marshal)
```

### Tests (manager_test.go)

- `TestCalcPollInterval` — verify each timeframe bucket
- `TestManagerStartPauseResume` — mock pool, start with 2 monitors, pause one, resume it
- `TestExecutePollNoSignal` — mock strategy returns flat; no signal inserted
- `TestExecutePollSignal` — mock strategy returns long; signal saved, notification created, Redis message published

---

## Section 3: Monitor API

### New file: `internal/api/monitor_handler.go`

Handler struct:
```go
type monitorHandler struct {
    db  *gorm.DB
    mgr *monitor.Manager
}
```

Endpoints:
```
POST   /api/v1/monitors              createMonitor
GET    /api/v1/monitors              listMonitors
GET    /api/v1/monitors/:id          getMonitor
PUT    /api/v1/monitors/:id          updateMonitor
DELETE /api/v1/monitors/:id          deleteMonitor
PATCH  /api/v1/monitors/:id/toggle   toggleMonitor (active ↔ paused)
GET    /api/v1/monitors/:id/signals  listSignals (paginated: limit, offset)
```

`createMonitor`:
- Parse body: `{name, adapter_id, symbol, market, timeframe, strategy_name, params, notify_in_app}`
- Validate strategy_name exists via `registry.Strategies().Get(name)`
- Insert `Monitor` record with `Status: "active"`
- Call `mgr.Add(ctx, monitor.ID)`
- Return 201 + monitor JSON

`toggleMonitor`:
- Load monitor; if active → set paused + `mgr.Pause(id)`; if paused → set active + `mgr.Resume(ctx, id)`
- Return updated monitor

`deleteMonitor`:
- Call `mgr.Remove(id)`, soft-delete via `db.Delete(&models.Monitor{}, id)`
- Return 204

`listSignals`:
- `SELECT * FROM monitor_signals WHERE monitor_id=? ORDER BY created_at DESC LIMIT ? OFFSET ?`
- Returns `{data: [...], total, limit, offset}`

### RegisterRoutes changes

Add `mgr *monitor.Manager` parameter. Wire:
```go
mnh := newMonitorHandler(db, mgr)
v1.Post("/monitors", mnh.createMonitor)
v1.Get("/monitors", mnh.listMonitors)
v1.Get("/monitors/:id", mnh.getMonitor)
v1.Put("/monitors/:id", mnh.updateMonitor)
v1.Delete("/monitors/:id", mnh.deleteMonitor)
v1.Patch("/monitors/:id/toggle", mnh.toggleMonitor)
v1.Get("/monitors/:id/signals", mnh.listSignals)
app.Get("/ws/monitors/signals", websocket.New(mnh.signalsWS))
```

### WebSocket: `/ws/monitors/signals`

New file: `internal/api/monitor_ws.go` (method on `monitorHandler`)

Protocol:
```
Client → Server: {"action":"subscribe",   "monitor_ids":[1,2,3]}
Client → Server: {"action":"unsubscribe", "monitor_ids":[1]}
Server → Client: {"monitor_id":1, "id":42, "direction":"long", "price":82150.0, "strength":0.87, "created_at":"..."}
```

Handler logic:
1. Subscribe to Redis pubsub channel `monitor:signals`
2. Maintain `subscribed map[int64]bool` per WS connection
3. Read loop: listen on Redis pubsub + client messages concurrently (goroutine each)
4. On Redis message: decode JSON signal, check `signal.MonitorID` in subscribed → write to WS
5. On client message: parse action, update subscribed map
6. On WS close: unsubscribe Redis

---

## Section 4: Frontend

### New TypeScript types (append to `types/index.ts`)

```ts
export interface Monitor {
  id: number
  name: string
  adapter_id: string
  symbol: string
  market: string
  timeframe: string
  strategy_name: string
  params: Record<string, unknown>
  status: 'active' | 'paused' | 'stopped'
  notify_in_app: boolean
  last_polled_at?: string
  last_signal_at?: string
  last_signal_dir?: string
  last_signal_price?: number
  created_at: string
  updated_at: string
}

export interface MonitorSignal {
  id: number
  monitor_id: number
  direction: 'long' | 'short' | 'flat'
  price: number
  strength: number
  metadata?: Record<string, unknown>
  created_at: string
}
```

### Zustand store (append to `stores/index.ts`)

```ts
interface MonitorState {
  monitors: Monitor[]
  setMonitors: (m: Monitor[]) => void
  addMonitor: (m: Monitor) => void
  updateMonitor: (m: Monitor) => void
  removeMonitor: (id: number) => void
  pendingSignals: MonitorSignal[]
  addSignal: (s: MonitorSignal) => void
  clearSignal: (id: number) => void
}

export const useMonitorStore = create<MonitorState>()((set) => ({
  monitors: [],
  setMonitors: (monitors) => set({ monitors }),
  addMonitor: (m) => set((s) => ({ monitors: [...s.monitors, m] })),
  updateMonitor: (m) => set((s) => ({ monitors: s.monitors.map((x) => (x.id === m.id ? m : x)) })),
  removeMonitor: (id) => set((s) => ({ monitors: s.monitors.filter((x) => x.id !== id) })),
  pendingSignals: [],
  addSignal: (sig) => set((s) => ({ pendingSignals: [...s.pendingSignals, sig] })),
  clearSignal: (id) => set((s) => ({ pendingSignals: s.pendingSignals.filter((x) => x.id !== id) })),
}))
```

### API client functions (new file `api/monitors.ts`)

```ts
export const getMonitors = () => api.get<Monitor[]>('/api/v1/monitors').then(r => r.data)
export const createMonitor = (body: Partial<Monitor>) => api.post<Monitor>('/api/v1/monitors', body).then(r => r.data)
export const deleteMonitor = (id: number) => api.delete(`/api/v1/monitors/${id}`)
export const toggleMonitor = (id: number) => api.patch<Monitor>(`/api/v1/monitors/${id}/toggle`).then(r => r.data)
export const getMonitorSignals = (id: number, params: { limit?: number; offset?: number }) =>
  api.get<PaginatedResponse<MonitorSignal>>(`/api/v1/monitors/${id}/signals`, { params }).then(r => r.data)
```

### React Query hooks (new file `hooks/useMonitors.ts`)

- `useMonitors()` — `useQuery(['monitors'], getMonitors)`
- `useCreateMonitor()` — mutation, invalidates `['monitors']` on success
- `useDeleteMonitor()` — mutation, invalidates `['monitors']`
- `useToggleMonitor()` — mutation, invalidates `['monitors']`
- `useMonitorSignals(id)` — `useQuery(['monitor-signals', id], ...)`

### WS hook (new file `hooks/useMonitorSignalsWS.ts`)

- Connects to `${VITE_WS_URL}/ws/monitors/signals`
- On open: sends subscribe message with all active monitor IDs from monitorStore
- On message: parses MonitorSignal JSON, calls `monitorStore.addSignal(signal)`
- Used in `Monitor.tsx` — opened when page mounts, closed on unmount

### Monitor Page (`pages/Monitor.tsx`)

Layout:
```
Header: "Live Monitors"  [+ Create Monitor] button

Card grid (responsive: 1-col mobile, 2-col tablet, 3-col desktop):
  MonitorCard:
    ● status dot (green pulse = active, gray = paused)
    Name + symbol badge
    Strategy name + timeframe + adapter
    Last signal: "↑ LONG @ $82,150  2h ago"  (or "No signals yet")
    Actions: [Pause/Resume]  [Delete]

Click card → expanded panel below (accordion):
  Signal History table:
    Columns: Time | Direction (colored chip) | Price | Strength
    Pagination: 20 per page, prev/next buttons
```

### Create Monitor Modal

Fields:
- Name input (auto-fill: `{StrategyName} {SYMBOL} {timeframe}` on change)
- Adapter selector (`useMarkets()` hook)
- Symbol text input
- Market auto-detected from adapter
- Timeframe selector (button group: 1m 5m 15m 1h 4h 1d)
- Strategy card grid (from `useStrategies()`)
- Auto-generated param form (copy pattern from Backtest page)
- Notify in-app toggle (default on)
- [Cancel]  [Create Monitor] buttons

### SignalToast (`components/SignalToast.tsx`)

- Reads `monitorStore.pendingSignals`
- Renders up to 3 stacked toasts in bottom-right corner
- Each toast: colored bar (green=long, red=short), symbol, direction, price
- 8s auto-dismiss via `setTimeout`; ✕ button calls `clearSignal(id)`
- Rendered in `Layout.tsx` (always present)

---

## Implementation Order

```
B1: Add Monitor + MonitorSignal models → autoMigrate       (no deps)
B2: interval.go + calcPollInterval                         (no deps)
B3: manager.go + poller.go (core poll loop)                (needs B1, B2)
B4: manager_test.go                                        (needs B3)
B5: monitor_handler.go (CRUD + signals API)                (needs B1, B3)
B6: monitor_ws.go (WS multiplexed)                        (needs B5)
B7: RegisterRoutes wiring + main.go                        (needs B5, B6)
B8: monitor_handler_test.go                                (needs B5)
F1: types/index.ts + stores/index.ts additions             (no deps)
F2: api/monitors.ts + hooks/useMonitors.ts                 (needs F1)
F3: hooks/useMonitorSignalsWS.ts                           (needs F1, F2)
F4: components/SignalToast.tsx + Layout.tsx wiring          (needs F1)
F5: pages/Monitor.tsx (cards + expanded table + modal)     (needs F2, F3, F4)
```

---

## Testing Requirements

| Task | Tests |
|---|---|
| B2 | `TestCalcPollInterval` — all timeframe buckets |
| B3 | `TestExecutePollNoSignal`, `TestExecutePollSignal`, `TestManagerStartPauseResume` |
| B5 | CRUD endpoints, toggle, signal pagination |
| B6 | Subscribe/unsubscribe filtering on WS |
| F5 | Card render, toggle, delete, create modal, signal table pagination |
