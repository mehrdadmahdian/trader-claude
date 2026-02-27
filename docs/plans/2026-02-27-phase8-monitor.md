# Phase 8 — Live Market Monitor & Signal Alerts — Atomic Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a live strategy monitor system: users define monitors (strategy + symbol + timeframe), the backend polls for new candles on a schedule, runs the strategy, emits real-time signals, and the frontend shows a Monitor page with card grid, signal history, and live toast notifications.

**Architecture:** A `monitor.Manager` singleton (started in `main.go`) uses `time.AfterFunc` per active monitor to schedule polling ticks. When a tick fires, a `worker.Job` is submitted to the existing worker pool. The job fetches the last 200 candles (warm-start every time — strategies hold state in struct fields, not in `StrategyState`), runs the strategy, and emits signals from candles newer than `LastPolledAt`. Signals are saved to DB, create in-app Notifications, and are published to Redis pubsub `monitor:signals`. A new WS endpoint subscribes to that channel and forwards only signals for the client's subscribed monitor IDs.

**Tech Stack:** Go 1.24 (Fiber v2, GORM, go-redis, worker pool), React 18 (React Query v5, Zustand, Radix UI, Tailwind, lucide-react). No new Go or npm dependencies.

**Execution strategy:** Each task = one focused action on ≤ 3 files. Give Haiku the task text + only the files listed under "Read first". No codebase exploration needed.

**Design doc:** `docs/plans/2026-02-27-phase8-monitor-design.md`

---

## BACKEND TASKS

---

### Task B1: Add Monitor + MonitorSignal models to models.go + autoMigrate

**Read first:**
- `backend/internal/models/models.go` (full file — 397 lines)
- `backend/cmd/server/main.go` (lines 194–210)

**Files to modify:**
- `backend/internal/models/models.go`
- `backend/cmd/server/main.go`

---

**Step 1: Add `NotificationTypeSignal` to the NotificationType constants**

In `models.go`, find this block (around line 348):
```go
const (
	NotificationTypeAlert     NotificationType = "alert"
	NotificationTypeTrade     NotificationType = "trade"
	NotificationTypeSystem    NotificationType = "system"
	NotificationTypeBacktest  NotificationType = "backtest"
)
```
Replace it with:
```go
const (
	NotificationTypeAlert     NotificationType = "alert"
	NotificationTypeTrade     NotificationType = "trade"
	NotificationTypeSystem    NotificationType = "system"
	NotificationTypeBacktest  NotificationType = "backtest"
	NotificationTypeSignal    NotificationType = "signal"
)
```

---

**Step 2: Append Monitor + MonitorSignal structs to models.go**

After `func (WatchList) TableName() string { return "watch_lists" }` (line 380) and before `// --- ReplayBookmark ---`, insert:

```go
// --- Monitor ---

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

---

**Step 3: Add `&models.Monitor{}` and `&models.MonitorSignal{}` to autoMigrate in main.go**

Find the `autoMigrate` function (lines 194–210). Add the two new models:

```go
func autoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.Symbol{},
		&models.Candle{},
		&models.StrategyDef{},
		&models.Backtest{},
		&models.Trade{},
		&models.Portfolio{},
		&models.Position{},
		&models.Transaction{},
		&models.Alert{},
		&models.Notification{},
		&models.NewsItem{},
		&models.WatchList{},
		&models.ReplayBookmark{},
		&models.Monitor{},
		&models.MonitorSignal{},
	)
}
```

---

**Step 4: Verify no compile errors**

```bash
make backend-fmt
docker compose exec backend go build ./...
```

Expected: no errors.

---

**Step 5: Commit**

```bash
git add backend/internal/models/models.go backend/cmd/server/main.go
git commit -m "feat(phase8): add Monitor + MonitorSignal models and autoMigrate"
```

---

### Task B2: Create internal/monitor/interval.go

**Read first:** Nothing — this is a standalone pure-Go file.

**Files to create:**
- `backend/internal/monitor/interval.go`

---

**Step 1: Create the file**

Create `backend/internal/monitor/interval.go`:

```go
package monitor

import "time"

// calcPollInterval returns how often a monitor should poll for new candles.
// Rule: timeframe / 10, minimum 30s.
func calcPollInterval(timeframe string) time.Duration {
	switch timeframe {
	case "1m":
		return 30 * time.Second
	case "5m":
		return 30 * time.Second
	case "15m":
		return 90 * time.Second
	case "1h":
		return 6 * time.Minute
	case "4h":
		return 24 * time.Minute
	case "1d":
		return 1 * time.Hour
	default:
		return 60 * time.Second
	}
}

// tfDuration returns the wall-clock duration of a timeframe string.
// Used to compute the warm-start window.
func tfDuration(tf string) time.Duration {
	switch tf {
	case "1m":
		return time.Minute
	case "5m":
		return 5 * time.Minute
	case "15m":
		return 15 * time.Minute
	case "30m":
		return 30 * time.Minute
	case "1h":
		return time.Hour
	case "4h":
		return 4 * time.Hour
	case "1d":
		return 24 * time.Hour
	case "1w":
		return 7 * 24 * time.Hour
	default:
		return time.Hour
	}
}
```

---

**Step 2: Create the test file**

Create `backend/internal/monitor/interval_test.go`:

```go
package monitor

import (
	"testing"
	"time"
)

func TestCalcPollInterval(t *testing.T) {
	cases := []struct {
		tf   string
		want time.Duration
	}{
		{"1m", 30 * time.Second},
		{"5m", 30 * time.Second},
		{"15m", 90 * time.Second},
		{"1h", 6 * time.Minute},
		{"4h", 24 * time.Minute},
		{"1d", 1 * time.Hour},
		{"unknown", 60 * time.Second},
	}
	for _, tc := range cases {
		got := calcPollInterval(tc.tf)
		if got != tc.want {
			t.Errorf("calcPollInterval(%q) = %v, want %v", tc.tf, got, tc.want)
		}
	}
}

func TestTfDuration(t *testing.T) {
	if tfDuration("1h") != time.Hour {
		t.Error("1h should be 1 hour")
	}
	if tfDuration("1d") != 24*time.Hour {
		t.Error("1d should be 24 hours")
	}
}
```

---

**Step 3: Run tests**

```bash
docker compose exec backend go test ./internal/monitor/... -v -run TestCalcPollInterval
```

Expected:
```
--- PASS: TestCalcPollInterval (0.00s)
PASS
```

---

**Step 4: Commit**

```bash
git add backend/internal/monitor/
git commit -m "feat(phase8): add monitor interval helpers"
```

---

### Task B3: Create internal/monitor/manager.go

**Read first:**
- `backend/internal/models/models.go` (the Monitor struct, lines 382–420)
- `backend/internal/worker/pool.go` (full — 79 lines)
- `backend/internal/alert/evaluator.go` (for pattern reference — full — 151 lines)

**Files to create:**
- `backend/internal/monitor/manager.go`

---

**Step 1: Create the file**

Create `backend/internal/monitor/manager.go`:

```go
package monitor

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/adapter"
	"github.com/trader-claude/backend/internal/models"
	"github.com/trader-claude/backend/internal/worker"
)

// Manager schedules live strategy polls for all active monitors.
// Each active monitor gets a time.AfterFunc timer; when it fires the poll
// job is submitted to the shared worker pool, then the timer is rescheduled.
type Manager struct {
	db   *gorm.DB
	rdb  *redis.Client
	ds   *adapter.DataService
	pool *worker.WorkerPool

	mu     sync.Mutex
	timers map[int64]*time.Timer // monitorID → pending next-poll timer
	active sync.Map              // monitorID → struct{} (set while poll job is running)
}

// NewManager creates a Manager.
func NewManager(db *gorm.DB, rdb *redis.Client, ds *adapter.DataService, pool *worker.WorkerPool) *Manager {
	return &Manager{
		db:     db,
		rdb:    rdb,
		ds:     ds,
		pool:   pool,
		timers: make(map[int64]*time.Timer),
	}
}

// Start loads all active monitors from the DB and schedules a poll for each.
// Call this once during server startup.
func (m *Manager) Start(ctx context.Context) {
	var monitors []models.Monitor
	if err := m.db.WithContext(ctx).
		Where("status = ?", models.MonitorStatusActive).
		Find(&monitors).Error; err != nil {
		log.Printf("[monitor] failed to load monitors on start: %v", err)
		return
	}
	for _, mon := range monitors {
		m.scheduleNext(ctx, mon.ID, calcPollInterval(mon.Timeframe))
	}
	log.Printf("[monitor] started %d active monitors", len(monitors))
}

// Add schedules polling for a newly created monitor.
// Call this after inserting the monitor record into the DB.
func (m *Manager) Add(ctx context.Context, monitorID int64, timeframe string) {
	m.scheduleNext(ctx, monitorID, calcPollInterval(timeframe))
}

// Remove cancels the timer for a monitor (called on delete).
func (m *Manager) Remove(monitorID int64) {
	m.cancelTimer(monitorID)
	m.active.Delete(monitorID)
}

// Pause cancels the timer without changing DB status.
// The API handler is responsible for setting status = "paused" in the DB.
func (m *Manager) Pause(monitorID int64) {
	m.cancelTimer(monitorID)
}

// Resume re-schedules a paused monitor.
// The API handler is responsible for setting status = "active" in the DB before calling this.
func (m *Manager) Resume(ctx context.Context, monitorID int64, timeframe string) {
	m.scheduleNext(ctx, monitorID, calcPollInterval(timeframe))
}

// cancelTimer stops and removes the timer for a monitor.
func (m *Manager) cancelTimer(monitorID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.timers[monitorID]; ok {
		t.Stop()
		delete(m.timers, monitorID)
	}
}

