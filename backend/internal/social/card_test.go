package social

import (
	"bytes"
	"image/png"
	"testing"
)

func TestGenerateBacktestCard_DarkTheme(t *testing.T) {
	data, err := GenerateBacktestCard(BacktestCardOpts{
		Theme:        "dark",
		StrategyName: "EMA Crossover",
		Symbol:       "BTCUSDT",
		Timeframe:    "1h",
		DateRange:    "Jan 1 – Dec 31, 2024",
		TotalReturn:  0.342,
		SharpeRatio:  1.87,
		MaxDrawdown:  -0.142,
		WinRate:      0.58,
		EquityCurve:  []float64{10000, 10200, 10100, 10500, 10800, 11200, 11000, 11500, 12000, 13420},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty PNG data")
	}

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("invalid PNG: %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != 1200 || bounds.Dy() != 630 {
		t.Errorf("expected 1200×630, got %d×%d", bounds.Dx(), bounds.Dy())
	}
}

func TestGenerateBacktestCard_LightTheme(t *testing.T) {
	data, err := GenerateBacktestCard(BacktestCardOpts{
		Theme:        "light",
		StrategyName: "RSI",
		Symbol:       "ETHUSDT",
		Timeframe:    "4h",
		DateRange:    "Jun 1 – Dec 31, 2025",
		TotalReturn:  -0.05,
		SharpeRatio:  0.45,
		MaxDrawdown:  -0.22,
		WinRate:      0.42,
		EquityCurve:  []float64{10000, 9800, 9600, 9700, 9500},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty PNG data")
	}
}

func TestGenerateSignalCard_Long(t *testing.T) {
	data, err := GenerateSignalCard(SignalCardOpts{
		Theme:        "dark",
		Symbol:       "BTCUSDT",
		Direction:    "LONG",
		Price:        82150.00,
		StrategyName: "EMA Crossover",
		Strength:     0.87,
		Timestamp:    "Feb 27, 2026 14:30 UTC",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("invalid PNG: %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != 1200 || bounds.Dy() != 630 {
		t.Errorf("expected 1200×630, got %d×%d", bounds.Dx(), bounds.Dy())
	}
}

func TestGenerateSignalCard_Short(t *testing.T) {
	data, err := GenerateSignalCard(SignalCardOpts{
		Theme:        "dark",
		Symbol:       "ETHUSDT",
		Direction:    "SHORT",
		Price:        2100.50,
		StrategyName: "RSI",
		Strength:     0.65,
		Timestamp:    "Feb 27, 2026 16:00 UTC",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty PNG data")
	}
}
