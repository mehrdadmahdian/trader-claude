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
	if msgs[0].Type != MsgCandle {
		t.Errorf("expected first message type=%s, got %s", MsgCandle, msgs[0].Type)
	}
}

func TestStep_AtLastCandle_ReturnsComplete(t *testing.T) {
	candles := makeCandles(2)
	s := NewSession("id-3", 1, candles, makeEquity(candles), nil)
	s.CurrentIdx = 1

	_, err := s.Step()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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

	s.Seek(999)
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