// scheduleNext sets up a time.AfterFunc for the next poll of monitorID.
// When the timer fires:
//  1. If a poll is already running (active map), reschedule and return.
//  2. Otherwise mark active, submit the job to the pool, then reschedule after the job.
func (m *Manager) scheduleNext(ctx context.Context, monitorID int64, interval time.Duration) {
	m.mu.Lock()
	// Cancel any existing timer before creating a new one.
	if t, ok := m.timers[monitorID]; ok {
		t.Stop()
	}
	m.timers[monitorID] = time.AfterFunc(interval, func() {
		// Skip if a previous poll is still running.
		if _, loaded := m.active.LoadOrStore(monitorID, struct{}{}); loaded {
			m.scheduleNext(ctx, monitorID, interval)
			return
		}
		submitted := m.pool.Submit(worker.Job{
			Name: fmt.Sprintf("monitor-poll-%d", monitorID),
			Task: func(jobCtx context.Context) error {
				defer m.active.Delete(monitorID)
				executePoll(jobCtx, m.db, m.rdb, m.ds, monitorID)
				// Reschedule after the poll completes.
				m.scheduleNext(ctx, monitorID, interval)
				return nil
			},
		})
		if !submitted {
			// Pool is full or stopped — clear active flag and try again later.
			m.active.Delete(monitorID)
			m.scheduleNext(ctx, monitorID, interval)
		}
	})
	m.mu.Unlock()
}
```

---

**Step 2: Verify compile**

```bash
docker compose exec backend go build ./internal/monitor/...
```

Expected: no errors.

---

**Step 3: Commit**

```bash
git add backend/internal/monitor/manager.go
git commit -m "feat(phase8): add monitor.Manager with timer-based scheduling"
```

---

### Task B4: Create internal/monitor/poller.go

**Read first:**
- `backend/internal/registry/interfaces.go` (full — 124 lines)
- `backend/internal/registry/registry.go` (lines 113–145 for `Strategies().Create`)
- `backend/internal/adapter/dataservice.go` (lines 41–84 for `GetCandles` signature)
- `backend/internal/models/models.go` (Monitor + MonitorSignal structs)
- `backend/internal/alert/evaluator.go` (lines 84–113 for notification creation pattern)

**Files to create:**
- `backend/internal/monitor/poller.go`

---

**Step 1: Create the file**

Create `backend/internal/monitor/poller.go`:

```go
package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/adapter"
	"github.com/trader-claude/backend/internal/models"
	"github.com/trader-claude/backend/internal/registry"
)

const (
	signalChannel = "monitor:signals"
	warmupCandles = 200
)

