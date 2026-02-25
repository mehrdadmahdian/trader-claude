# Phase 4 — Slow-Motion Replay Engine: Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a slow-motion replay overlay to the Backtest page so users can re-watch any completed backtest candle-by-candle, with play/pause/seek/speed controls and a bookmark system for saving annotated moments to MySQL.

**Architecture:** Active replay sessions live in an in-memory `sync.Map` (ephemeral, tied to the WS connection). Bookmarks persist to MySQL. The frontend mounts a full-screen overlay on top of the Backtest page without navigating away. The backend streams candle/trade/equity messages through a bidirectional WebSocket; control messages flow the other direction.

**Tech Stack:** Go 1.24 · Fiber v2 · `gofiber/contrib/websocket` · GORM · React 18 · TypeScript · lightweight-charts · Recharts · Zustand · lucide-react

**Reference design:** `docs/plans/2026-02-25-phase4-replay-engine-design.md`

---

## Task 1: Extend Models — AdapterID + ReplayBookmark

**Why AdapterID:** the `Backtest` model doesn't record which adapter was used. The replay needs to re-fetch candles, so we store it at run-time.

**Files:**
- Modify: `backend/internal/models/models.go`
- Modify: `backend/cmd/server/main.go` (AutoMigrate list)
- Modify: `backend/internal/api/backtest.go` (store AdapterID on run)

---

**Step 1: Add `AdapterID` to the `Backtest` struct**

In `backend/internal/models/models.go`, add one field to `Backtest` after the `Name` field:

```go
// inside Backtest struct, after Name:
AdapterID    string         `gorm:"type:varchar(50);not null;default:''" json:"adapter_id"`
```

---

**Step 2: Add `ReplayBookmark` model to the same file**

After the `WatchList` struct at the bottom of `models.go`:

```go
// --- ReplayBookmark ---

// ReplayBookmark stores an annotated moment in a replay session for future research.
type ReplayBookmark struct {
	ID            int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID        int64     `gorm:"index;default:1" json:"user_id"`
	BacktestRunID int64     `gorm:"index;not null" json:"backtest_run_id"`
	CandleIndex   int       `gorm:"not null" json:"candle_index"`
	Label         string    `gorm:"type:varchar(255)" json:"label"`
	Note          string    `gorm:"type:text" json:"note"`
	ChartSnapshot string    `gorm:"type:longtext" json:"chart_snapshot"` // base64 PNG
	CreatedAt     time.Time `json:"created_at"`
}

func (ReplayBookmark) TableName() string { return "replay_bookmarks" }
```

---

**Step 3: Add `ReplayBookmark` to AutoMigrate in `main.go`**

Find the `db.AutoMigrate(...)` call in `backend/cmd/server/main.go`. Add `&models.ReplayBookmark{}` to the list.

---

**Step 4: Store `AdapterID` in the `runBacktest` handler**

In `backend/internal/api/backtest.go`, in the `runBacktest` function, update the `bt` struct literal:

```go
bt := models.Backtest{
    Name:         req.Name,
    AdapterID:    req.Adapter,   // ← add this line
    StrategyName: req.Strategy,
    // ... rest unchanged
}
```

---

**Step 5: Verify compile**

```bash
cd backend && go build ./...
```

Expected: no errors (GORM AutoMigrate will add the new column on next startup).

---

**Step 6: Commit**

```bash
git add backend/internal/models/models.go backend/cmd/server/main.go backend/internal/api/backtest.go
git commit -m "feat(phase4): add AdapterID to Backtest model and ReplayBookmark model"
```

---

## Task 2: ReplaySession — Pure Logic + Unit Tests

The session holds all state. Pure-logic methods are separated from the WS goroutine so they are testable without a network connection.

**Files:**
- Create: `backend/internal/replay/session.go`
- Create: `backend/internal/replay/session_test.go`

---

**Step 1: Write the failing tests first**

Create `backend/internal/replay/session_test.go`:

```go
package replay

import (
	"testing"
	"time"

	"github.com/trader-claude/backend/internal/backtest"
	"github.com/trader-claude/backend/internal/models"
	"github.com/trader-claude/backend/internal/registry"
)

// ---- helpers ----

func makeCandles(n int) []registry.Candle {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := make([]registry.Candle, n)
	for i := range candles {
		candles[i] = registry.Candle{
			Symbol:    "BTC/USDT",
			Market:    "crypto",
			Timeframe: "1h",
			Timestamp: base.Add(time.Duration(i) * time.Hour),
			Open:      float64(100 + i),
			High:      float64(110 + i),
			Low:       float64(90 + i),
			Close:     float64(105 + i),
			Volume:    1000,
		}
	}
	return candles
}

func makeEquity(candles []registry.Candle) []backtest.EquityPoint {
	pts := make([]backtest.EquityPoint, len(candles))
	for i, c := range candles {
		pts[i] = backtest.EquityPoint{Timestamp: c.Timestamp, Value: 10000 + float64(i*100)}
	}
	return pts
}

// ---- tests ----

func TestNewSession_InitialState(t *testing.T) {
	candles := makeCandles(10)
	s := NewSession("id-1", 42, candles, makeEquity(candles), nil)

	if s.CurrentIdx != 0 {
		t.Errorf("expected CurrentIdx=0, got %d", s.CurrentIdx)
	}
	if s.State != StateIdle {
		t.Errorf("expected State=idle, got %s", s.State)
	}
	if s.Speed != 1.0 {
		t.Errorf("expected Speed=1.0, got %f", s.Speed)
	}
}

func TestStep_AdvancesIndex(t *testing.T) {
	candles := makeCandles(5)
	s := NewSession("id-2", 1, candles, makeEquity(candles), nil)

	msgs, err := s.Step()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.CurrentIdx != 1 {
		t.Errorf("expected CurrentIdx=1 after step, got %d", s.CurrentIdx)
	}
	if len(msgs) == 0 {
		t.Error("expected at least one message from Step()")
	}
	// First message should be a candle
	if msgs[0].Type != MsgCandle {
		t.Errorf("expected first message type=%s, got %s", MsgCandle, msgs[0].Type)
	}
}

func TestStep_AtLastCandle_ReturnsComplete(t *testing.T) {
	candles := makeCandles(2)
	s := NewSession("id-3", 1, candles, makeEquity(candles), nil)
	s.CurrentIdx = 1 // already at last candle

	_, err := s.Step()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// After stepping past the last candle, state must be complete
	if s.State != StateComplete {
		t.Errorf("expected State=complete after last candle, got %s", s.State)
	}
}

func TestSeek_JumpsToIndex(t *testing.T) {
	candles := makeCandles(10)
	s := NewSession("id-4", 1, candles, makeEquity(candles), nil)

	snapshot := s.Seek(5)
	if s.CurrentIdx != 5 {
		t.Errorf("expected CurrentIdx=5 after seek, got %d", s.CurrentIdx)
	}
	if snapshot.Type != MsgSeekSnapshot {
		t.Errorf("expected type=%s, got %s", MsgSeekSnapshot, snapshot.Type)
	}
	// The snapshot candles should be the first 5 (indices 0-4)
	data, ok := snapshot.Data.(SeekSnapshotData)
	if !ok {
		t.Fatal("snapshot.Data is not SeekSnapshotData")
	}
	if len(data.Candles) != 5 {
		t.Errorf("expected 5 candles in snapshot, got %d", len(data.Candles))
	}
}

func TestSeek_ClampsToBounds(t *testing.T) {
	candles := makeCandles(5)
	s := NewSession("id-5", 1, candles, makeEquity(candles), nil)

	s.Seek(999) // beyond end
	if s.CurrentIdx != len(candles)-1 {
		t.Errorf("expected CurrentIdx clamped to %d, got %d", len(candles)-1, s.CurrentIdx)
	}
}

func TestSetSpeed_ChangesInterval(t *testing.T) {
	candles := makeCandles(5)
	s := NewSession("id-6", 1, candles, makeEquity(candles), nil)

	s.SetSpeed(2.0)
	expected := baseInterval / 2
	got := s.Interval()
	if got != expected {
		t.Errorf("expected interval=%v at 2x speed, got %v", expected, got)
	}
}

func TestSetSpeed_InvalidIgnored(t *testing.T) {
	candles := makeCandles(5)
	s := NewSession("id-7", 1, candles, makeEquity(candles), nil)

	s.SetSpeed(-1)
	if s.Speed != 1.0 {
		t.Errorf("expected speed unchanged at 1.0, got %f", s.Speed)
	}
}

func TestStep_EmitsTradeOpen_AtEntryCandle(t *testing.T) {
	candles := makeCandles(5)
	equity := makeEquity(candles)

	// Trade opened at candle index 2
	entryTime := candles[2].Timestamp
	exitTime := candles[4].Timestamp
	pnl := 100.0
	pnlPct := 10.0
	exitPrice := candles[4].Close
	trade := models.Trade{
		Symbol:     "BTC/USDT",
		Direction:  models.TradeDirectionLong,
		EntryPrice: candles[2].Close,
		ExitPrice:  &exitPrice,
		EntryTime:  entryTime,
		ExitTime:   &exitTime,
		PnL:        &pnl,
		PnLPercent: &pnlPct,
	}

	s := NewSession("id-8", 1, candles, equity, []models.Trade{trade})
	s.CurrentIdx = 2

	msgs, err := s.Step()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasTradeOpen := false
	for _, m := range msgs {
		if m.Type == MsgTradeOpen {
			hasTradeOpen = true
		}
	}
	if !hasTradeOpen {
		t.Error("expected a trade_open message when stepping into candle at entry time")
	}
}
```

