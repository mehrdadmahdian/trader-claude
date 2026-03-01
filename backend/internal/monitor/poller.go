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
	// SignalChannel is the Redis pubsub channel for monitor signal events.
	SignalChannel = "monitor:signals"
	warmupCandles = 200
)

// SignalEvent is the payload published to Redis and forwarded over WebSocket.
type SignalEvent struct {
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
		return
	}
	if len(candles) == 0 {
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
		// Only collect signals from candles at or newer than the last poll boundary.
		if mon.LastPolledAt == nil || !c.Timestamp.Before(*mon.LastPolledAt) {
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
	// Save signal to DB and update monitor's last signal fields in a single transaction.
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
	now := time.Now()
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&ms).Error; err != nil {
			return err
		}
		return tx.Model(&models.Monitor{}).Where("id = ?", mon.ID).Updates(map[string]interface{}{
			"last_signal_at":    now,
			"last_signal_dir":   sig.Direction,
			"last_signal_price": sig.Price,
		}).Error
	}); err != nil {
		log.Printf("[monitor %d] failed to save signal: %v", mon.ID, err)
		return
	}

	// Paper trade execution
	if mon.Mode == models.MonitorModePaper && mon.PaperPortfolioID != nil {
		executePaperTrade(ctx, db, mon, sig)
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
	evt := SignalEvent{
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
		if pubErr := rdb.Publish(ctx, SignalChannel, string(b)).Err(); pubErr != nil {
			log.Printf("[monitor %d] failed to publish signal to Redis: %v", mon.ID, pubErr)
		}
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

// executePaperTrade records a paper trade transaction for the portfolio linked to the monitor.
func executePaperTrade(ctx context.Context, db *gorm.DB, mon models.Monitor, sig *registry.Signal) {
	// Load portfolio
	var portfolio models.Portfolio
	if err := db.WithContext(ctx).First(&portfolio, *mon.PaperPortfolioID).Error; err != nil {
		log.Printf("[monitor %d] paper trade: portfolio not found: %v", mon.ID, err)
		return
	}

	// Determine transaction type based on signal direction
	var txnType models.TransactionType
	switch sig.Direction {
	case "long":
		txnType = models.TransactionTypeBuy
	case "short":
		txnType = models.TransactionTypeSell
	default:
		log.Printf("[monitor %d] paper trade: unknown direction %q, skipping", mon.ID, sig.Direction)
		return
	}

	qty := paperTradeQuantity(portfolio.CurrentCash, sig.Price)
	if qty <= 0 {
		log.Printf("[monitor %d] paper trade: qty=0, skipping", mon.ID)
		return
	}

	// Create transaction
	txn := models.Transaction{
		PortfolioID: *mon.PaperPortfolioID,
		Symbol:      mon.Symbol,
		Type:        txnType,
		Quantity:    qty,
		Price:       sig.Price,
		ExecutedAt:  time.Now().UTC(),
	}
	if err := db.WithContext(ctx).Create(&txn).Error; err != nil {
		log.Printf("[monitor %d] paper trade: create txn failed: %v", mon.ID, err)
		return
	}

	// Deduct cost from portfolio cash for buy trades so subsequent signals reflect real balance
	if txnType == models.TransactionTypeBuy {
		cost := qty * sig.Price
		if err := db.WithContext(ctx).Model(&portfolio).Update("current_cash", portfolio.CurrentCash-cost).Error; err != nil {
			log.Printf("[monitor %d] paper trade: update cash failed: %v", mon.ID, err)
		}
	}

	log.Printf("[monitor %d] paper trade: %s %s qty=%.6f @ $%.4f", mon.ID, txnType, mon.Symbol, qty, sig.Price)
}

// paperTradeQuantity allocates 10% of available cash per trade at the given price.
func paperTradeQuantity(cash, price float64) float64 {
	if price <= 0 {
		return 0
	}
	return (cash * 0.10) / price
}