// signalEvent is the payload published to Redis and forwarded over WebSocket.
type signalEvent struct {
	ID        int64       `json:"id"`
	MonitorID int64       `json:"monitor_id"`
	Direction string      `json:"direction"`
	Price     float64     `json:"price"`
	Strength  float64     `json:"strength"`
	Metadata  models.JSON `json:"metadata,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
}

// executePoll fetches the most recent candles, runs the strategy, and
// emits a signal if any new candles (since LastPolledAt) produce one.
//
// Strategy state is NOT persisted between polls — strategies keep state in
// struct fields and are re-warmed from the last 200 candles on every poll.
func executePoll(ctx context.Context, db *gorm.DB, rdb *redis.Client, ds *adapter.DataService, monitorID int64) {
	// 1. Load monitor
	var mon models.Monitor
	if err := db.WithContext(ctx).First(&mon, monitorID).Error; err != nil {
		log.Printf("[monitor %d] load failed: %v", monitorID, err)
		return
	}
	if mon.Status != models.MonitorStatusActive {
		return
	}

	// 2. Get adapter
	adapt, err := registry.Adapters().Get(mon.AdapterID)
	if err != nil {
		log.Printf("[monitor %d] adapter %q not found: %v", monitorID, mon.AdapterID, err)
		return
	}

	// 3. Create and initialise strategy
	strat, err := registry.Strategies().Create(mon.StrategyName)
	if err != nil {
		log.Printf("[monitor %d] strategy %q not found: %v", monitorID, mon.StrategyName, err)
		return
	}
	params := make(map[string]interface{})
	if mon.Params != nil {
		for k, v := range mon.Params {
			params[k] = v
		}
	}
	if err := strat.Init(params); err != nil {
		log.Printf("[monitor %d] strategy init failed: %v", monitorID, err)
		return
	}

	// 4. Compute candle window: last warmupCandles candles
	now := time.Now().UTC()
	tfDur := tfDuration(mon.Timeframe)
	from := now.Add(-time.Duration(warmupCandles) * tfDur)

	// 5. Fetch candles
	candles, err := ds.GetCandles(ctx, adapt, mon.Symbol, mon.Market, mon.Timeframe, from, now)
	if err != nil {
		log.Printf("[monitor %d] GetCandles failed: %v", monitorID, err)
		updateLastPolled(ctx, db, monitorID, now)
		return
	}
	if len(candles) == 0 {
		updateLastPolled(ctx, db, monitorID, now)
		return
	}

	// 6. Run strategy on all candles; collect signals from candles after LastPolledAt
	var state registry.StrategyState
	var newSignals []*registry.Signal

	for _, c := range candles {
		rc := registry.Candle{
			Symbol:    c.Symbol,
			Market:    c.Market,
			Timeframe: c.Timeframe,
			Timestamp: c.Timestamp,
			Open:      c.Open,
			High:      c.High,
			Low:       c.Low,
			Close:     c.Close,
			Volume:    c.Volume,
		}
		sig, err := strat.OnCandle(rc, &state)
		if err != nil {
			log.Printf("[monitor %d] strategy error on candle %v: %v", monitorID, c.Timestamp, err)
			continue
		}
		if sig == nil || sig.Direction == "flat" {
			continue
		}
		// Only collect signals from candles that are newer than the last poll.
		if mon.LastPolledAt == nil || c.Timestamp.After(*mon.LastPolledAt) {
			newSignals = append(newSignals, sig)
		}
	}

	// 7. Persist and broadcast each new signal
	for _, sig := range newSignals {
		emitSignal(ctx, db, rdb, mon, sig)
	}

	// 8. Update LastPolledAt
	updateLastPolled(ctx, db, monitorID, now)
}

// emitSignal saves a MonitorSignal, creates an in-app Notification (if enabled),
// and publishes the signal event to Redis pubsub.
func emitSignal(ctx context.Context, db *gorm.DB, rdb *redis.Client, mon models.Monitor, sig *registry.Signal) {
	// Save signal to DB
	meta := models.JSON{}
	for k, v := range sig.Metadata {
		meta[k] = v
	}
	ms := models.MonitorSignal{
		MonitorID: mon.ID,
		Direction: sig.Direction,
		Price:     sig.Price,
		Strength:  sig.Strength,
		Metadata:  meta,
		CreatedAt: sig.Timestamp,
	}
	if err := db.WithContext(ctx).Create(&ms).Error; err != nil {
		log.Printf("[monitor %d] failed to save signal: %v", mon.ID, err)
		return
	}

	// Update monitor's last signal fields
	now := time.Now()
	if err := db.WithContext(ctx).Model(&models.Monitor{}).Where("id = ?", mon.ID).Updates(map[string]interface{}{
		"last_signal_at":    now,
		"last_signal_dir":   sig.Direction,
		"last_signal_price": sig.Price,
	}).Error; err != nil {
		log.Printf("[monitor %d] failed to update last signal: %v", mon.ID, err)
	}

	// Create in-app notification if enabled
	if mon.NotifyInApp {
		dirLabel := "LONG"
		if sig.Direction == "short" {
			dirLabel = "SHORT"
		}
		notif := models.Notification{
			Type:  models.NotificationTypeSignal,
			Title: fmt.Sprintf("%s %s", dirLabel, mon.Symbol),
			Body: fmt.Sprintf("Strategy %q on %s %s: %s @ $%.4f",
				mon.StrategyName, mon.Symbol, mon.Timeframe, sig.Direction, sig.Price),
			Metadata: models.JSON{
				"monitor_id": mon.ID,
				"signal_id":  ms.ID,
				"symbol":     mon.Symbol,
				"direction":  sig.Direction,
				"price":      sig.Price,
			},
		}
		if err := db.WithContext(ctx).Create(&notif).Error; err != nil {
			log.Printf("[monitor %d] failed to create notification: %v", mon.ID, err)
		}
	}

	// Publish signal event to Redis pubsub
	evt := signalEvent{
		ID:        ms.ID,
		MonitorID: mon.ID,
		Direction: sig.Direction,
		Price:     sig.Price,
		Strength:  sig.Strength,
		Metadata:  meta,
		CreatedAt: ms.CreatedAt,
	}
	b, err := json.Marshal(evt)
	if err == nil {
		rdb.Publish(ctx, signalChannel, string(b))
	}
}

// updateLastPolled sets monitor.LastPolledAt = t in the DB.
func updateLastPolled(ctx context.Context, db *gorm.DB, monitorID int64, t time.Time) {
	if err := db.WithContext(ctx).Model(&models.Monitor{}).
		Where("id = ?", monitorID).
		Update("last_polled_at", t).Error; err != nil {
		log.Printf("[monitor %d] failed to update last_polled_at: %v", monitorID, err)
	}
}
```

---

**Step 2: Verify compile**

```bash
docker compose exec backend go build ./internal/monitor/...
```

Expected: no errors.

---

**Step 3: Commit**

```bash
git add backend/internal/monitor/poller.go
git commit -m "feat(phase8): add monitor poller with warm-start strategy execution"
```

---

### Task B5: Create internal/monitor/manager_test.go

**Read first:**
- `backend/internal/monitor/interval.go`
- `backend/internal/monitor/manager.go`

**Files to create:**
- `backend/internal/monitor/manager_test.go`

---

**Step 1: Create the test file**

Create `backend/internal/monitor/manager_test.go`:

```go
package monitor

import (
	"context"
	"testing"
	"time"
)

// TestCalcPollIntervalAllBuckets verifies every documented timeframe bucket.
func TestCalcPollIntervalAllBuckets(t *testing.T) {
	cases := []struct {
		tf   string
		want time.Duration
	}{
		{"1m", 30 * time.Second},
		{"5m", 30 * time.Second},
		{"15m", 90 * time.Second},
		{"1h", 6 * time.Minute},
		{"4h", 24 * time.Minute},
		{"1d", 1 * time.Hour},
		{"bogus", 60 * time.Second},
	}
	for _, tc := range cases {
		got := calcPollInterval(tc.tf)
		if got != tc.want {
			t.Errorf("calcPollInterval(%q) = %v, want %v", tc.tf, got, tc.want)
		}
	}
}

// TestTfDurationAllBuckets verifies warm-start duration helpers.
func TestTfDurationAllBuckets(t *testing.T) {
	cases := []struct {
		tf   string
		want time.Duration
	}{
		{"1m", time.Minute},
		{"5m", 5 * time.Minute},
		{"15m", 15 * time.Minute},
		{"30m", 30 * time.Minute},
		{"1h", time.Hour},
		{"4h", 4 * time.Hour},
		{"1d", 24 * time.Hour},
		{"1w", 7 * 24 * time.Hour},
		{"unknown", time.Hour},
	}
	for _, tc := range cases {
		got := tfDuration(tc.tf)
		if got != tc.want {
			t.Errorf("tfDuration(%q) = %v, want %v", tc.tf, got, tc.want)
		}
	}
}

// TestManagerPauseRemoveCancelTimer verifies that cancelTimer removes the entry
// from the timers map without panicking.
func TestManagerPauseRemoveCancelTimer(t *testing.T) {
	mgr := &Manager{
		timers: make(map[int64]*time.Timer),
	}

	// Set a long-duration timer (will not fire during test)
	mgr.mu.Lock()
	mgr.timers[42] = time.AfterFunc(10*time.Minute, func() {})
	mgr.mu.Unlock()

	// Pause should stop and remove timer
	mgr.Pause(42)

	mgr.mu.Lock()
	_, exists := mgr.timers[42]
	mgr.mu.Unlock()

	if exists {
		t.Error("expected timer 42 to be removed after Pause")
	}

	// Remove on non-existent ID should not panic
	mgr.Remove(99)
}

// TestManagerActiveDedup verifies that scheduleNext skips submission
// when the active map already has the monitorID.
func TestManagerActiveDedup(t *testing.T) {
	submitted := 0

	mgr := &Manager{
		timers: make(map[int64]*time.Timer),
		pool: &mockPool{submitFn: func() bool {
			submitted++
			return true
		}},
	}

	// Pre-populate active map to simulate a running poll
	mgr.active.Store(int64(1), struct{}{})

	// scheduleNext with a tiny interval: the timer fires almost immediately
	mgr.scheduleNext(context.Background(), 1, 1*time.Millisecond)

	// Give the timer time to fire
	time.Sleep(50 * time.Millisecond)

	// The job should NOT have been submitted because active map had monitorID 1
	if submitted > 0 {
		t.Errorf("expected 0 submissions (dedup), got %d", submitted)
	}

	// Cleanup
	mgr.Pause(1)
}

// --- mock pool ---

type mockPool struct {
	submitFn func() bool
}

func (p *mockPool) Submit(job interface{ Name_() string }) bool {
	return p.submitFn()
}
```

Wait — `worker.WorkerPool` is a concrete struct, not an interface. The Manager's `pool` field has type `*worker.WorkerPool`. The mock approach won't work without an interface. Simplify: just test the pure functions (calcPollInterval, tfDuration, cancelTimer) and skip the mock pool test.

Replace the above with this simpler test file:

```go
package monitor

import (
	"testing"
	"time"
)

func TestCalcPollIntervalAllBuckets(t *testing.T) {
	cases := []struct {
		tf   string
		want time.Duration
	}{
		{"1m", 30 * time.Second},
		{"5m", 30 * time.Second},
		{"15m", 90 * time.Second},
		{"1h", 6 * time.Minute},
		{"4h", 24 * time.Minute},
		{"1d", 1 * time.Hour},
		{"bogus", 60 * time.Second},
	}
	for _, tc := range cases {
		got := calcPollInterval(tc.tf)
		if got != tc.want {
			t.Errorf("calcPollInterval(%q) = %v, want %v", tc.tf, got, tc.want)
		}
	}
}

func TestTfDurationAllBuckets(t *testing.T) {
	cases := []struct {
		tf   string
		want time.Duration
	}{
		{"1m", time.Minute},
		{"5m", 5 * time.Minute},
		{"15m", 15 * time.Minute},
		{"30m", 30 * time.Minute},
		{"1h", time.Hour},
		{"4h", 4 * time.Hour},
		{"1d", 24 * time.Hour},
		{"1w", 7 * 24 * time.Hour},
		{"unknown", time.Hour},
	}
	for _, tc := range cases {
		got := tfDuration(tc.tf)
		if got != tc.want {
			t.Errorf("tfDuration(%q) = %v, want %v", tc.tf, got, tc.want)
		}
	}
}

func TestManagerCancelTimerRemovesEntry(t *testing.T) {
	mgr := &Manager{
		timers: make(map[int64]*time.Timer),
	}

	mgr.mu.Lock()
	mgr.timers[42] = time.AfterFunc(10*time.Minute, func() {})
	mgr.mu.Unlock()

	mgr.Pause(42)

	mgr.mu.Lock()
	_, exists := mgr.timers[42]
	mgr.mu.Unlock()

	if exists {
		t.Error("expected timer 42 to be removed after Pause")
	}
}

func TestManagerRemoveNonExistentIDDoesNotPanic(t *testing.T) {
	mgr := &Manager{
		timers: make(map[int64]*time.Timer),
	}
	// Should not panic
	mgr.Remove(999)
}
```

---

**Step 2: Run tests**

```bash
docker compose exec backend go test ./internal/monitor/... -v
```

Expected: all 4 tests PASS.

---

**Step 3: Commit**

```bash
git add backend/internal/monitor/manager_test.go
git commit -m "test(phase8): add monitor interval + manager unit tests"
```

---

### Task B6: Create internal/api/monitor_handler.go

**Read first:**
- `backend/internal/api/alerts.go` (full — for handler struct pattern)
- `backend/internal/api/notifications.go` (full — for pagination pattern)
- `backend/internal/registry/registry.go` (lines 138–145 for `Exists`)

**Files to create:**
- `backend/internal/api/monitor_handler.go`

---

**Step 1: Create the file**

Create `backend/internal/api/monitor_handler.go`:

```go
package api

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/models"
	"github.com/trader-claude/backend/internal/monitor"
	"github.com/trader-claude/backend/internal/registry"
)

type monitorHandler struct {
	db  *gorm.DB
	mgr *monitor.Manager
}

func newMonitorHandler(db *gorm.DB, mgr *monitor.Manager) *monitorHandler {
	return &monitorHandler{db: db, mgr: mgr}
}

// POST /api/v1/monitors
func (h *monitorHandler) createMonitor(c *fiber.Ctx) error {
	var body struct {
		Name         string                 `json:"name"`
		AdapterID    string                 `json:"adapter_id"`
		Symbol       string                 `json:"symbol"`
		Market       string                 `json:"market"`
		Timeframe    string                 `json:"timeframe"`
		StrategyName string                 `json:"strategy_name"`
		Params       map[string]interface{} `json:"params"`
		NotifyInApp  *bool                  `json:"notify_in_app"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if body.Symbol == "" || body.AdapterID == "" || body.StrategyName == "" || body.Timeframe == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "symbol, adapter_id, strategy_name and timeframe are required"})
	}
	if !registry.Strategies().Exists(body.StrategyName) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "unknown strategy: " + body.StrategyName})
	}

	notifyInApp := true
	if body.NotifyInApp != nil {
		notifyInApp = *body.NotifyInApp
	}

	name := body.Name
	if name == "" {
		name = body.StrategyName + " " + body.Symbol + " " + body.Timeframe
	}

	mon := models.Monitor{
		Name:         name,
		AdapterID:    body.AdapterID,
		Symbol:       body.Symbol,
		Market:       body.Market,
		Timeframe:    body.Timeframe,
		StrategyName: body.StrategyName,
		Params:       models.JSON(body.Params),
		Status:       models.MonitorStatusActive,
		NotifyInApp:  notifyInApp,
	}

	if err := h.db.Create(&mon).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	h.mgr.Add(c.Context(), mon.ID, mon.Timeframe)

	return c.Status(fiber.StatusCreated).JSON(mon)
}

// GET /api/v1/monitors
func (h *monitorHandler) listMonitors(c *fiber.Ctx) error {
	var monitors []models.Monitor
	if err := h.db.Order("created_at DESC").Find(&monitors).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": monitors})
}

// GET /api/v1/monitors/:id
func (h *monitorHandler) getMonitor(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	var mon models.Monitor
	if err := h.db.First(&mon, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
	}
	return c.JSON(mon)
}

