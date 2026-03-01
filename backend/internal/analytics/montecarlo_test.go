package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/trader-claude/backend/internal/models"
)

func TestRunMonteCarlo_Success(t *testing.T) {
	db := setupTestDB(t)
	db.AutoMigrate(&models.Trade{})
	bt := models.Backtest{
		Name:         "BT",
		StrategyName: "ema",
		Symbol:       "BTCUSDT",
		Market:       "binance",
		Timeframe:    "1h",
		Status:       models.BacktestStatusCompleted,
	}
	db.Create(&bt)

	// Create trades with PnLPercent values
	now := time.Now()
	pnl1 := 1.5
	pnl2 := -0.5
	pnl3 := 2.0
	pnl4 := 0.8

	db.Create(&models.Trade{
		BacktestID: &bt.ID,
		Symbol:     "BTCUSDT",
		Market:     "binance",
		Direction:  models.TradeDirectionLong,
		EntryPrice: 40000.0,
		ExitPrice:  &[]float64{40600.0}[0],
		Quantity:   1.0,
		EntryTime:  now,
		ExitTime:   &[]time.Time{now.Add(1 * time.Hour)}[0],
		PnLPercent: &pnl1,
	})
	db.Create(&models.Trade{
		BacktestID: &bt.ID,
		Symbol:     "BTCUSDT",
		Market:     "binance",
		Direction:  models.TradeDirectionShort,
		EntryPrice: 40600.0,
		ExitPrice:  &[]float64{40400.0}[0],
		Quantity:   1.0,
		EntryTime:  now.Add(1 * time.Hour),
		ExitTime:   &[]time.Time{now.Add(2 * time.Hour)}[0],
		PnLPercent: &pnl2,
	})
	db.Create(&models.Trade{
		BacktestID: &bt.ID,
		Symbol:     "BTCUSDT",
		Market:     "binance",
		Direction:  models.TradeDirectionLong,
		EntryPrice: 40400.0,
		ExitPrice:  &[]float64{41216.0}[0],
		Quantity:   1.0,
		EntryTime:  now.Add(2 * time.Hour),
		ExitTime:   &[]time.Time{now.Add(3 * time.Hour)}[0],
		PnLPercent: &pnl3,
	})
	db.Create(&models.Trade{
		BacktestID: &bt.ID,
		Symbol:     "BTCUSDT",
		Market:     "binance",
		Direction:  models.TradeDirectionLong,
		EntryPrice: 41216.0,
		ExitPrice:  &[]float64{41545.0}[0],
		Quantity:   1.0,
		EntryTime:  now.Add(3 * time.Hour),
		ExitTime:   &[]time.Time{now.Add(4 * time.Hour)}[0],
		PnLPercent: &pnl4,
	})

	result, err := RunMonteCarlo(context.Background(), db, bt.ID, 100, 0.5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NumSimulations != 100 {
		t.Errorf("expected 100 sims, got %d", result.NumSimulations)
	}
	if len(result.Percentiles) != 5 {
		t.Errorf("expected 5 percentile curves, got %d", len(result.Percentiles))
	}
	if result.ProbabilityRuin < 0 || result.ProbabilityRuin > 1 {
		t.Errorf("ruin probability out of bounds: %f", result.ProbabilityRuin)
	}
	// With positive expected return (avg of 1.5, -0.5, 2.0, 0.8), final returns should mostly be positive
	if result.MaxReturn <= result.MedianReturn {
		t.Errorf("max return should be greater than median: max=%f, median=%f", result.MaxReturn, result.MedianReturn)
	}
}

func TestRunMonteCarlo_TooFewTrades(t *testing.T) {
	db := setupTestDB(t)
	db.AutoMigrate(&models.Trade{})
	bt := models.Backtest{
		Name:         "BT",
		StrategyName: "ema",
		Symbol:       "BTCUSDT",
		Market:       "binance",
		Timeframe:    "1h",
		Status:       models.BacktestStatusCompleted,
	}
	db.Create(&bt)

	// Create only 1 trade — should error
	now := time.Now()
	pnl := 1.5
	db.Create(&models.Trade{
		BacktestID: &bt.ID,
		Symbol:     "BTCUSDT",
		Market:     "binance",
		Direction:  models.TradeDirectionLong,
		EntryPrice: 40000.0,
		ExitPrice:  &[]float64{40600.0}[0],
		Quantity:   1.0,
		EntryTime:  now,
		ExitTime:   &[]time.Time{now.Add(1 * time.Hour)}[0],
		PnLPercent: &pnl,
	})

	_, err := RunMonteCarlo(context.Background(), db, bt.ID, 100, 0.5)
	if err == nil {
		t.Fatal("expected error for too few trades")
	}
}

func TestRunMonteCarlo_NoTradesWithPnL(t *testing.T) {
	db := setupTestDB(t)
	db.AutoMigrate(&models.Trade{})
	bt := models.Backtest{
		Name:         "BT",
		StrategyName: "ema",
		Symbol:       "BTCUSDT",
		Market:       "binance",
		Timeframe:    "1h",
		Status:       models.BacktestStatusCompleted,
	}
	db.Create(&bt)

	// Create trades without PnLPercent — should error
	now := time.Now()
	db.Create(&models.Trade{
		BacktestID: &bt.ID,
		Symbol:     "BTCUSDT",
		Market:     "binance",
		Direction:  models.TradeDirectionLong,
		EntryPrice: 40000.0,
		ExitPrice:  &[]float64{40600.0}[0],
		Quantity:   1.0,
		EntryTime:  now,
		ExitTime:   &[]time.Time{now.Add(1 * time.Hour)}[0],
		PnLPercent: nil,
	})
	db.Create(&models.Trade{
		BacktestID: &bt.ID,
		Symbol:     "BTCUSDT",
		Market:     "binance",
		Direction:  models.TradeDirectionLong,
		EntryPrice: 40600.0,
		ExitPrice:  &[]float64{41200.0}[0],
		Quantity:   1.0,
		EntryTime:  now.Add(1 * time.Hour),
		ExitTime:   &[]time.Time{now.Add(2 * time.Hour)}[0],
		PnLPercent: nil,
	})

	_, err := RunMonteCarlo(context.Background(), db, bt.ID, 100, 0.5)
	if err == nil {
		t.Fatal("expected error for no trades with PnL data")
	}
}

func TestRunMonteCarlo_DefaultsValues(t *testing.T) {
	db := setupTestDB(t)
	db.AutoMigrate(&models.Trade{})
	bt := models.Backtest{
		Name:         "BT",
		StrategyName: "ema",
		Symbol:       "BTCUSDT",
		Market:       "binance",
		Timeframe:    "1h",
		Status:       models.BacktestStatusCompleted,
	}
	db.Create(&bt)

	// Create trades with PnLPercent values
	now := time.Now()
	pnl1 := 1.0
	pnl2 := 1.5

	db.Create(&models.Trade{
		BacktestID: &bt.ID,
		Symbol:     "BTCUSDT",
		Market:     "binance",
		Direction:  models.TradeDirectionLong,
		EntryPrice: 40000.0,
		ExitPrice:  &[]float64{40400.0}[0],
		Quantity:   1.0,
		EntryTime:  now,
		ExitTime:   &[]time.Time{now.Add(1 * time.Hour)}[0],
		PnLPercent: &pnl1,
	})
	db.Create(&models.Trade{
		BacktestID: &bt.ID,
		Symbol:     "BTCUSDT",
		Market:     "binance",
		Direction:  models.TradeDirectionLong,
		EntryPrice: 40400.0,
		ExitPrice:  &[]float64{41006.0}[0],
		Quantity:   1.0,
		EntryTime:  now.Add(1 * time.Hour),
		ExitTime:   &[]time.Time{now.Add(2 * time.Hour)}[0],
		PnLPercent: &pnl2,
	})

	// Call with 0 for numSims and ruinThreshold to test defaults
	result, err := RunMonteCarlo(context.Background(), db, bt.ID, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NumSimulations != 1000 {
		t.Errorf("expected default 1000 sims, got %d", result.NumSimulations)
	}
}
