package analytics

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/models"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.Backtest{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestCompareRuns_Success(t *testing.T) {
	db := setupTestDB(t)
	db.Create(&models.Backtest{Name: "Run1", StrategyName: "ema", Symbol: "BTCUSDT", Timeframe: "1h", Status: models.BacktestStatusCompleted})
	db.Create(&models.Backtest{Name: "Run2", StrategyName: "rsi", Symbol: "ETHUSDT", Timeframe: "4h", Status: models.BacktestStatusCompleted})

	result, err := CompareRuns(context.Background(), db, []int64{1, 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Runs) != 2 {
		t.Errorf("expected 2 runs, got %d", len(result.Runs))
	}
}

func TestCompareRuns_MaxFiveRuns(t *testing.T) {
	db := setupTestDB(t)
	_, err := CompareRuns(context.Background(), db, []int64{1, 2, 3, 4, 5, 6})
	if err == nil {
		t.Fatal("expected error for > 5 runs")
	}
}

func TestCompareRuns_EmptyIDs(t *testing.T) {
	db := setupTestDB(t)
	_, err := CompareRuns(context.Background(), db, []int64{})
	if err == nil {
		t.Fatal("expected error for empty IDs")
	}
}
