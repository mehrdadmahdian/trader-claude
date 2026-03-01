package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/trader-claude/backend/internal/models"
)

func TestWalkForward_WindowSplitting(t *testing.T) {
	db := setupTestDB(t)
	db.AutoMigrate(&models.Candle{})

	now := time.Now().UTC().Truncate(time.Hour)
	startDate := now.Add(-300 * time.Hour) // 300 hours of data
	endDate := now

	bt := models.Backtest{
		Name: "WalkForwardBT", StrategyName: "ema_crossover",
		Symbol: "BTCUSDT", Market: "crypto", Timeframe: "1h",
		StartDate: startDate, EndDate: endDate,
		Status: models.BacktestStatusCompleted,
	}
	db.Create(&bt)

	// Seed synthetic candles across full range
	price := 50000.0
	for i := 0; i < 300; i++ {
		db.Create(&models.Candle{
			Symbol: "BTCUSDT", Market: "crypto", Timeframe: "1h",
			Timestamp: startDate.Add(time.Duration(i) * time.Hour),
			Open: price, High: price * 1.001, Low: price * 0.999, Close: price, Volume: 100,
		})
		price *= 1.001
	}

	// Run walk-forward with 3 windows
	result, err := RunWalkForward(context.Background(), db, bt.ID, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Windows) != 3 {
		t.Errorf("expected 3 windows, got %d", len(result.Windows))
	}

	// Verify windows don't overlap and cover full range
	for i := 0; i < len(result.Windows); i++ {
		w := result.Windows[i]
		if w.WindowIndex != i {
			t.Errorf("window %d: expected index %d, got %d", i, i, w.WindowIndex)
		}

		// Verify train_start < train_end == test_start < test_end
		if !w.TrainStart.Before(w.TrainEnd) {
			t.Errorf("window %d: train_start >= train_end", i)
		}
		if w.TrainEnd != w.TestStart {
			t.Errorf("window %d: train_end != test_start (boundary mismatch)", i)
		}
		if !w.TestStart.Before(w.TestEnd) {
			t.Errorf("window %d: test_start >= test_end", i)
		}

		// Verify 70/30 split approximately
		windowDuration := w.TestEnd.Sub(w.TrainStart)
		trainDuration := w.TrainEnd.Sub(w.TrainStart)
		trainRatio := float64(trainDuration) / float64(windowDuration)
		if trainRatio < 0.68 || trainRatio > 0.72 {
			t.Errorf("window %d: expected 70%% train ratio, got %.1f%%", i, trainRatio*100)
		}
	}

	// Verify no gaps between consecutive windows
	for i := 0; i < len(result.Windows)-1; i++ {
		if result.Windows[i].TestEnd != result.Windows[i+1].TrainStart {
			t.Errorf("gap between window %d and %d", i, i+1)
		}
	}
}

func TestWalkForward_DefaultWindows(t *testing.T) {
	db := setupTestDB(t)
	db.AutoMigrate(&models.Candle{})

	now := time.Now().UTC().Truncate(time.Hour)
	startDate := now.Add(-100 * time.Hour)
	endDate := now

	bt := models.Backtest{
		Name: "WalkForwardBT", StrategyName: "ema_crossover",
		Symbol: "BTCUSDT", Market: "crypto", Timeframe: "1h",
		StartDate: startDate, EndDate: endDate,
		Status: models.BacktestStatusCompleted,
	}
	db.Create(&bt)

	// Seed candles
	price := 50000.0
	for i := 0; i < 100; i++ {
		db.Create(&models.Candle{
			Symbol: "BTCUSDT", Market: "crypto", Timeframe: "1h",
			Timestamp: startDate.Add(time.Duration(i) * time.Hour),
			Open: price, High: price * 1.001, Low: price * 0.999, Close: price, Volume: 100,
		})
		price *= 1.001
	}

	// Run with numWindows=0, should default to 5
	result, err := RunWalkForward(context.Background(), db, bt.ID, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Windows) != 5 {
		t.Errorf("expected 5 windows (default), got %d", len(result.Windows))
	}
}

func TestWalkForward_CapsMaxWindows(t *testing.T) {
	db := setupTestDB(t)
	db.AutoMigrate(&models.Candle{})

	now := time.Now().UTC().Truncate(time.Hour)
	startDate := now.Add(-100 * time.Hour)
	endDate := now

	bt := models.Backtest{
		Name: "WalkForwardBT", StrategyName: "ema_crossover",
		Symbol: "BTCUSDT", Market: "crypto", Timeframe: "1h",
		StartDate: startDate, EndDate: endDate,
		Status: models.BacktestStatusCompleted,
	}
	db.Create(&bt)

	// Seed candles
	price := 50000.0
	for i := 0; i < 100; i++ {
		db.Create(&models.Candle{
			Symbol: "BTCUSDT", Market: "crypto", Timeframe: "1h",
			Timestamp: startDate.Add(time.Duration(i) * time.Hour),
			Open: price, High: price * 1.001, Low: price * 0.999, Close: price, Volume: 100,
		})
		price *= 1.001
	}

	// Run with numWindows=50, should cap at 20
	result, err := RunWalkForward(context.Background(), db, bt.ID, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Windows) != 20 {
		t.Errorf("expected 20 windows (capped), got %d", len(result.Windows))
	}
}