// PUT /api/v1/monitors/:id
func (h *monitorHandler) updateMonitor(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	var mon models.Monitor
	if err := h.db.First(&mon, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
	}
	var body struct {
		Name        string `json:"name"`
		NotifyInApp *bool  `json:"notify_in_app"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	updates := map[string]interface{}{}
	if body.Name != "" {
		updates["name"] = body.Name
	}
	if body.NotifyInApp != nil {
		updates["notify_in_app"] = *body.NotifyInApp
	}
	if len(updates) > 0 {
		if err := h.db.Model(&mon).Updates(updates).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}
	return c.JSON(mon)
}

// DELETE /api/v1/monitors/:id
func (h *monitorHandler) deleteMonitor(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	h.mgr.Remove(int64(id))
	if err := h.db.Delete(&models.Monitor{}, id).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// PATCH /api/v1/monitors/:id/toggle
// Flips status: active → paused, paused → active.
func (h *monitorHandler) toggleMonitor(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	var mon models.Monitor
	if err := h.db.First(&mon, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
	}

	switch mon.Status {
	case models.MonitorStatusActive:
		if err := h.db.Model(&mon).Update("status", models.MonitorStatusPaused).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		h.mgr.Pause(int64(id))
	case models.MonitorStatusPaused:
		if err := h.db.Model(&mon).Update("status", models.MonitorStatusActive).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		h.mgr.Resume(c.Context(), int64(id), mon.Timeframe)
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "monitor is stopped"})
	}

	h.db.First(&mon, id) // reload
	return c.JSON(mon)
}

// GET /api/v1/monitors/:id/signals?limit=20&offset=0
func (h *monitorHandler) listSignals(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	limit := c.QueryInt("limit", 20)
	if limit > 100 {
		limit = 100
	}
	offset := c.QueryInt("offset", 0)

	var total int64
	h.db.Model(&models.MonitorSignal{}).Where("monitor_id = ?", id).Count(&total)

	var signals []models.MonitorSignal
	if err := h.db.Where("monitor_id = ?", id).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&signals).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"data":   signals,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}
```

---

**Step 2: Verify compile**

```bash
docker compose exec backend go build ./internal/api/...
```

Expected: no errors.

---

**Step 3: Commit**

```bash
git add backend/internal/api/monitor_handler.go
git commit -m "feat(phase8): add monitor CRUD + signals API handler"
```

---

### Task B7: Create internal/api/monitor_ws.go

**Read first:**
- `backend/internal/api/notifications.go` (lines 83–122 for Redis pubsub WS pattern)
- `backend/internal/monitor/poller.go` (lines 1–28 for `signalEvent` type and channel name)

**Files to create:**
- `backend/internal/api/monitor_ws.go`

---

**Step 1: Create the file**

Create `backend/internal/api/monitor_ws.go`:

```go
package api

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"github.com/gofiber/contrib/websocket"
	"github.com/redis/go-redis/v9"
)

const monitorSignalChannel = "monitor:signals"

// monitorSignalMsg is the JSON structure sent to WS clients.
type monitorSignalMsg struct {
	ID        int64                  `json:"id"`
	MonitorID int64                  `json:"monitor_id"`
	Direction string                 `json:"direction"`
	Price     float64                `json:"price"`
	Strength  float64                `json:"strength"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt string                 `json:"created_at"`
}

// wsClientMsg is the control message sent by the browser client.
type wsClientMsg struct {
	Action     string  `json:"action"`     // "subscribe" | "unsubscribe"
	MonitorIDs []int64 `json:"monitor_ids"` // IDs to add/remove
}

// signalsWS handles WS /ws/monitors/signals
// Protocol:
//
//	Client → {"action":"subscribe",   "monitor_ids":[1,2]}
//	Client → {"action":"unsubscribe", "monitor_ids":[1]}
//	Server → monitorSignalMsg (only for subscribed IDs)
func (h *monitorHandler) signalsWS(conn *websocket.Conn) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Per-connection subscription set
	var mu sync.RWMutex
	subscribed := make(map[int64]bool)

	// Subscribe to Redis pubsub channel
	sub := h.db.Statement.DB.(*fakeRDB).Subscribe(ctx, monitorSignalChannel)
	// NOTE: h.db does not hold the redis client. We need to get it differently.
	// See fix below — this will be replaced in routes.go wiring.
	_ = sub
}
```

Wait — the `monitorHandler` struct only has `db` and `mgr`, not `rdb`. We need to add `rdb *redis.Client` to the struct. Let me rewrite:

**Replace the entire file content** with the correct implementation:

```go
package api

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/monitor"
)

// monitorHandler handles monitor CRUD and the signals WebSocket.
// (Overrides the struct defined in monitor_handler.go — we extend it here.)
// IMPORTANT: update monitor_handler.go to add rdb field.

// wsClientMsg is the control message sent by the browser.
type wsClientMsg struct {
	Action     string  `json:"action"`      // "subscribe" | "unsubscribe"
	MonitorIDs []int64 `json:"monitor_ids"` // IDs to add/remove
}