---

**Step 2: Run tests — expect compile failure**

```bash
cd backend && go test ./internal/replay/... 2>&1
```

Expected: `cannot find package "github.com/trader-claude/backend/internal/replay"`

---

**Step 3: Implement `session.go`**

Create `backend/internal/replay/session.go`:

```go
package replay

import (
	"fmt"
	"sync"
	"time"

	"github.com/trader-claude/backend/internal/backtest"
	"github.com/trader-claude/backend/internal/models"
	"github.com/trader-claude/backend/internal/registry"
)

// baseInterval is the candle-to-candle delay at 1x speed.
const baseInterval = 300 * time.Millisecond

// Session state constants.
const (
	StateIdle     = "idle"
	StatePlaying  = "playing"
	StatePaused   = "paused"
	StateComplete = "complete"
)

// Message type constants.
const (
	MsgCandle       = "candle"
	MsgTradeOpen    = "trade_open"
	MsgTradeClose   = "trade_close"
	MsgEquityUpdate = "equity_update"
	MsgSeekSnapshot = "seek_snapshot"
	MsgStatus       = "status"
)

// Message is a single outbound WS frame.
type Message struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// StatusData is the payload of a status message.
type StatusData struct {
	State   string `json:"state"`
	Index   int    `json:"index"`
	Total   int    `json:"total"`
	Speed   float64 `json:"speed"`
}

// SeekSnapshotData is the payload of a seek_snapshot message.
type SeekSnapshotData struct {
	Candles []registry.Candle     `json:"candles"`
	Equity  []backtest.EquityPoint `json:"equity"`
	Trades  []models.Trade        `json:"trades"`
}

// TradeEventData is the payload of trade_open / trade_close messages.
type TradeEventData struct {
	Trade models.Trade `json:"trade"`
}

// EquityUpdateData is the payload of equity_update messages.
type EquityUpdateData struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// ControlMsg is an inbound WS frame from the client.
type ControlMsg struct {
	Type  string  `json:"type"`
	Speed float64 `json:"speed,omitempty"`
	Index int     `json:"index,omitempty"`
}

// Session holds the full state of one replay session.
type Session struct {
	ID          string
	RunID       int64
	State       string
	Speed       float64
	CurrentIdx  int
	ControlChan chan ControlMsg

	candles     []registry.Candle
	equity      []backtest.EquityPoint
	tradeByOpen map[time.Time]models.Trade // keyed by EntryTime
	tradeByClose map[time.Time]models.Trade // keyed by ExitTime
	mu          sync.Mutex
}

// NewSession creates a new Session. trades may be nil.
func NewSession(id string, runID int64, candles []registry.Candle, equity []backtest.EquityPoint, trades []models.Trade) *Session {
	tradeByOpen := make(map[time.Time]models.Trade, len(trades))
	tradeByClose := make(map[time.Time]models.Trade, len(trades))
	for _, t := range trades {
		tradeByOpen[t.EntryTime] = t
		if t.ExitTime != nil {
			tradeByClose[*t.ExitTime] = t
		}
	}

	return &Session{
		ID:           id,
		RunID:        runID,
		State:        StateIdle,
		Speed:        1.0,
		CurrentIdx:   0,
		ControlChan:  make(chan ControlMsg, 32),
		candles:      candles,
		equity:       equity,
		tradeByOpen:  tradeByOpen,
		tradeByClose: tradeByClose,
	}
}

// Interval returns the tick duration at the current speed.
func (s *Session) Interval() time.Duration {
	if s.Speed <= 0 {
		return baseInterval
	}
	return time.Duration(float64(baseInterval) / s.Speed)
}

// SetSpeed sets the playback speed multiplier. Ignored if <= 0.
func (s *Session) SetSpeed(speed float64) {
	if speed <= 0 {
		return
	}
	s.Speed = speed
}

// Step advances one candle and returns the messages to emit.
// Returns ErrComplete if already past the last candle.
func (s *Session) Step() ([]Message, error) {
	if s.CurrentIdx >= len(s.candles) {
		s.State = StateComplete
		return []Message{s.statusMsg()}, nil
	}

	candle := s.candles[s.CurrentIdx]
	msgs := make([]Message, 0, 4)

	// Candle message
	msgs = append(msgs, Message{Type: MsgCandle, Data: candle})

	// Trade open at this candle?
	if trade, ok := s.tradeByOpen[candle.Timestamp]; ok {
		msgs = append(msgs, Message{Type: MsgTradeOpen, Data: TradeEventData{Trade: trade}})
	}

	// Trade close at this candle?
	if trade, ok := s.tradeByClose[candle.Timestamp]; ok {
		msgs = append(msgs, Message{Type: MsgTradeClose, Data: TradeEventData{Trade: trade}})
	}

	// Equity update
	if s.CurrentIdx < len(s.equity) {
		ep := s.equity[s.CurrentIdx]
		msgs = append(msgs, Message{Type: MsgEquityUpdate, Data: EquityUpdateData{Timestamp: ep.Timestamp, Value: ep.Value}})
	}

	s.CurrentIdx++

	// Check complete
	if s.CurrentIdx >= len(s.candles) {
		s.State = StateComplete
	}

	return msgs, nil
}

// Seek jumps to the given candle index and returns a seek_snapshot message.
// Index is clamped to [0, len(candles)-1].
func (s *Session) Seek(idx int) Message {
	if idx < 0 {
		idx = 0
	}
	if idx >= len(s.candles) {
		idx = len(s.candles) - 1
	}
	s.CurrentIdx = idx
	s.State = StatePaused

	return Message{
		Type: MsgSeekSnapshot,
		Data: SeekSnapshotData{
			Candles: s.candles[:idx],
			Equity:  s.equity[:idx],
			Trades:  s.tradesUpTo(idx),
		},
	}
}

// tradesUpTo returns all trades whose entry candle is before index idx.
func (s *Session) tradesUpTo(idx int) []models.Trade {
	if idx == 0 {
		return nil
	}
	cutoff := s.candles[idx].Timestamp
	var result []models.Trade
	for _, t := range s.tradeByOpen {
		if t.EntryTime.Before(cutoff) {
			result = append(result, t)
		}
	}
	return result
}

// StatusMsg returns the current status message.
func (s *Session) statusMsg() Message {
	return Message{
		Type: MsgStatus,
		Data: StatusData{
			State: s.State,
			Index: s.CurrentIdx,
			Total: len(s.candles),
			Speed: s.Speed,
		},
	}
}

// Total returns the number of candles in the session.
func (s *Session) Total() int { return len(s.candles) }

// Run is the blocking event loop that drives the session over the WS connection.
// It exits when the done channel is closed (client disconnected) or replay completes.
// writeFn must be goroutine-safe only from the Run goroutine (single writer pattern).
func (s *Session) Run(writeFn func(Message) error, done <-chan struct{}) error {
	ticker := time.NewTicker(24 * time.Hour) // effectively stopped; reset on "start"
	defer ticker.Stop()

	// Send initial status
	if err := writeFn(s.statusMsg()); err != nil {
		return fmt.Errorf("replay: initial write failed: %w", err)
	}

	for {
		select {
		case <-done:
			return nil

		case ctrl, ok := <-s.ControlChan:
			if !ok {
				return nil
			}
			switch ctrl.Type {
			case "start", "resume":
				s.State = StatePlaying
				ticker.Reset(s.Interval())
			case "pause":
				s.State = StatePaused
				ticker.Reset(24 * time.Hour)
			case "step":
				msgs, _ := s.Step()
				for _, m := range msgs {
					_ = writeFn(m)
				}
				if s.State == StateComplete {
					return nil
				}
			case "set_speed":
				s.SetSpeed(ctrl.Speed)
				if s.State == StatePlaying {
					ticker.Reset(s.Interval())
				}
			case "seek":
				snapshot := s.Seek(ctrl.Index)
				_ = writeFn(snapshot)
			}
			_ = writeFn(s.statusMsg())

		case <-ticker.C:
			if s.State != StatePlaying {
				continue
			}
			msgs, _ := s.Step()
			for _, m := range msgs {
				_ = writeFn(m)
			}
			if s.State == StateComplete {
				return nil
			}
			// Reset ticker for next candle (speed may have changed)
			ticker.Reset(s.Interval())
		}
	}
}
```

