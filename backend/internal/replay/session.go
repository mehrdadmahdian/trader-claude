package replay

import (
	"fmt"
	"sync"
	"time"

	"github.com/trader-claude/backend/internal/backtest"
	"github.com/trader-claude/backend/internal/models"
	"github.com/trader-claude/backend/internal/registry"
)

const baseInterval = 300 * time.Millisecond

const (
	StateIdle     = "idle"
	StatePlaying  = "playing"
	StatePaused   = "paused"
	StateComplete = "complete"
)

const (
	MsgCandle       = "candle"
	MsgTradeOpen    = "trade_open"
	MsgTradeClose   = "trade_close"
	MsgEquityUpdate = "equity_update"
	MsgSeekSnapshot = "seek_snapshot"
	MsgStatus       = "status"
)

type Message struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type StatusData struct {
	State string  `json:"state"`
	Index int     `json:"index"`
	Total int     `json:"total"`
	Speed float64 `json:"speed"`
}

type SeekSnapshotData struct {
	Candles []registry.Candle      `json:"candles"`
	Equity  []backtest.EquityPoint `json:"equity"`
	Trades  []models.Trade         `json:"trades"`
}

type TradeEventData struct {
	Trade models.Trade `json:"trade"`
}

type EquityUpdateData struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

type ControlMsg struct {
	Type  string  `json:"type"`
	Speed float64 `json:"speed,omitempty"`
	Index int     `json:"index,omitempty"`
}

type Session struct {
	ID          string
	RunID       int64
	State       string
	Speed       float64
	CurrentIdx  int
	ControlChan chan ControlMsg

	candles      []registry.Candle
	equity       []backtest.EquityPoint
	tradeByOpen  map[time.Time]models.Trade
	tradeByClose map[time.Time]models.Trade
	mu           sync.Mutex
}

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

func (s *Session) Interval() time.Duration {
	if s.Speed <= 0 {
		return baseInterval
	}
	return time.Duration(float64(baseInterval) / s.Speed)
}

func (s *Session) SetSpeed(speed float64) {
	if speed <= 0 {
		return
	}
	s.Speed = speed
}

func (s *Session) Step() ([]Message, error) {
	if s.CurrentIdx >= len(s.candles) {
		s.State = StateComplete
		return []Message{s.statusMsg()}, nil
	}

	candle := s.candles[s.CurrentIdx]
	msgs := make([]Message, 0, 4)

	msgs = append(msgs, Message{Type: MsgCandle, Data: candle})

	if trade, ok := s.tradeByOpen[candle.Timestamp]; ok {
		msgs = append(msgs, Message{Type: MsgTradeOpen, Data: TradeEventData{Trade: trade}})
	}

	if trade, ok := s.tradeByClose[candle.Timestamp]; ok {
		msgs = append(msgs, Message{Type: MsgTradeClose, Data: TradeEventData{Trade: trade}})
	}

	if s.CurrentIdx < len(s.equity) {
		ep := s.equity[s.CurrentIdx]
		msgs = append(msgs, Message{Type: MsgEquityUpdate, Data: EquityUpdateData{Timestamp: ep.Timestamp, Value: ep.Value}})
	}

	s.CurrentIdx++

	if s.CurrentIdx >= len(s.candles) {
		s.State = StateComplete
	}

	return msgs, nil
}

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

func (s *Session) Total() int { return len(s.candles) }

func (s *Session) Run(writeFn func(Message) error, done <-chan struct{}) error {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

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
			ticker.Reset(s.Interval())
		}
	}
}