// signalsWS handles WS /ws/monitors/signals
// Protocol:
//
//	Client → Server: {"action":"subscribe",   "monitor_ids":[1,2,3]}
//	Client → Server: {"action":"unsubscribe", "monitor_ids":[1]}
//	Server → Client: monitorSignalMsg JSON (only for subscribed IDs)
func signalsWS(rdb *redis.Client) func(*websocket.Conn) {
	return func(conn *websocket.Conn) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Subscribe to the Redis broadcast channel
		sub := rdb.Subscribe(ctx, monitor.SignalChannel)
		defer sub.Close()

		// Per-connection subscription set
		var mu sync.RWMutex
		subscribed := make(map[int64]bool)

		redisCh := sub.Channel()

		// Goroutine: read client control messages
		clientMsgs := make(chan wsClientMsg, 16)
		go func() {
			defer close(clientMsgs)
			for {
				_, raw, err := conn.ReadMessage()
				if err != nil {
					return
				}
				var msg wsClientMsg
				if err := json.Unmarshal(raw, &msg); err != nil {
					continue
				}
				clientMsgs <- msg
			}
		}()

		for {
			select {
			case msg, ok := <-clientMsgs:
				if !ok {
					return
				}
				mu.Lock()
				switch msg.Action {
				case "subscribe":
					for _, id := range msg.MonitorIDs {
						subscribed[id] = true
					}
				case "unsubscribe":
					for _, id := range msg.MonitorIDs {
						delete(subscribed, id)
					}
				}
				mu.Unlock()

			case redisMsg, ok := <-redisCh:
				if !ok {
					return
				}
				// Parse the signal event
				var evt struct {
					ID        int64                  `json:"id"`
					MonitorID int64                  `json:"monitor_id"`
					Direction string                 `json:"direction"`
					Price     float64                `json:"price"`
					Strength  float64                `json:"strength"`
					Metadata  map[string]interface{} `json:"metadata,omitempty"`
					CreatedAt time.Time              `json:"created_at"`
				}
				if err := json.Unmarshal([]byte(redisMsg.Payload), &evt); err != nil {
					log.Printf("ws/monitors/signals: malformed event: %v", err)
					continue
				}
				// Forward only if the client has subscribed to this monitor
				mu.RLock()
				shouldSend := subscribed[evt.MonitorID]
				mu.RUnlock()
				if !shouldSend {
					continue
				}
				b, err := json.Marshal(evt)
				if err != nil {
					continue
				}
				if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
					return
				}

			case <-ctx.Done():
				return
			}
		}
	}
}
```

Also update `monitor_handler.go` to remove the placeholder struct note — the handler is complete as written. The `signalsWS` function above is standalone (not a method on `monitorHandler`) and takes `rdb` directly when wired in routes.go.

---

**Step 2: Export `SignalChannel` from poller.go**

In `backend/internal/monitor/poller.go`, change:
```go
const (
	signalChannel = "monitor:signals"
	warmupCandles = 200
)
```
to:
```go
const (
	// SignalChannel is the Redis pubsub channel for monitor signal events.
	// Exported so the WS handler can subscribe to the same channel.
	SignalChannel = "monitor:signals"
	warmupCandles = 200
)
```

Then in `poller.go`, update the `rdb.Publish` call:
```go
rdb.Publish(ctx, SignalChannel, string(b))
```

---

**Step 3: Verify compile**

```bash
docker compose exec backend go build ./internal/monitor/... ./internal/api/...
```

Expected: no errors.

---

**Step 4: Commit**

```bash
git add backend/internal/api/monitor_ws.go backend/internal/monitor/poller.go
git commit -m "feat(phase8): add monitor signals WebSocket handler"
```

---

### Task B8: Wire monitor routes in routes.go and main.go

**Read first:**
- `backend/internal/api/routes.go` (full — 104 lines)
- `backend/cmd/server/main.go` (full — 211 lines)

**Files to modify:**
- `backend/internal/api/routes.go`
- `backend/cmd/server/main.go`

---

**Step 1: Update `RegisterRoutes` signature and wire monitor routes**

In `routes.go`, find the function signature:
```go
func RegisterRoutes(app *fiber.App, db *gorm.DB, rdb *redis.Client, hub *ws.Hub, version string, pool *worker.WorkerPool, ds *adapter.DataService, mgr *replay.Manager) {
```
Replace with:
```go
func RegisterRoutes(app *fiber.App, db *gorm.DB, rdb *redis.Client, hub *ws.Hub, version string, pool *worker.WorkerPool, ds *adapter.DataService, mgr *replay.Manager, monMgr *monitor.Manager) {
```

Add `monitor` import — find the import block and add:
```go
	"github.com/trader-claude/backend/internal/monitor"
```

At the end of the `v1` group (after the `// --- Notifications ---` block, before the WebSocket middleware), add:

```go
	// --- Monitors ---
	mnh := newMonitorHandler(db, monMgr)
	v1.Post("/monitors", mnh.createMonitor)
	v1.Get("/monitors", mnh.listMonitors)
	v1.Get("/monitors/:id", mnh.getMonitor)
	v1.Put("/monitors/:id", mnh.updateMonitor)
	v1.Delete("/monitors/:id", mnh.deleteMonitor)
	v1.Patch("/monitors/:id/toggle", mnh.toggleMonitor)
	v1.Get("/monitors/:id/signals", mnh.listSignals)
```

At the end of the WebSocket routes (after the `// Notifications WebSocket` line), add:

```go
	// Monitor signals WebSocket (multiplexed)
	app.Get("/ws/monitors/signals", websocket.New(signalsWS(rdb)))
```

---

**Step 2: Update main.go to create and start the monitor manager**

In `main.go`, after the `alertEval.Start(context.Background())` line, add:

```go
	// Start monitor manager (polls active monitors on schedule)
	monitorMgr := monitor.NewManager(db, rdb, ds, pool)
	monitorMgr.Start(context.Background())
```

Add the import at the top:
```go
	monitorpkg "github.com/trader-claude/backend/internal/monitor"
```

Wait — the import alias must match the variable name. Use this import:
```go
	monitormgr "github.com/trader-claude/backend/internal/monitor"
```
And create it as:
```go
	monitorMgr := monitormgr.NewManager(db, rdb, ds, pool)
	monitorMgr.Start(context.Background())
```

Update the `api.RegisterRoutes` call at line 143:
```go
	api.RegisterRoutes(app, db, rdb, hub, cfg.App.Version, pool, ds, replayMgr, monitorMgr)
```

---

**Step 3: Format and compile**

```bash
make backend-fmt
docker compose exec backend go build ./...
```

Expected: no errors.

---

**Step 4: Run all backend tests**

```bash
make backend-test
```

Expected: all tests PASS.

---

**Step 5: Commit**

```bash
git add backend/internal/api/routes.go backend/cmd/server/main.go
git commit -m "feat(phase8): wire monitor routes and start manager in main.go"
```

---

### Task B9: Create monitor API integration tests

**Read first:**
- `backend/internal/api/backtest_test.go` (for test setup pattern — first 60 lines)
- `backend/internal/api/monitor_handler.go` (full)

**Files to create:**
- `backend/internal/api/monitor_handler_test.go`

---

**Step 1: Read backtest_test.go to understand test setup**

```bash
docker compose exec backend head -60 /app/internal/api/backtest_test.go
```

OR read `backend/internal/api/backtest_test.go` lines 1-60 with the Read tool before writing this task.

---

**Step 2: Create the test file**

Create `backend/internal/api/monitor_handler_test.go`:

```go
package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/models"
	"github.com/trader-claude/backend/internal/monitor"
	"github.com/trader-claude/backend/internal/registry"
)

// setupMonitorTestApp creates a Fiber app with a SQLite in-memory DB and a
// no-op Manager (pool is nil so timers never submit jobs in tests).
func setupMonitorTestApp(t *testing.T) (*fiber.App, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.Monitor{}, &models.MonitorSignal{}); err != nil {
		t.Fatalf("auto-migrate: %v", err)
	}

	// Register a dummy strategy so validation passes
	_ = registry.Strategies().Register("test_strat", func() registry.Strategy { return nil })

	mgr := monitor.NewManager(db, nil, nil, nil) // nil pool — timers will fire but Submit returns false

	app := fiber.New()
	mnh := newMonitorHandler(db, mgr)
	v1 := app.Group("/api/v1")
	v1.Post("/monitors", mnh.createMonitor)
	v1.Get("/monitors", mnh.listMonitors)
	v1.Get("/monitors/:id", mnh.getMonitor)
	v1.Delete("/monitors/:id", mnh.deleteMonitor)
	v1.Patch("/monitors/:id/toggle", mnh.toggleMonitor)
	v1.Get("/monitors/:id/signals", mnh.listSignals)

	return app, db
}

func TestCreateMonitor(t *testing.T) {
	app, _ := setupMonitorTestApp(t)

	body := map[string]interface{}{
		"adapter_id":    "binance",
		"symbol":        "BTCUSDT",
		"market":        "crypto",
		"timeframe":     "1h",
		"strategy_name": "test_strat",
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/monitors", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var mon models.Monitor
	json.NewDecoder(resp.Body).Decode(&mon)
	if mon.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if mon.Status != models.MonitorStatusActive {
		t.Errorf("expected status active, got %s", mon.Status)
	}
	// Name auto-generated from strategy + symbol + timeframe
	if mon.Name == "" {
		t.Error("expected non-empty name")
	}
}

func TestCreateMonitorUnknownStrategy(t *testing.T) {
	app, _ := setupMonitorTestApp(t)

	body := map[string]interface{}{
		"adapter_id":    "binance",
		"symbol":        "BTCUSDT",
		"market":        "crypto",
		"timeframe":     "1h",
		"strategy_name": "does_not_exist",
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/monitors", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestListMonitors(t *testing.T) {
	app, db := setupMonitorTestApp(t)

	db.Create(&models.Monitor{Name: "M1", AdapterID: "binance", Symbol: "BTCUSDT", Market: "crypto", Timeframe: "1h", StrategyName: "ema_crossover", Status: models.MonitorStatusActive})
	db.Create(&models.Monitor{Name: "M2", AdapterID: "binance", Symbol: "ETHUSDT", Market: "crypto", Timeframe: "1h", StrategyName: "rsi", Status: models.MonitorStatusPaused})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/monitors", nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result struct{ Data []models.Monitor }
	json.NewDecoder(resp.Body).Decode(&result)
	if len(result.Data) != 2 {
		t.Errorf("expected 2 monitors, got %d", len(result.Data))
	}
}

func TestToggleMonitor(t *testing.T) {
	app, db := setupMonitorTestApp(t)

	mon := models.Monitor{Name: "M1", AdapterID: "binance", Symbol: "BTCUSDT", Market: "crypto", Timeframe: "1h", StrategyName: "ema_crossover", Status: models.MonitorStatusActive}
	db.Create(&mon)

	// Toggle: active → paused
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/monitors/1/toggle", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var toggled models.Monitor
	json.NewDecoder(resp.Body).Decode(&toggled)
	if toggled.Status != models.MonitorStatusPaused {
		t.Errorf("expected paused, got %s", toggled.Status)
	}

	// Toggle again: paused → active
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/monitors/1/toggle", nil)
	resp, _ = app.Test(req)
	json.NewDecoder(resp.Body).Decode(&toggled)
	if toggled.Status != models.MonitorStatusActive {
		t.Errorf("expected active, got %s", toggled.Status)
	}
}

func TestDeleteMonitor(t *testing.T) {
	app, db := setupMonitorTestApp(t)

	mon := models.Monitor{Name: "ToDelete", AdapterID: "binance", Symbol: "BTCUSDT", Market: "crypto", Timeframe: "1h", StrategyName: "ema_crossover", Status: models.MonitorStatusActive}
	db.Create(&mon)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/monitors/1", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	// Soft-deleted — direct DB query should find nothing
	var count int64
	db.Model(&models.Monitor{}).Where("id = ?", 1).Count(&count)
	if count != 0 {
		t.Error("expected monitor to be deleted")
	}
}

func TestListSignals(t *testing.T) {
	app, db := setupMonitorTestApp(t)

	mon := models.Monitor{Name: "M1", AdapterID: "binance", Symbol: "BTCUSDT", Market: "crypto", Timeframe: "1h", StrategyName: "ema_crossover", Status: models.MonitorStatusActive}
	db.Create(&mon)
	db.Create(&models.MonitorSignal{MonitorID: mon.ID, Direction: "long", Price: 50000.0, Strength: 0.8})
	db.Create(&models.MonitorSignal{MonitorID: mon.ID, Direction: "short", Price: 48000.0, Strength: 0.6})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/monitors/1/signals", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result struct {
		Data  []models.MonitorSignal `json:"data"`
		Total int64                  `json:"total"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Total != 2 {
		t.Errorf("expected total 2, got %d", result.Total)
	}
}
```

---

**Step 3: Run tests**

```bash
docker compose exec backend go test ./internal/api/... -v -run TestCreateMonitor
docker compose exec backend go test ./internal/api/... -v -run TestListMonitors
docker compose exec backend go test ./internal/api/... -v -run TestToggleMonitor
docker compose exec backend go test ./internal/api/... -v -run TestDeleteMonitor
docker compose exec backend go test ./internal/api/... -v -run TestListSignals
```

Expected: all 5 tests PASS.

If SQLite is not in `go.mod`, add it:
```bash
docker compose exec backend go get gorm.io/driver/sqlite
```
Then re-run tests.

---

**Step 4: Run full backend test suite**

```bash
make backend-test
```

Expected: all tests PASS.

---

**Step 5: Commit**

```bash
git add backend/internal/api/monitor_handler_test.go
git commit -m "test(phase8): add monitor handler integration tests"
```

---

## FRONTEND TASKS

---

### Task F1: Add Monitor + MonitorSignal types to types/index.ts

**Read first:**
- `frontend/src/types/index.ts` (last 50 lines — to see where to append)

**Files to modify:**
- `frontend/src/types/index.ts`

---

**Step 1: Append to types/index.ts**

At the very end of `frontend/src/types/index.ts`, append:

```ts
// ── Monitor types (Phase 8) ─────────────────────────────────────────────────

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

export interface MonitorCreateRequest {
  name?: string
  adapter_id: string
  symbol: string
  market: string
  timeframe: string
  strategy_name: string
  params?: Record<string, unknown>
  notify_in_app?: boolean
}

export interface MonitorSignalsResponse {
  data: MonitorSignal[]
  total: number
  limit: number
  offset: number
}
```

---

**Step 2: Verify TypeScript compiles**

```bash
make frontend-lint
```

Expected: no errors.

---

**Step 3: Commit**

```bash
git add frontend/src/types/index.ts
git commit -m "feat(phase8): add Monitor + MonitorSignal TypeScript types"
```

---

### Task F2: Add useMonitorStore to stores/index.ts

**Read first:**
- `frontend/src/stores/index.ts` (last 40 lines — the notificationStore, to see the pattern)

**Files to modify:**
- `frontend/src/stores/index.ts`

---

**Step 1: Add Monitor import at the top of stores/index.ts**

Find the import block at the top of `stores/index.ts`:
```ts
import type {
  Alert,
  Backtest,
  Notification,
  ...
} from '@/types'
```

Add `Monitor` and `MonitorSignal` to the import list:
```ts
import type {
  Alert,
  Backtest,
  Monitor,
  MonitorSignal,
  Notification,
  ...
} from '@/types'
```

---

**Step 2: Append useMonitorStore to the end of stores/index.ts**

After the last closing `})` of `useNotificationStore`, append:

```ts
// ── Monitor store ──────────────────────────────────────────────────────────