---

**Step 4: Run tests — expect pass**

```bash
cd backend && go test ./internal/replay/... -v 2>&1
```

Expected: all tests PASS.

---

**Step 5: Commit**

```bash
git add backend/internal/replay/
git commit -m "feat(phase4): add ReplaySession with step/seek/speed logic and unit tests"
```

---

## Task 3: ReplayManager

**Files:**
- Create: `backend/internal/replay/manager.go`
- Create: `backend/internal/replay/manager_test.go`

---

**Step 1: Write the failing tests**

Create `backend/internal/replay/manager_test.go`:

```go
package replay

import (
	"testing"
)

func TestManager_StoreAndRetrieve(t *testing.T) {
	m := NewManager()
	candles := makeCandles(5)
	s := NewSession("abc", 1, candles, makeEquity(candles), nil)

	m.Store(s)
	got, ok := m.Get("abc")
	if !ok {
		t.Fatal("expected to find session abc")
	}
	if got.ID != "abc" {
		t.Errorf("expected ID=abc, got %s", got.ID)
	}
}

func TestManager_Delete(t *testing.T) {
	m := NewManager()
	candles := makeCandles(3)
	s := NewSession("xyz", 2, candles, makeEquity(candles), nil)
	m.Store(s)

	m.Delete("xyz")
	_, ok := m.Get("xyz")
	if ok {
		t.Error("expected session xyz to be deleted")
	}
}

func TestManager_GetMissing_ReturnsFalse(t *testing.T) {
	m := NewManager()
	_, ok := m.Get("nonexistent")
	if ok {
		t.Error("expected ok=false for nonexistent session")
	}
}
```

---

**Step 2: Run to see failure**

```bash
cd backend && go test ./internal/replay/... -run TestManager 2>&1
```

Expected: FAIL — `NewManager` undefined.

---

**Step 3: Implement `manager.go`**

Create `backend/internal/replay/manager.go`:

```go
package replay

import "sync"

// Manager is the in-memory registry of active replay sessions.
// Safe for concurrent access.
type Manager struct {
	sessions sync.Map
}

// NewManager creates an empty Manager.
func NewManager() *Manager {
	return &Manager{}
}

// Store registers a session under its ID.
func (m *Manager) Store(s *Session) {
	m.sessions.Store(s.ID, s)
}

// Get retrieves a session by ID. Returns false if not found.
func (m *Manager) Get(id string) (*Session, bool) {
	v, ok := m.sessions.Load(id)
	if !ok {
		return nil, false
	}
	return v.(*Session), true
}

// Delete removes a session by ID.
func (m *Manager) Delete(id string) {
	m.sessions.Delete(id)
}
```

---

**Step 4: Run tests — expect pass**

```bash
cd backend && go test ./internal/replay/... -v 2>&1
```

Expected: all tests PASS.

---

**Step 5: Commit**

```bash
git add backend/internal/replay/manager.go backend/internal/replay/manager_test.go
git commit -m "feat(phase4): add ReplayManager (sync.Map registry) with tests"
```

---

## Task 4: Replay HTTP + WebSocket Handlers + Routes

**Files:**
- Create: `backend/internal/api/replay.go`
- Create: `backend/internal/api/replay_test.go`
- Modify: `backend/internal/api/routes.go`
- Modify: `backend/cmd/server/main.go` (pass manager to routes)

---

**Step 1: Write handler tests first**

Create `backend/internal/api/replay_test.go`:

```go
package api

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/trader-claude/backend/internal/replay"
)

// setupReplayApp builds a minimal Fiber app with just the replay routes
// and a pre-seeded manager.
func setupReplayApp(mgr *replay.Manager) *fiber.App {
	app := fiber.New()
	rh := &replayHandler{db: nil, ds: nil, manager: mgr}
	app.Post("/api/v1/backtest/runs/:id/replay", rh.createReplay)
	app.Post("/api/v1/replay/bookmarks", rh.createBookmark)
	app.Get("/api/v1/replay/bookmarks", rh.listBookmarks)
	app.Delete("/api/v1/replay/bookmarks/:id", rh.deleteBookmark)
	return app
}

func TestCreateReplay_InvalidID(t *testing.T) {
	mgr := replay.NewManager()
	app := setupReplayApp(mgr)

	req := httptest.NewRequest("POST", "/api/v1/backtest/runs/not-a-number/replay", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateReplay_RequiresDB(t *testing.T) {
	// Without a DB, createReplay should return 500 (can't load backtest).
	// This test confirms the handler path exists and handles nil DB gracefully.
	mgr := replay.NewManager()
	app := setupReplayApp(mgr)

	req := httptest.NewRequest("POST", "/api/v1/backtest/runs/1/replay", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	// 500 because db is nil — acceptable in unit test context
	if resp.StatusCode == fiber.StatusOK {
		t.Error("expected non-200 when db is nil")
	}
}

func TestListBookmarks_NoDB_Returns500(t *testing.T) {
	mgr := replay.NewManager()
	app := setupReplayApp(mgr)

	req := httptest.NewRequest("GET", "/api/v1/replay/bookmarks?run_id=1", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode == fiber.StatusOK {
		t.Error("expected non-200 when db is nil")
	}
}

func TestDeleteBookmark_InvalidID(t *testing.T) {
	mgr := replay.NewManager()
	app := setupReplayApp(mgr)

	req := httptest.NewRequest("DELETE", "/api/v1/replay/bookmarks/not-a-number", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

// TestResponseShape_CreateBookmark verifies the error JSON shape.
func TestResponseShape_CreateBookmark_NoBody(t *testing.T) {
	mgr := replay.NewManager()
	app := setupReplayApp(mgr)

	req := httptest.NewRequest("POST", "/api/v1/replay/bookmarks", nil)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	var res map[string]interface{}
	if err := json.Unmarshal(body, &res); err != nil {
		t.Fatalf("response is not JSON: %s", string(body))
	}
	if _, ok := res["error"]; !ok {
		t.Error("expected 'error' key in response")
	}
}
```

---

**Step 2: Run to see failure**

```bash
cd backend && go test ./internal/api/... -run TestCreateReplay 2>&1
```

Expected: FAIL — `replayHandler` undefined.

---

**Step 3: Implement `replay.go`**

Create `backend/internal/api/replay.go`:

