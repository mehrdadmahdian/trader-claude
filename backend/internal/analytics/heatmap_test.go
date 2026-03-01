package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/trader-claude/backend/internal/models"
	"github.com/trader-claude/backend/internal/registry"
	"github.com/trader-claude/backend/internal/strategy"
)

func init() {
	if !registry.Strategies().Exists("ema_crossover") {
		registry.Strategies().Register("ema_crossover", func() registry.Strategy { return &strategy.EMACrossover{} })
	}
}

func TestLinspace(t *testing.T) {
	vals := linspace(1, 5, 5)
	if len(vals) != 5 {
		t.Fatalf("expected 5 values, got %d", len(vals))
	}
	if vals[0] != 1 || vals[4] != 5 {
		t.Errorf("unexpected range: %v", vals)
	}
}

func TestExtractMinMax_Valid(t *testing.T) {
	p := &registry.ParamDefinition{Name: "test", Min: 5, Max: 50}
	min, max, err := extractMinMax(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if min != 5 || max != 50 {
		t.Errorf("unexpected min/max: %v/%v", min, max)
	}
}

func TestExtractMinMax_NoMin(t *testing.T) {
	p := &registry.ParamDefinition{Name: "test", Min: nil, Max: 50}
	_, _, err := extractMinMax(p)
	if err == nil {
		t.Fatal("expected error for nil Min")
	}
}

func TestRunHeatmap_SmallGrid(t *testing.T) {
	db := setupTestDB(t)
	db.AutoMigrate(&models.Candle{})

	now := time.Now().UTC().Truncate(time.Hour)
	bt := models.Backtest{
		Name: "HeatBT", StrategyName: "ema_crossover",
		Symbol: "BTCUSDT", Market: "crypto", Timeframe: "1h",
		StartDate: now.Add(-100 * time.Hour), EndDate: now,
		Status: models.BacktestStatusCompleted,
	}
	db.Create(&bt)

	// Seed 100 synthetic candles
	price := 50000.0
	for i := 0; i < 100; i++ {
		db.Create(&models.Candle{
			Symbol: "BTCUSDT", Market: "crypto", Timeframe: "1h",
			Timestamp: bt.StartDate.Add(time.Duration(i) * time.Hour),
			Open: price, High: price * 1.001, Low: price * 0.999, Close: price, Volume: 100,
		})
		price *= 1.001
	}

	result, err := RunHeatmap(context.Background(), db, bt.ID, "fast_period", "slow_period", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.XValues) != 2 {
		t.Errorf("expected 2 x values, got %d", len(result.XValues))
	}
	if len(result.Cells) != 2 {
		t.Errorf("expected 2 rows, got %d", len(result.Cells))
	}
	for _, row := range result.Cells {
		if len(row) != 2 {
			t.Errorf("expected 2 cells per row, got %d", len(row))
		}
	}
}