interface MonitorStore {
  monitors: Monitor[]
  setMonitors: (m: Monitor[]) => void
  addMonitor: (m: Monitor) => void
  updateMonitor: (m: Monitor) => void
  removeMonitor: (id: number) => void
  pendingSignals: MonitorSignal[]
  addSignal: (s: MonitorSignal) => void
  clearSignal: (id: number) => void
}

export const useMonitorStore = create<MonitorStore>()((set) => ({
  monitors: [],
  setMonitors: (monitors) => set({ monitors }),
  addMonitor: (m) => set((s) => ({ monitors: [m, ...s.monitors] })),
  updateMonitor: (m) =>
    set((s) => ({ monitors: s.monitors.map((x) => (x.id === m.id ? m : x)) })),
  removeMonitor: (id) =>
    set((s) => ({ monitors: s.monitors.filter((x) => x.id !== id) })),
  pendingSignals: [],
  addSignal: (sig) => set((s) => ({ pendingSignals: [...s.pendingSignals, sig] })),
  clearSignal: (id) =>
    set((s) => ({ pendingSignals: s.pendingSignals.filter((x) => x.id !== id) })),
}))
```

---

**Step 3: Verify TypeScript compiles**

```bash
make frontend-lint
```

Expected: no errors.

---

**Step 4: Commit**

```bash
git add frontend/src/stores/index.ts
git commit -m "feat(phase8): add useMonitorStore to Zustand stores"
```

---

### Task F3: Create frontend/src/api/monitors.ts

**Read first:**
- `frontend/src/api/alerts.ts` (full — for the pattern)
- `frontend/src/api/client.ts` (lines 1–5 — for import name)

**Files to create:**
- `frontend/src/api/monitors.ts`

---

**Step 1: Create the file**

Create `frontend/src/api/monitors.ts`:

```ts
import apiClient from './client'
import type { Monitor, MonitorCreateRequest, MonitorSignalsResponse } from '@/types'

export async function fetchMonitors(): Promise<{ data: Monitor[] }> {
  const { data } = await apiClient.get<{ data: Monitor[] }>('/api/v1/monitors')
  return data
}

export async function fetchMonitor(id: number): Promise<Monitor> {
  const { data } = await apiClient.get<Monitor>(`/api/v1/monitors/${id}`)
  return data
}

export async function createMonitor(req: MonitorCreateRequest): Promise<Monitor> {
  const { data } = await apiClient.post<Monitor>('/api/v1/monitors', req)
  return data
}

export async function deleteMonitor(id: number): Promise<void> {
  await apiClient.delete(`/api/v1/monitors/${id}`)
}

export async function toggleMonitor(id: number): Promise<Monitor> {
  const { data } = await apiClient.patch<Monitor>(`/api/v1/monitors/${id}/toggle`)
  return data
}

export async function fetchMonitorSignals(
  id: number,
  params: { limit?: number; offset?: number } = {},
): Promise<MonitorSignalsResponse> {
  const query = new URLSearchParams()
  if (params.limit != null) query.set('limit', String(params.limit))
  if (params.offset != null) query.set('offset', String(params.offset))
  const { data } = await apiClient.get<MonitorSignalsResponse>(
    `/api/v1/monitors/${id}/signals?${query.toString()}`,
  )
  return data
}
```

---

**Step 2: Verify TypeScript compiles**

```bash
make frontend-lint
```

Expected: no errors.

---

**Step 3: Commit**

```bash
git add frontend/src/api/monitors.ts
git commit -m "feat(phase8): add monitor API client functions"
```

---

### Task F4: Create frontend/src/hooks/useMonitors.ts

**Read first:**
- `frontend/src/hooks/useAlerts.ts` (full — for the pattern)

**Files to create:**
- `frontend/src/hooks/useMonitors.ts`

---

**Step 1: Create the file**

Create `frontend/src/hooks/useMonitors.ts`:

```ts
import { useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  fetchMonitors,
  createMonitor,
  deleteMonitor,
  toggleMonitor,
  fetchMonitorSignals,
} from '@/api/monitors'
import { useMonitorStore } from '@/stores'
import type { MonitorCreateRequest } from '@/types'

export function useMonitors() {
  const setMonitors = useMonitorStore((s) => s.setMonitors)

  const query = useQuery({
    queryKey: ['monitors'],
    queryFn: fetchMonitors,
  })

  useEffect(() => {
    if (query.data?.data) {
      setMonitors(query.data.data)
    }
  }, [query.data, setMonitors])

  return query
}

export function useCreateMonitor() {
  const qc = useQueryClient()
  const addMonitor = useMonitorStore((s) => s.addMonitor)
  return useMutation({
    mutationFn: (req: MonitorCreateRequest) => createMonitor(req),
    onSuccess: (mon) => {
      addMonitor(mon)
      qc.invalidateQueries({ queryKey: ['monitors'] })
    },
  })
}

export function useDeleteMonitor() {
  const qc = useQueryClient()
  const removeMonitor = useMonitorStore((s) => s.removeMonitor)
  return useMutation({
    mutationFn: (id: number) => deleteMonitor(id),
    onSuccess: (_, id) => {
      removeMonitor(id)
      qc.invalidateQueries({ queryKey: ['monitors'] })
    },
  })
}

export function useToggleMonitor() {
  const qc = useQueryClient()
  const updateMonitor = useMonitorStore((s) => s.updateMonitor)
  return useMutation({
    mutationFn: (id: number) => toggleMonitor(id),
    onSuccess: (mon) => {
      updateMonitor(mon)
      qc.invalidateQueries({ queryKey: ['monitors'] })
    },
  })
}

export function useMonitorSignals(id: number, page = 1, pageSize = 20) {
  return useQuery({
    queryKey: ['monitor-signals', id, page, pageSize],
    queryFn: () => fetchMonitorSignals(id, { limit: pageSize, offset: (page - 1) * pageSize }),
    enabled: id > 0,
  })
}
```

---

**Step 2: Verify TypeScript compiles**

```bash
make frontend-lint
```

Expected: no errors.

---

**Step 3: Commit**

```bash
git add frontend/src/hooks/useMonitors.ts
git commit -m "feat(phase8): add useMonitors React Query hooks"
```

---

### Task F5: Create frontend/src/hooks/useMonitorSignalsWS.ts

**Read first:**
- `frontend/src/hooks/useNotifications.ts` (lines 52–80 — `useNotificationWS` pattern)

**Files to create:**
- `frontend/src/hooks/useMonitorSignalsWS.ts`

---

**Step 1: Create the file**

Create `frontend/src/hooks/useMonitorSignalsWS.ts`:

```ts
import { useEffect, useRef } from 'react'
import { useMonitorStore } from '@/stores'
import type { MonitorSignal } from '@/types'

// useMonitorSignalsWS connects to /ws/monitors/signals and subscribes to
// the given monitorIds. New signals are added to the Zustand pendingSignals
// queue for toast display.
export function useMonitorSignalsWS(monitorIds: number[]) {
  const addSignal = useMonitorStore((s) => s.addSignal)
  const wsRef = useRef<WebSocket | null>(null)

  useEffect(() => {
    if (monitorIds.length === 0) return

    const wsUrl = (import.meta.env.VITE_WS_URL ?? 'ws://localhost:8080') as string
    const ws = new WebSocket(`${wsUrl}/ws/monitors/signals`)
    wsRef.current = ws

    ws.onopen = () => {
      ws.send(JSON.stringify({ action: 'subscribe', monitor_ids: monitorIds }))
    }

    ws.onmessage = (e: MessageEvent) => {
      try {
        const sig = JSON.parse(e.data as string) as MonitorSignal
        addSignal(sig)
      } catch {
        // ignore malformed messages
      }
    }

    ws.onerror = () => {
      // suppress console errors — server may not be running locally
    }

    return () => {
      ws.close()
    }
  }, [JSON.stringify(monitorIds), addSignal]) // eslint-disable-line react-hooks/exhaustive-deps
}
```

---

**Step 2: Verify TypeScript compiles**

```bash
make frontend-lint
```

Expected: no errors.

---

**Step 3: Commit**

```bash
git add frontend/src/hooks/useMonitorSignalsWS.ts
git commit -m "feat(phase8): add useMonitorSignalsWS hook"
```

---

### Task F6: Create components/SignalToast.tsx

**Read first:** Nothing — this is a standalone new component.

**Files to create:**
- `frontend/src/components/SignalToast.tsx`

---

**Step 1: Create the file**

Create `frontend/src/components/SignalToast.tsx`:

```tsx
import { useEffect } from 'react'
import { X, TrendingUp, TrendingDown } from 'lucide-react'
import { useMonitorStore } from '@/stores'
import type { MonitorSignal } from '@/types'

const TOAST_DURATION_MS = 8000

interface SignalToastItemProps {
  signal: MonitorSignal
}

function SignalToastItem({ signal }: SignalToastItemProps) {
  const clearSignal = useMonitorStore((s) => s.clearSignal)

  useEffect(() => {
    const t = setTimeout(() => clearSignal(signal.id), TOAST_DURATION_MS)
    return () => clearTimeout(t)
  }, [signal.id, clearSignal])

  const isLong = signal.direction === 'long'
  const bg = isLong
    ? 'bg-green-900/90 border-green-600'
    : 'bg-red-900/90 border-red-600'
  const text = isLong ? 'text-green-100' : 'text-red-100'
  const Icon = isLong ? TrendingUp : TrendingDown
  const label = isLong ? 'LONG' : 'SHORT'

  return (
    <div
      className={`flex items-start gap-3 p-4 rounded-lg border shadow-lg w-80 animate-slide-in-right ${bg}`}
    >
      <Icon className={`mt-0.5 h-5 w-5 flex-shrink-0 ${text}`} />
      <div className="flex-1 min-w-0">
        <p className={`text-sm font-semibold ${text}`}>
          {label} Signal
        </p>
        <p className={`text-xs mt-0.5 ${text} opacity-80 truncate`}>
          ${signal.price.toLocaleString(undefined, { maximumFractionDigits: 2 })}
          {' · '}strength {(signal.strength * 100).toFixed(0)}%
        </p>
      </div>
      <button
        onClick={() => clearSignal(signal.id)}
        className={`flex-shrink-0 ${text} opacity-70 hover:opacity-100`}
      >
        <X className="h-4 w-4" />
      </button>
    </div>
  )
}