```go
package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/adapter"
	"github.com/trader-claude/backend/internal/backtest"
	"github.com/trader-claude/backend/internal/models"
	"github.com/trader-claude/backend/internal/registry"
	"github.com/trader-claude/backend/internal/replay"
)

// replayHandler holds dependencies for replay and bookmark endpoints.
type replayHandler struct {
	db      *gorm.DB
	ds      *adapter.DataService
	manager *replay.Manager
}

func newReplayHandler(db *gorm.DB, ds *adapter.DataService, mgr *replay.Manager) *replayHandler {
	return &replayHandler{db: db, ds: ds, manager: mgr}
}

// ---- createReplay -------------------------------------------------------

// createReplay handles POST /api/v1/backtest/runs/:id/replay
func (h *replayHandler) createReplay(c *fiber.Ctx) error {
	runID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid backtest run id"})
	}

	// Load backtest record
	var bt models.Backtest
	if err := h.db.WithContext(c.Context()).First(&bt, runID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "backtest not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load backtest"})
	}

	if bt.Status != models.BacktestStatusCompleted {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "backtest must be completed before replay"})
	}

	// Load trades
	var trades []models.Trade
	if err := h.db.WithContext(c.Context()).Where("backtest_id = ?", runID).Order("entry_time ASC").Find(&trades).Error; err != nil {
		trades = []models.Trade{}
	}

	// Look up adapter by AdapterID (or fall back to market-based lookup)
	var mktAdapter registry.MarketAdapter
	if bt.AdapterID != "" {
		mktAdapter, err = registry.Adapters().Get(bt.AdapterID)
		if err != nil {
			log.Printf("[replay] adapter %q not found, falling back to market lookup", bt.AdapterID)
		}
	}
	if mktAdapter == nil {
		// Fallback: pick first adapter supporting this market
		for _, name := range registry.Adapters().Names() {
			a, _ := registry.Adapters().Get(name)
			for _, m := range a.Markets() {
				if m == bt.Market {
					mktAdapter = a
					break
				}
			}
			if mktAdapter != nil {
				break
			}
		}
	}
	if mktAdapter == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "no adapter found for market " + bt.Market})
	}

	// Re-fetch candles (same call as original backtest)
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()

	modelCandles, err := h.ds.GetCandles(ctx, mktAdapter, bt.Symbol, bt.Market, bt.Timeframe, bt.StartDate, bt.EndDate)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to fetch candles: " + err.Error()})
	}

	// Convert to registry.Candle
	candles := make([]registry.Candle, len(modelCandles))
	for i, mc := range modelCandles {
		candles[i] = registry.Candle{
			Symbol:    mc.Symbol,
			Market:    mc.Market,
			Timeframe: mc.Timeframe,
			Timestamp: mc.Timestamp,
			Open:      mc.Open,
			High:      mc.High,
			Low:       mc.Low,
			Close:     mc.Close,
			Volume:    mc.Volume,
		}
	}

	// Parse equity curve from stored JSON
	equity := make([]backtest.EquityPoint, 0, len(bt.EquityCurve))
	for _, item := range bt.EquityCurve {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		tsStr, _ := m["timestamp"].(string)
		val, _ := m["value"].(float64)
		ts, _ := time.Parse(time.RFC3339, tsStr)
		equity = append(equity, backtest.EquityPoint{Timestamp: ts, Value: val})
	}

	// Generate replay ID
	replayID, err := generateID()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to generate replay id"})
	}

	// Create and register session
	session := replay.NewSession(replayID, runID, candles, equity, trades)
	h.manager.Store(session)

	return c.JSON(fiber.Map{
		"replay_id":    replayID,
		"total_candles": len(candles),
	})
}

// ---- replayWS -----------------------------------------------------------

// replayWS handles WS /ws/replay/:replay_id
func (h *replayHandler) replayWS(c *websocket.Conn) {
	replayID := c.Params("replay_id")
	session, ok := h.manager.Get(replayID)
	if !ok {
		_ = c.WriteJSON(fiber.Map{"type": "error", "message": "replay session not found"})
		return
	}
	defer h.manager.Delete(replayID)

	// done is closed when the read loop exits (client disconnected)
	done := make(chan struct{})

	// Read loop: forward control messages into session.ControlChan
	go func() {
		defer close(done)
		for {
			var ctrl replay.ControlMsg
			if err := c.ReadJSON(&ctrl); err != nil {
				return
			}
			select {
			case session.ControlChan <- ctrl:
			default:
				// drop if channel full
			}
		}
	}()

	// Write function (single writer — only called from this goroutine)
	writeFn := func(msg replay.Message) error {
		return c.WriteJSON(msg)
	}

	// Run the session (blocks until done or complete)
	if err := session.Run(writeFn, done); err != nil {
		log.Printf("[replay %s] session error: %v", replayID, err)
	}
}

// ---- Bookmark endpoints -------------------------------------------------

type createBookmarkRequest struct {
	BacktestRunID int64  `json:"backtest_run_id"`
	CandleIndex   int    `json:"candle_index"`
	Label         string `json:"label"`
	Note          string `json:"note"`
	ChartSnapshot string `json:"chart_snapshot"` // base64 PNG from canvas.toDataURL()
}

// createBookmark handles POST /api/v1/replay/bookmarks
func (h *replayHandler) createBookmark(c *fiber.Ctx) error {
	var req createBookmarkRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.BacktestRunID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "backtest_run_id is required"})
	}

	bm := models.ReplayBookmark{
		UserID:        1,
		BacktestRunID: req.BacktestRunID,
		CandleIndex:   req.CandleIndex,
		Label:         req.Label,
		Note:          req.Note,
		ChartSnapshot: req.ChartSnapshot,
	}

	if err := h.db.WithContext(c.Context()).Create(&bm).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to save bookmark"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"data": bm})
}

// listBookmarks handles GET /api/v1/replay/bookmarks?run_id=N
func (h *replayHandler) listBookmarks(c *fiber.Ctx) error {
	runIDStr := c.Query("run_id")
	if runIDStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "run_id query param is required"})
	}
	runID, err := strconv.ParseInt(runIDStr, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid run_id"})
	}

	var bookmarks []models.ReplayBookmark
	if err := h.db.WithContext(c.Context()).
		Where("backtest_run_id = ?", runID).
		Order("created_at DESC").
		Find(&bookmarks).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list bookmarks"})
	}

	if bookmarks == nil {
		bookmarks = []models.ReplayBookmark{}
	}

	return c.JSON(fiber.Map{"data": bookmarks})
}

// getBookmark handles GET /api/v1/replay/bookmarks/:id
func (h *replayHandler) getBookmark(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	var bm models.ReplayBookmark
	if err := h.db.WithContext(c.Context()).First(&bm, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "bookmark not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to fetch bookmark"})
	}

	return c.JSON(fiber.Map{"data": bm})
}

// deleteBookmark handles DELETE /api/v1/replay/bookmarks/:id
func (h *replayHandler) deleteBookmark(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	result := h.db.WithContext(c.Context()).Delete(&models.ReplayBookmark{}, id)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete bookmark"})
	}
	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "bookmark not found"})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// ---- helpers ------------------------------------------------------------

func generateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("rand.Read: %w", err)
	}
	return hex.EncodeToString(b), nil
}
```

---

**Step 4: Wire routes in `routes.go`**

In `backend/internal/api/routes.go`:

1. Add `mgr *replay.Manager` parameter to `RegisterRoutes`:
```go
func RegisterRoutes(app *fiber.App, db *gorm.DB, rdb *redis.Client, hub *ws.Hub, version string, pool *worker.WorkerPool, ds *adapter.DataService, mgr *replay.Manager) {
```

2. Add import: `"github.com/trader-claude/backend/internal/replay"`

3. Wire the handlers (after the existing backtest routes):
```go
// --- Replay ---
rh := newReplayHandler(db, ds, mgr)
v1.Post("/backtest/runs/:id/replay", rh.createReplay)
v1.Post("/replay/bookmarks", rh.createBookmark)
v1.Get("/replay/bookmarks", rh.listBookmarks)
v1.Get("/replay/bookmarks/:id", rh.getBookmark)
v1.Delete("/replay/bookmarks/:id", rh.deleteBookmark)

// Replay WebSocket (after the existing WS middleware)
app.Get("/ws/replay/:replay_id", websocket.New(rh.replayWS))
```

---

**Step 5: Update `main.go` to pass manager**

In `backend/cmd/server/main.go`, create the manager and pass it to `RegisterRoutes`:

```go
// After worker pool setup, before routes:
replayMgr := replay.NewManager()

// Update RegisterRoutes call:
api.RegisterRoutes(app, db, rdb, hub, cfg.AppVersion, pool, ds, replayMgr)
```

Add import: `"github.com/trader-claude/backend/internal/replay"`

---

**Step 6: Run all tests**

```bash
cd backend && go test ./... 2>&1
```

Expected: all PASS.

---

**Step 7: Commit**

```bash
git add backend/internal/api/replay.go backend/internal/api/replay_test.go backend/internal/api/routes.go backend/cmd/server/main.go
git commit -m "feat(phase4): add replay HTTP+WS handlers, bookmark CRUD, and wire routes"
```

---

## Task 5: Frontend Types + Store + API Client

**Files:**
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/stores/index.ts`
- Create: `frontend/src/api/replay.ts`

---

**Step 1: Add replay types to `types/index.ts`**

Append to `frontend/src/types/index.ts`:

```ts
// ── Replay types ───────────────────────────────────────────────────────────

export type ReplayState = 'idle' | 'playing' | 'paused' | 'complete'

export interface ReplayStatus {
  state: ReplayState
  index: number
  total: number
  speed: number
}

export interface ReplayCandle {
  symbol: string
  market: string
  timeframe: string
  timestamp: string
  open: number
  high: number
  low: number
  close: number
  volume: number
}

export interface ReplayEquityPoint {
  timestamp: string
  value: number
}

export interface ReplayTradeEvent {
  trade: Trade
}

export interface ReplaySeekSnapshot {
  candles: ReplayCandle[]
  equity: ReplayEquityPoint[]
  trades: Trade[]
}

// Discriminated union of all server → client messages
export type ReplayServerMsg =
  | { type: 'candle';        data: ReplayCandle }
  | { type: 'trade_open';    data: ReplayTradeEvent }
  | { type: 'trade_close';   data: ReplayTradeEvent }
  | { type: 'equity_update'; data: ReplayEquityPoint }
  | { type: 'seek_snapshot'; data: ReplaySeekSnapshot }
  | { type: 'status';        data: ReplayStatus }
  | { type: 'error';         message: string }

// Client → server control messages
export interface ReplayControlMsg {
  type: 'start' | 'pause' | 'resume' | 'step' | 'set_speed' | 'seek'
  speed?: number
  index?: number
}

export interface ReplayBookmark {
  id: number
  user_id: number
  backtest_run_id: number
  candle_index: number
  label: string
  note: string
  chart_snapshot: string // base64 PNG (may be empty when listing)
  created_at: string
}

export interface CreateBookmarkRequest {
  backtest_run_id: number
  candle_index: number
  label: string
  note: string
  chart_snapshot: string
}
```

---

**Step 2: Add replay state to `backtestStore` in `stores/index.ts`**

Replace the current `BacktestStore` interface and `useBacktestStore`:

```ts
// ── Backtest store ─────────────────────────────────────────────────────────

interface BacktestStore {
  backtests: Backtest[]
  activeBacktest: Backtest | null
  setBacktests: (b: Backtest[]) => void
  setActiveBacktest: (b: Backtest | null) => void
  updateBacktest: (b: Backtest) => void

  // Replay
  replayActive: boolean
  replayId: string | null
  replayState: import('@/types').ReplayState
  replayIndex: number
  replayTotal: number
  replaySpeed: number
  replayCandles: import('@/types').ReplayCandle[]
  replayEquity: import('@/types').ReplayEquityPoint[]
  replayTrades: import('@/types').Trade[]
  setReplayActive: (active: boolean, replayId?: string | null) => void
  applyReplayMsg: (msg: import('@/types').ReplayServerMsg) => void
  resetReplay: () => void
}

export const useBacktestStore = create<BacktestStore>()((set) => ({
  backtests: [],
  activeBacktest: null,
  setBacktests: (backtests) => set({ backtests }),
  setActiveBacktest: (activeBacktest) => set({ activeBacktest }),
  updateBacktest: (b) =>
    set((s) => ({
      backtests: s.backtests.map((x) => (x.id === b.id ? b : x)),
      activeBacktest: s.activeBacktest?.id === b.id ? b : s.activeBacktest,
    })),

  // Replay initial state
  replayActive: false,
  replayId: null,
  replayState: 'idle',
  replayIndex: 0,
  replayTotal: 0,
  replaySpeed: 1,
  replayCandles: [],
  replayEquity: [],
  replayTrades: [],

  setReplayActive: (active, replayId = null) =>
    set({ replayActive: active, replayId }),

  applyReplayMsg: (msg) =>
    set((s) => {
      switch (msg.type) {
        case 'status':
          return {
            replayState: msg.data.state,
            replayIndex: msg.data.index,
            replayTotal: msg.data.total,
            replaySpeed: msg.data.speed,
          }
        case 'candle':
          return { replayCandles: [...s.replayCandles, msg.data] }
        case 'equity_update':
          return { replayEquity: [...s.replayEquity, msg.data] }
        case 'trade_open':
        case 'trade_close':
          return { replayTrades: [...s.replayTrades, msg.data.trade] }
        case 'seek_snapshot':
          return {
            replayCandles: msg.data.candles,
            replayEquity: msg.data.equity,
            replayTrades: msg.data.trades,
          }
        default:
          return {}
      }
    }),

  resetReplay: () =>
    set({
      replayActive: false,
      replayId: null,
      replayState: 'idle',
      replayIndex: 0,
      replayTotal: 0,
      replaySpeed: 1,
      replayCandles: [],
      replayEquity: [],
      replayTrades: [],
    }),
}))
```

---

**Step 3: Create `frontend/src/api/replay.ts`**

```ts
import apiClient from './client'
import type { ReplayBookmark, CreateBookmarkRequest } from '@/types'

export async function createReplaySession(runId: number): Promise<{ replay_id: string; total_candles: number }> {
  const res = await apiClient.post(`/backtest/runs/${runId}/replay`)
  return res.data
}

export async function createBookmark(req: CreateBookmarkRequest): Promise<ReplayBookmark> {
  const res = await apiClient.post('/replay/bookmarks', req)
  return res.data.data
}

export async function listBookmarks(runId: number): Promise<ReplayBookmark[]> {
  const res = await apiClient.get('/replay/bookmarks', { params: { run_id: runId } })
  return res.data.data
}

export async function deleteBookmark(id: number): Promise<void> {
  await apiClient.delete(`/replay/bookmarks/${id}`)
}
```

---

**Step 4: Verify TypeScript compiles**

```bash
cd frontend && npx tsc --noEmit 2>&1
```

Expected: no errors.

---

**Step 5: Commit**

```bash
git add frontend/src/types/index.ts frontend/src/stores/index.ts frontend/src/api/replay.ts
git commit -m "feat(phase4): add replay types, store fields, and API client functions"
```

---

## Task 6: useReplayWS Hook + ReplayChart + ReplayOverlay Skeleton

**Files:**
- Create: `frontend/src/hooks/useReplayWS.ts`
- Create: `frontend/src/components/replay/ReplayChart.tsx`
- Create: `frontend/src/components/replay/ReplayOverlay.tsx`

The WS URL pattern from the project is `ws://localhost:8080` (from `VITE_WS_URL`).

---

**Step 1: Create `useReplayWS.ts`**

Create `frontend/src/hooks/useReplayWS.ts`:

```ts
import { useEffect, useRef, useCallback } from 'react'
import { useBacktestStore } from '@/stores'
import type { ReplayControlMsg, ReplayServerMsg } from '@/types'

const WS_BASE = (import.meta.env.VITE_WS_URL as string) ?? 'ws://localhost:8080'

export function useReplayWS(replayId: string | null) {
  const ws = useRef<WebSocket | null>(null)
  const applyMsg = useBacktestStore((s) => s.applyReplayMsg)
  const resetReplay = useBacktestStore((s) => s.resetReplay)

  useEffect(() => {
    if (!replayId) return

    const socket = new WebSocket(`${WS_BASE}/ws/replay/${replayId}`)
    ws.current = socket

    socket.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data) as ReplayServerMsg
        applyMsg(msg)
      } catch {
        // malformed message — ignore
      }
    }

    socket.onerror = () => {
      console.error('[ReplayWS] connection error')
    }

    socket.onclose = () => {
      ws.current = null
    }

    return () => {
      socket.close()
      ws.current = null
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [replayId])

  const sendControl = useCallback((msg: ReplayControlMsg) => {
    if (ws.current?.readyState === WebSocket.OPEN) {
      ws.current.send(JSON.stringify(msg))
    }
  }, [])

  return { sendControl }
}
```

---

**Step 2: Create `ReplayChart.tsx`**

Create `frontend/src/components/replay/ReplayChart.tsx`:

```tsx
import { useEffect, useRef } from 'react'
import {
  createChart,
  type IChartApi,
  type ISeriesApi,
  type CandlestickData,
  type Time,
  ColorType,
} from 'lightweight-charts'
import { useBacktestStore } from '@/stores'
import type { ReplayCandle, Trade } from '@/types'

interface ReplayChartProps {
  className?: string
}

function candleToChartData(c: ReplayCandle): CandlestickData {
  return {
    time: (new Date(c.timestamp).getTime() / 1000) as Time,
    open: c.open,
    high: c.high,
    low: c.low,
    close: c.close,
  }
}

export function ReplayChart({ className }: ReplayChartProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const chartRef = useRef<IChartApi | null>(null)
  const candleSeriesRef = useRef<ISeriesApi<'Candlestick'> | null>(null)

  const candles = useBacktestStore((s) => s.replayCandles)
  const trades = useBacktestStore((s) => s.replayTrades)

  // Create chart once
  useEffect(() => {
    if (!containerRef.current) return
    const chart = createChart(containerRef.current, {
      layout: {
        background: { type: ColorType.Solid, color: 'transparent' },
        textColor: '#9ca3af',
      },
      grid: {
        vertLines: { color: '#1f2937' },
        horzLines: { color: '#1f2937' },
      },
      timeScale: { borderColor: '#374151' },
      rightPriceScale: { borderColor: '#374151' },
    })
    const candleSeries = chart.addCandlestickSeries({
      upColor: '#22c55e',
      downColor: '#ef4444',
      borderUpColor: '#22c55e',
      borderDownColor: '#ef4444',
      wickUpColor: '#22c55e',
      wickDownColor: '#ef4444',
    })
    chartRef.current = chart
    candleSeriesRef.current = candleSeries

    const observer = new ResizeObserver(() => {
      if (containerRef.current) {
        chart.applyOptions({ width: containerRef.current.clientWidth })
      }
    })
    observer.observe(containerRef.current)

    return () => {
      observer.disconnect()
      chart.remove()
      chartRef.current = null
      candleSeriesRef.current = null
    }
  }, [])

  // Update candles (append-only when streaming, bulk replace on seek)
  useEffect(() => {
    if (!candleSeriesRef.current || candles.length === 0) return
    const data = candles.map(candleToChartData)
    candleSeriesRef.current.setData(data)
    chartRef.current?.timeScale().scrollToRealTime()
  }, [candles])

  // Update markers from trades
  useEffect(() => {
    if (!candleSeriesRef.current) return
    const markers = trades
      .flatMap((t): Array<{ time: Time; position: 'belowBar' | 'aboveBar'; color: string; shape: 'arrowUp' | 'arrowDown'; text: string }> => {
        const entryMarker = {
          time: (new Date(t.entry_time).getTime() / 1000) as Time,
          position: 'belowBar' as const,
          color: '#22c55e',
          shape: 'arrowUp' as const,
          text: 'BUY',
        }
        if (!t.exit_time) return [entryMarker]
        return [
          entryMarker,
          {
            time: (new Date(t.exit_time).getTime() / 1000) as Time,
            position: 'aboveBar' as const,
            color: '#ef4444',
            shape: 'arrowDown' as const,
            text: 'SELL',
          },
        ]
      })
      .sort((a, b) => (a.time as number) - (b.time as number))

    candleSeriesRef.current.setMarkers(markers)
  }, [trades])

  return <div ref={containerRef} className={className} style={{ height: '100%' }} />
}
```