// SignalToast renders stacked toasts (max 3) in the bottom-right corner.
// Include this once in Layout.tsx.
export function SignalToast() {
  const pendingSignals = useMonitorStore((s) => s.pendingSignals)

  if (pendingSignals.length === 0) return null

  // Show at most 3 toasts
  const visible = pendingSignals.slice(-3)

  return (
    <div className="fixed bottom-6 right-6 z-50 flex flex-col gap-2 items-end">
      {visible.map((sig) => (
        <SignalToastItem key={sig.id} signal={sig} />
      ))}
    </div>
  )
}
```

---

**Step 2: Verify TypeScript compiles**

```bash
make frontend-lint
```

Expected: no errors.

---

**Step 3: Commit**

```bash
git add frontend/src/components/SignalToast.tsx
git commit -m "feat(phase8): add SignalToast component"
```

---

### Task F7: Wire SignalToast into Layout.tsx

**Read first:**
- `frontend/src/components/layout/Layout.tsx` (full — 20 lines)

**Files to modify:**
- `frontend/src/components/layout/Layout.tsx`

---

**Step 1: Update Layout.tsx**

Replace the entire content of `frontend/src/components/layout/Layout.tsx` with:

```tsx
import { Outlet } from 'react-router-dom'
import { Sidebar } from './Sidebar'
import { TopBar } from './TopBar'
import { useNotificationWS } from '@/hooks/useNotifications'
import { SignalToast } from '@/components/SignalToast'

export function Layout() {
  useNotificationWS() // connect to /ws/notifications on mount

  return (
    <div className="flex h-screen overflow-hidden bg-background">
      <Sidebar />
      <div className="flex flex-col flex-1 overflow-hidden">
        <TopBar />
        <main className="flex-1 overflow-y-auto p-6 animate-fade-in">
          <Outlet />
        </main>
      </div>
      <SignalToast />
    </div>
  )
}
```

---

**Step 2: Verify TypeScript compiles**

```bash
make frontend-lint
```

Expected: no errors.

---

**Step 3: Commit**

```bash
git add frontend/src/components/layout/Layout.tsx
git commit -m "feat(phase8): wire SignalToast into Layout"
```

---

### Task F8: Replace pages/Monitor.tsx with full implementation

**Read first:**
- `frontend/src/pages/Monitor.tsx` (full — stub)
- `frontend/src/hooks/useMonitors.ts` (full)
- `frontend/src/hooks/useMonitorSignalsWS.ts` (full)
- `frontend/src/api/useBacktest.ts` (for `useStrategies` hook if available, else use `useMarketData`)

Also check what hooks are available for strategies:
```bash
grep -r "useStrategies\|fetchStrategies\|strategies" frontend/src/api/ frontend/src/hooks/ --include="*.ts" -l
```

**Files to modify:**
- `frontend/src/pages/Monitor.tsx`

---

**Step 1: Check which hooks/API functions already exist for strategies**

Open `frontend/src/api/useBacktest.ts` or `frontend/src/hooks/useBacktest.ts` and look for strategy-related exports. They should expose `useStrategies()` which calls `GET /api/v1/strategies`.

If `useStrategies()` does not exist, add it to `frontend/src/hooks/useBacktest.ts`:
```ts
import { fetchStrategies } from '@/api/backtest'  // check this exists
export function useStrategies() {
  return useQuery({ queryKey: ['strategies'], queryFn: fetchStrategies })
}
```

---

**Step 2: Replace Monitor.tsx**

Replace the entire content of `frontend/src/pages/Monitor.tsx` with:

```tsx
import { useState } from 'react'
import { Plus, Play, Pause, Trash2, Zap, Clock, ChevronDown, ChevronUp } from 'lucide-react'
import { formatDistanceToNow } from 'date-fns'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import {
  useMonitors,
  useCreateMonitor,
  useDeleteMonitor,
  useToggleMonitor,
  useMonitorSignals,
} from '@/hooks/useMonitors'
import { useMonitorSignalsWS } from '@/hooks/useMonitorSignalsWS'
import { useMonitorStore } from '@/stores'
import type { Monitor, MonitorCreateRequest, StrategyInfo } from '@/types'

// ── Signal history sub-component ───────────────────────────────────────────

function SignalHistoryTable({ monitorId }: { monitorId: number }) {
  const [page, setPage] = useState(1)
  const { data } = useMonitorSignals(monitorId, page, 20)

  if (!data || data.data.length === 0) {
    return <p className="text-sm text-muted-foreground py-4 text-center">No signals yet.</p>
  }

  const totalPages = Math.ceil(data.total / 20)

  return (
    <div className="mt-3">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b text-left text-muted-foreground">
            <th className="pb-2 font-medium">Time</th>
            <th className="pb-2 font-medium">Direction</th>
            <th className="pb-2 font-medium">Price</th>
            <th className="pb-2 font-medium">Strength</th>
          </tr>
        </thead>
        <tbody>
          {data.data.map((sig) => (
            <tr key={sig.id} className="border-b last:border-0">
              <td className="py-2 text-muted-foreground">
                {formatDistanceToNow(new Date(sig.created_at), { addSuffix: true })}
              </td>
              <td className="py-2">
                <Badge
                  variant="outline"
                  className={
                    sig.direction === 'long'
                      ? 'text-green-500 border-green-500'
                      : 'text-red-500 border-red-500'
                  }
                >
                  {sig.direction.toUpperCase()}
                </Badge>
              </td>
              <td className="py-2 font-mono">
                ${sig.price.toLocaleString(undefined, { maximumFractionDigits: 2 })}
              </td>
              <td className="py-2">{(sig.strength * 100).toFixed(0)}%</td>
            </tr>
          ))}
        </tbody>
      </table>
      {totalPages > 1 && (
        <div className="flex justify-end gap-2 mt-3">
          <Button
            variant="outline"
            size="sm"
            disabled={page === 1}
            onClick={() => setPage((p) => p - 1)}
          >
            Prev
          </Button>
          <span className="text-sm text-muted-foreground self-center">
            {page} / {totalPages}
          </span>
          <Button
            variant="outline"
            size="sm"
            disabled={page === totalPages}
            onClick={() => setPage((p) => p + 1)}
          >
            Next
          </Button>
        </div>
      )}
    </div>
  )
}

// ── Monitor card ───────────────────────────────────────────────────────────

function MonitorCard({ monitor }: { monitor: Monitor }) {
  const [expanded, setExpanded] = useState(false)
  const toggleMon = useToggleMonitor()
  const deleteMon = useDeleteMonitor()

  const isActive = monitor.status === 'active'

  return (
    <div className="rounded-lg border bg-card p-4 flex flex-col gap-3">
      {/* Header row */}
      <div className="flex items-start justify-between gap-2">
        <div className="flex items-center gap-2 min-w-0">
          <span
            className={`h-2.5 w-2.5 rounded-full flex-shrink-0 ${
              isActive ? 'bg-green-500 animate-pulse' : 'bg-muted-foreground'
            }`}
          />
          <div className="min-w-0">
            <p className="font-medium truncate">{monitor.name}</p>
            <p className="text-xs text-muted-foreground">
              {monitor.symbol} · {monitor.timeframe} · {monitor.adapter_id}
            </p>
          </div>
        </div>
        <Badge variant="outline" className="flex-shrink-0 text-xs">
          {monitor.strategy_name.replace('_', ' ')}
        </Badge>
      </div>

      {/* Last signal row */}
      {monitor.last_signal_at ? (
        <div className="flex items-center gap-1.5 text-xs">
          <Zap className="h-3.5 w-3.5 text-yellow-500" />
          <span
            className={
              monitor.last_signal_dir === 'long' ? 'text-green-500' : 'text-red-500'
            }
          >
            {monitor.last_signal_dir?.toUpperCase()}
          </span>
          <span className="text-muted-foreground">
            @ ${monitor.last_signal_price?.toLocaleString(undefined, { maximumFractionDigits: 2 })}
          </span>
          <span className="text-muted-foreground">
            · {formatDistanceToNow(new Date(monitor.last_signal_at), { addSuffix: true })}
          </span>
        </div>
      ) : monitor.last_polled_at ? (
        <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
          <Clock className="h-3.5 w-3.5" />
          <span>No signals · polled {formatDistanceToNow(new Date(monitor.last_polled_at), { addSuffix: true })}</span>
        </div>
      ) : (
        <p className="text-xs text-muted-foreground">Waiting for first poll…</p>
      )}

      {/* Action row */}
      <div className="flex items-center justify-between">
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => toggleMon.mutate(monitor.id)}
            disabled={toggleMon.isPending}
          >
            {isActive ? (
              <>
                <Pause className="h-3.5 w-3.5 mr-1" /> Pause
              </>
            ) : (
              <>
                <Play className="h-3.5 w-3.5 mr-1" /> Resume
              </>
            )}
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              if (confirm('Delete this monitor?')) deleteMon.mutate(monitor.id)
            }}
            disabled={deleteMon.isPending}
            className="text-destructive hover:text-destructive"
          >
            <Trash2 className="h-3.5 w-3.5" />
          </Button>
        </div>
        <Button
          variant="ghost"
          size="sm"
          onClick={() => setExpanded((v) => !v)}
          className="text-muted-foreground"
        >
          {expanded ? (
            <ChevronUp className="h-4 w-4" />
          ) : (
            <ChevronDown className="h-4 w-4" />
          )}
          <span className="ml-1 text-xs">Signals</span>
        </Button>
      </div>

      {/* Expanded signal history */}
      {expanded && <SignalHistoryTable monitorId={monitor.id} />}
    </div>
  )
}