---

**Step 3: Create `ReplayOverlay.tsx` skeleton**

Create `frontend/src/components/replay/ReplayOverlay.tsx`:

```tsx
import { useEffect } from 'react'
import { X } from 'lucide-react'
import { useBacktestStore } from '@/stores'
import { useReplayWS } from '@/hooks/useReplayWS'
import { ReplayChart } from './ReplayChart'

interface ReplayOverlayProps {
  runId: number
  strategyName: string
  symbol: string
  onClose: () => void
}

export function ReplayOverlay({ runId, strategyName, symbol, onClose }: ReplayOverlayProps) {
  const replayId = useBacktestStore((s) => s.replayId)
  const resetReplay = useBacktestStore((s) => s.resetReplay)
  const { sendControl } = useReplayWS(replayId)

  // Reset store on unmount
  useEffect(() => {
    return () => resetReplay()
  }, [resetReplay])

  // Close on Escape
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [onClose])

  return (
    <div className="fixed inset-0 z-50 flex flex-col bg-gray-950">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-gray-800">
        <div className="flex items-center gap-3">
          <span className="text-xs font-semibold uppercase tracking-widest text-blue-400">Replay</span>
          <span className="text-sm text-gray-300">{strategyName} / {symbol}</span>
        </div>
        <button
          onClick={onClose}
          className="rounded p-1.5 hover:bg-gray-800 text-gray-400 hover:text-white transition-colors"
          aria-label="Close replay"
        >
          <X className="h-4 w-4" />
        </button>
      </div>

      {/* Chart area — ReplayControlBar, EquityMiniChart, SignalToast added in Task 7 */}
      <div className="relative flex-1 overflow-hidden">
        <ReplayChart className="absolute inset-0" />
      </div>

      {/* Control bar placeholder — filled in Task 7 */}
      <div className="h-14 border-t border-gray-800 flex items-center px-4 gap-2 text-xs text-gray-500">
        Controls coming in Task 7
      </div>
    </div>
  )
}
```

---

**Step 4: Verify TypeScript compiles**

```bash
cd frontend && npx tsc --noEmit 2>&1
```

Expected: no errors.

---

**Step 5: Commit**

```bash
git add frontend/src/hooks/useReplayWS.ts frontend/src/components/replay/
git commit -m "feat(phase4): add useReplayWS hook, ReplayChart, and ReplayOverlay skeleton"
```

---

## Task 7: ReplayControlBar + EquityMiniChart + SignalToast

**Files:**
- Create: `frontend/src/components/replay/ReplayControlBar.tsx`
- Create: `frontend/src/components/replay/EquityMiniChart.tsx`
- Create: `frontend/src/components/replay/SignalToast.tsx`
- Modify: `frontend/src/components/replay/ReplayOverlay.tsx` (integrate them)

---

**Step 1: Create `ReplayControlBar.tsx`**

Create `frontend/src/components/replay/ReplayControlBar.tsx`:

```tsx
import { SkipBack, Rewind, Play, Pause, StepForward } from 'lucide-react'
import { useBacktestStore } from '@/stores'
import type { ReplayControlMsg } from '@/types'

interface ReplayControlBarProps {
  onControl: (msg: ReplayControlMsg) => void
}

const SPEED_OPTIONS = [0.25, 0.5, 1, 2, 5, 10]

export function ReplayControlBar({ onControl }: ReplayControlBarProps) {
  const state = useBacktestStore((s) => s.replayState)
  const index = useBacktestStore((s) => s.replayIndex)
  const total = useBacktestStore((s) => s.replayTotal)
  const speed = useBacktestStore((s) => s.replaySpeed)
  const candles = useBacktestStore((s) => s.replayCandles)

  const isComplete = state === 'complete'
  const isPlaying = state === 'playing'
  const timestampLabel = candles[index - 1]
    ? new Date(candles[index - 1].timestamp).toLocaleString()
    : '—'

  function handleSeek(e: React.ChangeEvent<HTMLInputElement>) {
    onControl({ type: 'seek', index: Number(e.target.value) })
  }

  return (
    <div className="h-14 border-t border-gray-800 flex items-center gap-4 px-4">
      {/* Transport buttons */}
      <button
        onClick={() => onControl({ type: 'seek', index: 0 })}
        disabled={index === 0}
        className="p-1.5 rounded hover:bg-gray-800 text-gray-400 hover:text-white disabled:opacity-30 transition-colors"
        title="Restart"
      >
        <SkipBack className="h-4 w-4" />
      </button>

      <button
        onClick={() => onControl({ type: 'step' })}
        disabled={isComplete || isPlaying}
        className="p-1.5 rounded hover:bg-gray-800 text-gray-400 hover:text-white disabled:opacity-30 transition-colors"
        title="Step forward"
      >
        <StepForward className="h-4 w-4" />
      </button>

      <button
        onClick={() => onControl({ type: isPlaying ? 'pause' : (state === 'idle' ? 'start' : 'resume') })}
        disabled={isComplete}
        className="p-2 rounded-full bg-blue-600 hover:bg-blue-500 text-white disabled:opacity-30 transition-colors"
        title={isPlaying ? 'Pause' : 'Play'}
      >
        {isPlaying ? <Pause className="h-4 w-4" /> : <Play className="h-4 w-4" />}
      </button>

      {/* Seek slider */}
      <div className="flex-1 flex items-center gap-3">
        <input
          type="range"
          min={0}
          max={Math.max(total - 1, 0)}
          value={index}
          onChange={handleSeek}
          className="flex-1 accent-blue-500 cursor-pointer"
        />
        <span className="text-xs text-gray-400 tabular-nums whitespace-nowrap">
          {index} / {total}
        </span>
      </div>

      {/* Speed chips */}
      <div className="flex items-center gap-1">
        {SPEED_OPTIONS.map((s) => (
          <button
            key={s}
            onClick={() => onControl({ type: 'set_speed', speed: s })}
            className={`px-2 py-0.5 text-xs rounded transition-colors ${
              speed === s
                ? 'bg-blue-600 text-white'
                : 'text-gray-400 hover:text-white hover:bg-gray-800'
            }`}
          >
            {s}x
          </button>
        ))}
      </div>

      {/* Timestamp */}
      <span className="text-xs text-gray-500 tabular-nums">{timestampLabel}</span>
    </div>
  )
}
```

---

**Step 2: Create `EquityMiniChart.tsx`**

Create `frontend/src/components/replay/EquityMiniChart.tsx`:

```tsx
import { LineChart, Line, Tooltip, ResponsiveContainer } from 'recharts'
import { useBacktestStore } from '@/stores'

export function EquityMiniChart() {
  const equity = useBacktestStore((s) => s.replayEquity)

  if (equity.length < 2) return null

  const latest = equity[equity.length - 1]
  const first = equity[0]
  const pct = first.value > 0 ? ((latest.value - first.value) / first.value) * 100 : 0
  const isPositive = pct >= 0

  return (
    <div className="absolute bottom-16 right-4 w-48 rounded-lg border border-gray-700 bg-gray-900/90 backdrop-blur p-2">
      <div className="flex items-baseline justify-between mb-1">
        <span className="text-xs text-gray-400">Equity</span>
        <span className={`text-xs font-semibold ${isPositive ? 'text-green-400' : 'text-red-400'}`}>
          {isPositive ? '+' : ''}{pct.toFixed(2)}%
        </span>
      </div>
      <div className="text-sm font-bold text-white mb-1">
        ${latest.value.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
      </div>
      <ResponsiveContainer width="100%" height={40}>
        <LineChart data={equity}>
          <Line
            type="monotone"
            dataKey="value"
            stroke={isPositive ? '#22c55e' : '#ef4444'}
            strokeWidth={1.5}
            dot={false}
            isAnimationActive={false}
          />
          <Tooltip
            contentStyle={{ background: '#111827', border: '1px solid #374151', borderRadius: 6, fontSize: 11 }}
            formatter={(v: number) => [`$${v.toFixed(2)}`, 'Equity']}
            labelFormatter={() => ''}
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  )
}
```

---

**Step 3: Create `SignalToast.tsx`**

Create `frontend/src/components/replay/SignalToast.tsx`:

```tsx
import { useEffect, useState } from 'react'
import { useBacktestStore } from '@/stores'
import type { Trade } from '@/types'

interface ToastEntry {
  id: number
  trade: Trade
  kind: 'open' | 'close'
}

let toastCounter = 0

export function SignalToast() {
  const trades = useBacktestStore((s) => s.replayTrades)
  const [toasts, setToasts] = useState<ToastEntry[]>([])
  const [seen, setSeen] = useState(new Set<number>())

  // Watch for new trades and spawn toasts
  useEffect(() => {
    const newTrades = trades.filter((t) => !seen.has(t.id ?? -1))
    if (newTrades.length === 0) return

    const newToasts: ToastEntry[] = []
    newTrades.forEach((t) => {
      if (t.id !== undefined) {
        newToasts.push({ id: ++toastCounter, trade: t, kind: 'open' })
        if (t.exit_time) {
          newToasts.push({ id: ++toastCounter, trade: t, kind: 'close' })
        }
      }
    })

    setSeen((s) => {
      const next = new Set(s)
      newTrades.forEach((t) => t.id !== undefined && next.add(t.id))
      return next
    })
    setToasts((prev) => [...prev, ...newToasts])
  }, [trades, seen])

  // Auto-dismiss after 8s
  useEffect(() => {
    if (toasts.length === 0) return
    const timer = setTimeout(() => {
      setToasts((prev) => prev.slice(1))
    }, 8000)
    return () => clearTimeout(timer)
  }, [toasts])

  if (toasts.length === 0) return null

  return (
    <div className="absolute bottom-16 left-4 flex flex-col gap-2">
      {toasts.slice(0, 3).map((entry) => (
        <div
          key={entry.id}
          className="flex items-center gap-3 px-3 py-2 rounded-lg border bg-gray-900/95 backdrop-blur text-sm shadow-lg animate-in slide-in-from-left-4"
          style={{ borderColor: entry.kind === 'open' ? '#22c55e55' : '#ef444455' }}
        >
          <span
            className={`font-bold text-xs px-1.5 py-0.5 rounded ${
              entry.kind === 'open' ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'
            }`}
          >
            {entry.kind === 'open' ? 'BUY' : 'SELL'}
          </span>
          <span className="text-gray-200">{entry.trade.symbol}</span>
          <span className="text-gray-400 tabular-nums">
            ${(entry.kind === 'open' ? entry.trade.entry_price : entry.trade.exit_price ?? 0).toFixed(2)}
          </span>
          <button
            onClick={() => setToasts((prev) => prev.filter((t) => t.id !== entry.id))}
            className="ml-auto text-gray-600 hover:text-gray-300 text-xs"
          >
            ✕
          </button>
        </div>
      ))}
    </div>
  )
}
```

---

**Step 4: Update `ReplayOverlay.tsx` to integrate all components**

Replace the placeholder content in `ReplayOverlay.tsx`:

```tsx
import { useEffect } from 'react'
import { X, Bookmark } from 'lucide-react'
import { useBacktestStore } from '@/stores'
import { useReplayWS } from '@/hooks/useReplayWS'
import { ReplayChart } from './ReplayChart'
import { ReplayControlBar } from './ReplayControlBar'
import { EquityMiniChart } from './EquityMiniChart'
import { SignalToast } from './SignalToast'

interface ReplayOverlayProps {
  runId: number
  strategyName: string
  symbol: string
  onClose: () => void
  onSave: () => void
}

export function ReplayOverlay({ runId, strategyName, symbol, onClose, onSave }: ReplayOverlayProps) {
  const replayId = useBacktestStore((s) => s.replayId)
  const resetReplay = useBacktestStore((s) => s.resetReplay)
  const { sendControl } = useReplayWS(replayId)

  useEffect(() => {
    return () => resetReplay()
  }, [resetReplay])

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
      if (e.key === ' ') {
        e.preventDefault()
        // Toggle play/pause — handled by control bar
      }
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [onClose])

  return (
    <div className="fixed inset-0 z-50 flex flex-col bg-gray-950">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-gray-800 shrink-0">
        <div className="flex items-center gap-3">
          <span className="text-xs font-semibold uppercase tracking-widest text-blue-400">Replay</span>
          <span className="text-sm text-gray-300">{strategyName} · {symbol}</span>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={onSave}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs rounded-md border border-gray-700 hover:bg-gray-800 text-gray-300 hover:text-white transition-colors"
          >
            <Bookmark className="h-3.5 w-3.5" />
            Save Bookmark
          </button>
          <button
            onClick={onClose}
            className="rounded p-1.5 hover:bg-gray-800 text-gray-400 hover:text-white transition-colors"
            aria-label="Close replay"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
      </div>

      {/* Chart area */}
      <div className="relative flex-1 overflow-hidden">
        <ReplayChart className="absolute inset-0" />
        <EquityMiniChart />
        <SignalToast />
      </div>

      {/* Control bar */}
      <ReplayControlBar onControl={sendControl} />
    </div>
  )
}
```

---

**Step 5: Verify TypeScript compiles**

```bash
cd frontend && npx tsc --noEmit 2>&1
```

Expected: no errors.

---

**Step 6: Commit**

```bash
git add frontend/src/components/replay/
git commit -m "feat(phase4): add ReplayControlBar, EquityMiniChart, SignalToast, and complete ReplayOverlay"
```

---

## Task 8: BookmarkModal + Wire into Backtest Page

**Files:**
- Create: `frontend/src/components/replay/BookmarkModal.tsx`
- Modify: `frontend/src/pages/Backtest.tsx`

---

**Step 1: Create `BookmarkModal.tsx`**

Create `frontend/src/components/replay/BookmarkModal.tsx`:

```tsx
import { useState, useRef } from 'react'
import { X, Bookmark, Loader2 } from 'lucide-react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { createBookmark } from '@/api/replay'
import { useBacktestStore } from '@/stores'

interface BookmarkModalProps {
  runId: number
  onClose: () => void
}

export function BookmarkModal({ runId, onClose }: BookmarkModalProps) {
  const [label, setLabel] = useState('')
  const [note, setNote] = useState('')
  const index = useBacktestStore((s) => s.replayIndex)
  const queryClient = useQueryClient()

  const { mutate, isPending } = useMutation({
    mutationFn: async () => {
      // Capture chart canvas screenshot
      const canvas = document.querySelector<HTMLCanvasElement>('canvas')
      const snapshot = canvas ? canvas.toDataURL('image/png') : ''

      return createBookmark({
        backtest_run_id: runId,
        candle_index: index,
        label: label.trim() || `Candle ${index}`,
        note: note.trim(),
        chart_snapshot: snapshot,
      })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['replay-bookmarks', runId] })
      onClose()
    },
  })

  return (
    <div className="fixed inset-0 z-[60] flex items-center justify-center bg-black/60">
      <div className="w-full max-w-md rounded-xl border border-gray-700 bg-gray-900 shadow-2xl">
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-gray-800">
          <div className="flex items-center gap-2 text-sm font-medium text-white">
            <Bookmark className="h-4 w-4 text-blue-400" />
            Save Replay Bookmark
          </div>
          <button onClick={onClose} className="text-gray-400 hover:text-white">
            <X className="h-4 w-4" />
          </button>
        </div>

        {/* Body */}
        <div className="px-5 py-4 space-y-4">
          <div>
            <label className="block text-xs text-gray-400 mb-1">Candle index</label>
            <div className="text-sm text-gray-300 font-mono">{index}</div>
          </div>

          <div>
            <label className="block text-xs text-gray-400 mb-1">Label</label>
            <input
              type="text"
              value={label}
              onChange={(e) => setLabel(e.target.value)}
              placeholder={`Candle ${index}`}
              className="w-full rounded-md border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
          </div>

          <div>
            <label className="block text-xs text-gray-400 mb-1">Note</label>
            <textarea
              value={note}
              onChange={(e) => setNote(e.target.value)}
              placeholder="What's interesting about this moment?"
              rows={3}
              className="w-full rounded-md border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-1 focus:ring-blue-500 resize-none"
            />
          </div>

          <p className="text-xs text-gray-500">
            A screenshot of the current chart will be saved automatically.
          </p>
        </div>

        {/* Footer */}
        <div className="flex justify-end gap-3 px-5 py-4 border-t border-gray-800">
          <button
            onClick={onClose}
            className="px-4 py-2 text-sm rounded-md text-gray-400 hover:text-white hover:bg-gray-800 transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={() => mutate()}
            disabled={isPending}
            className="flex items-center gap-2 px-4 py-2 text-sm rounded-md bg-blue-600 hover:bg-blue-500 text-white disabled:opacity-50 transition-colors"
          >
            {isPending && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
            Save Bookmark
          </button>
        </div>
      </div>
    </div>
  )
}
```