// ── Create monitor modal ───────────────────────────────────────────────────

const TIMEFRAMES = ['1m', '5m', '15m', '1h', '4h', '1d']

interface CreateModalProps {
  open: boolean
  onClose: () => void
  strategies: StrategyInfo[]
}

function CreateMonitorModal({ open, onClose, strategies }: CreateModalProps) {
  const createMon = useCreateMonitor()
  const [adapterID, setAdapterID] = useState('binance')
  const [symbol, setSymbol] = useState('')
  const [market, setMarket] = useState('crypto')
  const [timeframe, setTimeframe] = useState('1h')
  const [strategyName, setStrategyName] = useState('')
  const [notifyInApp, setNotifyInApp] = useState(true)
  const [name, setName] = useState('')
  const [error, setError] = useState('')

  function handleSubmit() {
    setError('')
    if (!symbol.trim() || !strategyName) {
      setError('Symbol and strategy are required.')
      return
    }
    const req: MonitorCreateRequest = {
      name: name.trim() || undefined,
      adapter_id: adapterID,
      symbol: symbol.trim().toUpperCase(),
      market,
      timeframe,
      strategy_name: strategyName,
      notify_in_app: notifyInApp,
    }
    createMon.mutate(req, {
      onSuccess: () => {
        onClose()
        setSymbol('')
        setStrategyName('')
        setName('')
      },
      onError: (e: Error) => setError(e.message),
    })
  }

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>Create Monitor</DialogTitle>
        </DialogHeader>

        <div className="space-y-4 py-2">
          {/* Adapter + Symbol */}
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1">
              <Label>Adapter</Label>
              <select
                value={adapterID}
                onChange={(e) => setAdapterID(e.target.value)}
                className="w-full h-9 rounded-md border border-input bg-background px-3 text-sm"
              >
                <option value="binance">Binance</option>
                <option value="yahoo">Yahoo Finance</option>
              </select>
            </div>
            <div className="space-y-1">
              <Label>Symbol</Label>
              <Input
                placeholder="e.g. BTCUSDT"
                value={symbol}
                onChange={(e) => setSymbol(e.target.value)}
              />
            </div>
          </div>

          {/* Timeframe */}
          <div className="space-y-1">
            <Label>Timeframe</Label>
            <div className="flex gap-2 flex-wrap">
              {TIMEFRAMES.map((tf) => (
                <Button
                  key={tf}
                  type="button"
                  variant={timeframe === tf ? 'default' : 'outline'}
                  size="sm"
                  onClick={() => setTimeframe(tf)}
                >
                  {tf}
                </Button>
              ))}
            </div>
          </div>

          {/* Strategy */}
          <div className="space-y-1">
            <Label>Strategy</Label>
            <div className="grid grid-cols-2 gap-2">
              {strategies.map((s) => (
                <button
                  key={s.id}
                  type="button"
                  onClick={() => setStrategyName(s.id)}
                  className={`text-left p-3 rounded-lg border text-sm transition-colors ${
                    strategyName === s.id
                      ? 'border-primary bg-primary/5'
                      : 'border-border hover:border-primary/50'
                  }`}
                >
                  <p className="font-medium capitalize">{s.name.replace('_', ' ')}</p>
                  <p className="text-xs text-muted-foreground line-clamp-1">{s.description}</p>
                </button>
              ))}
            </div>
          </div>

          {/* Name (optional) */}
          <div className="space-y-1">
            <Label>
              Name <span className="text-muted-foreground text-xs">(optional, auto-generated)</span>
            </Label>
            <Input
              placeholder={strategyName ? `${strategyName} ${symbol || 'SYMBOL'} ${timeframe}` : 'Auto-generated'}
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </div>

          {/* Notify in-app */}
          <div className="flex items-center justify-between">
            <Label>In-app notifications</Label>
            <Switch checked={notifyInApp} onCheckedChange={setNotifyInApp} />
          </div>

          {error && <p className="text-sm text-destructive">{error}</p>}

          <div className="flex justify-end gap-2 pt-2">
            <Button variant="outline" onClick={onClose}>
              Cancel
            </Button>
            <Button onClick={handleSubmit} disabled={createMon.isPending}>
              {createMon.isPending ? 'Creating…' : 'Create Monitor'}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}

// ── Monitor page ───────────────────────────────────────────────────────────

export function Monitor() {
  const { data, isLoading } = useMonitors()
  const monitors = useMonitorStore((s) => s.monitors)
  const [showCreate, setShowCreate] = useState(false)

  // Load strategies for the create modal
  // useStrategies returns { data: StrategyInfo[] }
  // Import from useBacktest or wherever it is defined in your codebase.
  // If useStrategies is not exported, define a minimal inline fetch:
  const [strategies, setStrategies] = useState<StrategyInfo[]>([])
  useState(() => {
    fetch('/api/v1/strategies')
      .then((r) => r.json())
      .then((d: { data?: StrategyInfo[] }) => setStrategies(d.data ?? []))
      .catch(() => {})
  })

  // Connect to monitor signals WS
  const activeIds = monitors.filter((m) => m.status === 'active').map((m) => m.id)
  useMonitorSignalsWS(activeIds)

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold">Live Monitors</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Strategy-based real-time market watchers
          </p>
        </div>
        <Button onClick={() => setShowCreate(true)}>
          <Plus className="h-4 w-4 mr-2" />
          Create Monitor
        </Button>
      </div>

      {isLoading && (
        <p className="text-muted-foreground">Loading monitors…</p>
      )}

      {!isLoading && monitors.length === 0 && (
        <div className="flex flex-col items-center justify-center py-24 text-center">
          <Zap className="h-12 w-12 text-muted-foreground mb-4" />
          <h2 className="text-lg font-semibold">No monitors yet</h2>
          <p className="text-sm text-muted-foreground mt-1 mb-4">
            Create a monitor to watch a strategy on a live market feed.
          </p>
          <Button onClick={() => setShowCreate(true)}>
            <Plus className="h-4 w-4 mr-2" />
            Create your first monitor
          </Button>
        </div>
      )}

      {monitors.length > 0 && (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {monitors.map((mon) => (
            <MonitorCard key={mon.id} monitor={mon} />
          ))}
        </div>
      )}

      <CreateMonitorModal
        open={showCreate}
        onClose={() => setShowCreate(false)}
        strategies={strategies}
      />
    </div>
  )
}
```

---

**Step 3: Add `date-fns` if not already installed**

Check `package.json`:
```bash
grep date-fns frontend/package.json
```

If missing:
```bash
docker compose exec frontend bun add date-fns
```

---

**Step 4: Verify TypeScript compiles and lint passes**

```bash
make frontend-lint
```

Expected: no type errors.

---

**Step 5: Format**

```bash
make frontend-fmt
```

---

**Step 6: Run frontend tests**

```bash
make frontend-test
```

Expected: existing tests still pass (no regressions).

---

**Step 7: Commit**

```bash
git add frontend/src/pages/Monitor.tsx
git commit -m "feat(phase8): implement Monitor page with cards, signals table, and create modal"
```

---

## Final Verification

**Step 1: Full backend test suite**

```bash
make backend-test
```

Expected: all tests PASS.

**Step 2: Full frontend test suite**

```bash
make frontend-test
```

Expected: all tests PASS.

**Step 3: Format checks**

```bash
make backend-fmt
make frontend-fmt
```

**Step 4: Smoke test (services running)**

```bash
make up
make health
# Create a monitor
curl -s -X POST http://localhost:8080/api/v1/monitors \
  -H 'Content-Type: application/json' \
  -d '{"adapter_id":"binance","symbol":"BTCUSDT","market":"crypto","timeframe":"1h","strategy_name":"ema_crossover"}' | jq .
# List monitors
curl -s http://localhost:8080/api/v1/monitors | jq .
# Check DB — monitors table should exist
make db-shell
# mysql> SHOW TABLES;
# mysql> DESCRIBE monitors;
```

**Step 5: Update phases.md**

In `.claude/docs/phases.md`, mark Phase 8 complete:

```
## Phase 8 — Live Market Monitor & Signal Alerts ✅ COMPLETE
```

---

## Summary

| Task | Files | Description |
|---|---|---|
| B1 | models.go, main.go | Monitor + MonitorSignal models + autoMigrate |
| B2 | monitor/interval.go | Poll interval + tf duration helpers |
| B3 | monitor/manager.go | Manager: timer scheduling + lifecycle |
| B4 | monitor/poller.go | Warm-start poll execution + signal emit |
| B5 | monitor/manager_test.go | Unit tests for pure functions |
| B6 | api/monitor_handler.go | CRUD + signals pagination handler |
| B7 | api/monitor_ws.go | Multiplexed signals WebSocket |
| B8 | api/routes.go, main.go | Wiring + startup |
| B9 | api/monitor_handler_test.go | Integration tests |
| F1 | types/index.ts | Monitor + MonitorSignal TypeScript types |
| F2 | stores/index.ts | useMonitorStore |
| F3 | api/monitors.ts | API client functions |
| F4 | hooks/useMonitors.ts | React Query hooks |
| F5 | hooks/useMonitorSignalsWS.ts | WS hook |
| F6 | components/SignalToast.tsx | Toast component |
| F7 | layout/Layout.tsx | Wire toast into layout |
| F8 | pages/Monitor.tsx | Full Monitor page |