---

**Step 2: Add `useReplayBookmarks` query hook**

Append to `frontend/src/hooks/useBacktest.ts` (or create `frontend/src/hooks/useReplay.ts` if that file doesn't exist):

Check if `frontend/src/hooks/useBacktest.ts` exists. If yes, append there. If not, create `frontend/src/hooks/useReplay.ts`:

```ts
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { createReplaySession, listBookmarks, deleteBookmark } from '@/api/replay'

export function useCreateReplaySession() {
  return useMutation({
    mutationFn: (runId: number) => createReplaySession(runId),
  })
}

export function useReplayBookmarks(runId: number | null) {
  return useQuery({
    queryKey: ['replay-bookmarks', runId],
    queryFn: () => listBookmarks(runId!),
    enabled: !!runId,
  })
}

export function useDeleteBookmark(runId: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: deleteBookmark,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['replay-bookmarks', runId] }),
  })
}
```

---

**Step 3: Wire replay into `Backtest.tsx`**

Find the right panel in `Backtest.tsx` where the results tabs are shown (Overview, Trades, Chart). Add:

1. Import at the top:
```tsx
import { ReplayOverlay } from '@/components/replay/ReplayOverlay'
import { BookmarkModal } from '@/components/replay/BookmarkModal'
import { useCreateReplaySession, useReplayBookmarks, useDeleteBookmark } from '@/hooks/useReplay'
```

2. State variables (inside the page component):
```tsx
const [replayOpen, setReplayOpen] = useState(false)
const [bookmarkModalOpen, setBookmarkModalOpen] = useState(false)

const setReplayActive = useBacktestStore((s) => s.setReplayActive)
const createSession = useCreateReplaySession()
```

3. Handler to launch replay:
```tsx
async function handleStartReplay(runId: number) {
  const { replay_id } = await createSession.mutateAsync(runId)
  setReplayActive(true, replay_id)
  setReplayOpen(true)
}
```

4. "Replay" button — add next to the existing results action area (e.g., alongside delete button on a completed backtest run):
```tsx
{bt.status === 'completed' && (
  <button
    onClick={() => handleStartReplay(bt.id)}
    disabled={createSession.isPending}
    className="flex items-center gap-1.5 px-3 py-1.5 text-xs rounded-md border border-gray-700 hover:bg-gray-800 text-gray-300 hover:text-white transition-colors"
  >
    <Play className="h-3 w-3" />
    Replay
  </button>
)}
```

5. "Bookmarks" tab — add a fourth tab alongside Overview, Trades, Chart:

In the tab list:
```tsx
<button
  onClick={() => setActiveTab('bookmarks')}
  className={tabClass('bookmarks')}
>
  Bookmarks
</button>
```

Bookmarks tab content (using `useReplayBookmarks`):
```tsx
{activeTab === 'bookmarks' && selectedRunId && (
  <BookmarksTab runId={selectedRunId} />
)}
```

Create a `BookmarksTab` component inline or in a separate file:
```tsx
function BookmarksTab({ runId }: { runId: number }) {
  const { data: bookmarks = [], isLoading } = useReplayBookmarks(runId)
  const deleteM = useDeleteBookmark(runId)

  if (isLoading) return <div className="p-6 text-sm text-gray-500">Loading bookmarks…</div>
  if (bookmarks.length === 0)
    return (
      <div className="p-6 text-sm text-gray-500">
        No bookmarks yet. Start a replay and click "Save Bookmark" to capture moments.
      </div>
    )

  return (
    <div className="divide-y divide-gray-800">
      {bookmarks.map((bm) => (
        <div key={bm.id} className="p-4 flex gap-4">
          {bm.chart_snapshot && (
            <img
              src={bm.chart_snapshot}
              alt="chart snapshot"
              className="w-32 h-20 object-cover rounded border border-gray-700 shrink-0"
            />
          )}
          <div className="flex-1 min-w-0">
            <div className="flex items-start justify-between gap-2">
              <div className="text-sm font-medium text-white truncate">{bm.label || `Candle ${bm.candle_index}`}</div>
              <button
                onClick={() => deleteM.mutate(bm.id)}
                className="text-gray-600 hover:text-red-400 text-xs shrink-0"
              >
                Delete
              </button>
            </div>
            <div className="text-xs text-gray-500 mt-0.5">Candle {bm.candle_index}</div>
            {bm.note && <p className="text-xs text-gray-400 mt-1 line-clamp-2">{bm.note}</p>}
            <div className="text-xs text-gray-600 mt-1">{new Date(bm.created_at).toLocaleString()}</div>
          </div>
        </div>
      ))}
    </div>
  )
}
```

6. Mount overlay and bookmark modal at the bottom of the page JSX:
```tsx
{replayOpen && activeBacktest && (
  <ReplayOverlay
    runId={activeBacktest.id}
    strategyName={activeBacktest.strategy_name}
    symbol={activeBacktest.symbol}
    onClose={() => setReplayOpen(false)}
    onSave={() => setBookmarkModalOpen(true)}
  />
)}

{bookmarkModalOpen && activeBacktest && (
  <BookmarkModal
    runId={activeBacktest.id}
    onClose={() => setBookmarkModalOpen(false)}
  />
)}
```

---

**Step 4: Verify TypeScript compiles**

```bash
cd frontend && npx tsc --noEmit 2>&1
```

Expected: no errors.

---

**Step 5: Run all backend tests**

```bash
cd backend && go test ./... 2>&1
```

Expected: all PASS.

---

**Step 6: Final commit**

```bash
git add frontend/src/components/replay/BookmarkModal.tsx \
        frontend/src/hooks/useReplay.ts \
        frontend/src/pages/Backtest.tsx
git commit -m "feat(phase4): add BookmarkModal, Bookmarks tab, and wire Replay button into Backtest page"
```

---

## Final Verification

After all tasks complete, run the full test suite:

```bash
# Backend
cd backend && go test ./... -v 2>&1 | grep -E "^(ok|FAIL|---)"

# Frontend type check
cd frontend && npx tsc --noEmit
```

Start the stack and manually test the full flow:
```bash
make up
# 1. Run a backtest to completion
# 2. Click "Replay" — overlay opens, chart is blank
# 3. Click Play — candles stream in one by one
# 4. Pause, step forward, seek to middle
# 5. Change speed to 5x
# 6. Click "Save Bookmark" — BookmarkModal opens, fill in label + note, save
# 7. Close replay, open Bookmarks tab — bookmark appears with screenshot
```

---

## Summary of New Files

| File | Description |
|---|---|
| `backend/internal/replay/session.go` | Session state + pure logic (Step, Seek, SetSpeed, Run) |
| `backend/internal/replay/session_test.go` | Session unit tests |
| `backend/internal/replay/manager.go` | sync.Map session registry |
| `backend/internal/replay/manager_test.go` | Manager unit tests |
| `backend/internal/api/replay.go` | HTTP + WS handlers, bookmark CRUD |
| `backend/internal/api/replay_test.go` | Handler unit tests |
| `frontend/src/api/replay.ts` | API client functions |
| `frontend/src/hooks/useReplayWS.ts` | WebSocket hook |
| `frontend/src/hooks/useReplay.ts` | React Query hooks |
| `frontend/src/components/replay/ReplayOverlay.tsx` | Full-screen overlay |
| `frontend/src/components/replay/ReplayChart.tsx` | Candlestick chart (append-only) |
| `frontend/src/components/replay/ReplayControlBar.tsx` | Transport + speed controls |
| `frontend/src/components/replay/EquityMiniChart.tsx` | Mini equity curve (Recharts) |
| `frontend/src/components/replay/SignalToast.tsx` | Trade signal toasts |
| `frontend/src/components/replay/BookmarkModal.tsx` | Bookmark save modal |
