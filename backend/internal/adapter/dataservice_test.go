package adapter

import (
	"testing"
	"time"

	"github.com/trader-claude/backend/internal/models"
)

// --- timeframeDuration ---

func TestTimeframeDuration(t *testing.T) {
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
		{"invalid", 0},
	}

	for _, tc := range cases {
		t.Run(tc.tf, func(t *testing.T) {
			got := timeframeDuration(tc.tf)
			if got != tc.want {
				t.Errorf("timeframeDuration(%q) = %v, want %v", tc.tf, got, tc.want)
			}
		})
	}
}

// --- findGaps ---

func makeCandle(ts time.Time) models.Candle {
	return models.Candle{Timestamp: ts}
}

func t0(offsetMin int) time.Time {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	return base.Add(time.Duration(offsetMin) * time.Minute)
}

func TestFindGaps_noCandles(t *testing.T) {
	from := t0(0)
	to := t0(60)
	gaps := findGaps(nil, from, to, time.Minute)
	if len(gaps) != 1 {
		t.Fatalf("expected 1 gap for empty DB, got %d", len(gaps))
	}
	if !gaps[0].from.Equal(from) || !gaps[0].to.Equal(to) {
		t.Errorf("gap should cover full range [%v, %v], got [%v, %v]", from, to, gaps[0].from, gaps[0].to)
	}
}

func TestFindGaps_fullyCovered(t *testing.T) {
	// 5 consecutive 1-minute candles → no gaps
	candles := []models.Candle{
		makeCandle(t0(0)),
		makeCandle(t0(1)),
		makeCandle(t0(2)),
		makeCandle(t0(3)),
		makeCandle(t0(4)),
	}
	gaps := findGaps(candles, t0(0), t0(4), time.Minute)
	if len(gaps) != 0 {
		t.Errorf("expected no gaps for full coverage, got %d", len(gaps))
	}
}

func TestFindGaps_gapBefore(t *testing.T) {
	// Requested from t0(0), first candle at t0(10) → 10-min gap at start
	candles := []models.Candle{
		makeCandle(t0(10)),
		makeCandle(t0(11)),
	}
	gaps := findGaps(candles, t0(0), t0(11), time.Minute)
	if len(gaps) != 1 {
		t.Fatalf("expected 1 gap before first candle, got %d", len(gaps))
	}
	if !gaps[0].from.Equal(t0(0)) {
		t.Errorf("gap.from: want %v, got %v", t0(0), gaps[0].from)
	}
	if !gaps[0].to.Equal(t0(9)) { // one step before t0(10)
		t.Errorf("gap.to: want %v, got %v", t0(9), gaps[0].to)
	}
}

func TestFindGaps_gapAfter(t *testing.T) {
	// Requested to t0(20), last candle at t0(10) → gap after
	candles := []models.Candle{
		makeCandle(t0(0)),
		makeCandle(t0(1)),
		makeCandle(t0(10)),
	}
	gaps := findGaps(candles, t0(0), t0(20), time.Minute)
	// There's a gap between t0(1) and t0(10), and after t0(10)
	if len(gaps) < 1 {
		t.Fatalf("expected at least 1 gap, got %d", len(gaps))
	}
	// Last gap should cover after t0(10)
	last := gaps[len(gaps)-1]
	if !last.from.Equal(t0(11)) {
		t.Errorf("last gap.from: want %v, got %v", t0(11), last.from)
	}
	if !last.to.Equal(t0(20)) {
		t.Errorf("last gap.to: want %v, got %v", t0(20), last.to)
	}
}

func TestFindGaps_gapInMiddle(t *testing.T) {
	// Candles at 0,1,2 then jump to 10,11 → gap between 2 and 10
	candles := []models.Candle{
		makeCandle(t0(0)),
		makeCandle(t0(1)),
		makeCandle(t0(2)),
		makeCandle(t0(10)),
		makeCandle(t0(11)),
	}
	gaps := findGaps(candles, t0(0), t0(11), time.Minute)
	// Should detect one middle gap
	middleGaps := 0
	for _, g := range gaps {
		if g.from.Equal(t0(3)) && g.to.Equal(t0(9)) {
			middleGaps++
		}
	}
	if middleGaps != 1 {
		t.Errorf("expected middle gap [t0(3), t0(9)], gaps=%v", gaps)
	}
}

func TestFindGaps_multipleGaps(t *testing.T) {
	// Candles at 0, 5, 10 (with gaps between each on 1m timeframe)
	candles := []models.Candle{
		makeCandle(t0(0)),
		makeCandle(t0(5)),
		makeCandle(t0(10)),
	}
	gaps := findGaps(candles, t0(0), t0(10), time.Minute)
	// Should find gaps: [t0(1), t0(4)] and [t0(6), t0(9)]
	if len(gaps) != 2 {
		t.Fatalf("expected 2 gaps, got %d: %v", len(gaps), gaps)
	}
}

// --- splitKey ---

func TestSplitKey(t *testing.T) {
	parts := splitKey("binance:BTC/USDT:crypto:1h", 4)
	if parts == nil {
		t.Fatal("expected non-nil parts")
	}
	if parts[0] != "binance" {
		t.Errorf("parts[0] want 'binance', got %q", parts[0])
	}
	if parts[1] != "BTC/USDT" {
		t.Errorf("parts[1] want 'BTC/USDT', got %q", parts[1])
	}
	if parts[2] != "crypto" {
		t.Errorf("parts[2] want 'crypto', got %q", parts[2])
	}
	if parts[3] != "1h" {
		t.Errorf("parts[3] want '1h', got %q", parts[3])
	}
}

func TestSplitKey_wrongCount(t *testing.T) {
	parts := splitKey("a:b:c", 4) // only 3 parts
	if parts != nil {
		t.Errorf("expected nil for wrong part count, got %v", parts)
	}
}

// --- mergeAndSort ---

func TestMergeAndSort(t *testing.T) {
	a := []models.Candle{makeCandle(t0(0)), makeCandle(t0(2))}
	b := []models.Candle{makeCandle(t0(1)), makeCandle(t0(3))}

	merged := mergeAndSort(a, b)
	if len(merged) != 4 {
		t.Fatalf("expected 4 candles, got %d", len(merged))
	}
	for i := 0; i < len(merged)-1; i++ {
		if !merged[i].Timestamp.Before(merged[i+1].Timestamp) {
			t.Errorf("candles not sorted: index %d (%v) >= index %d (%v)",
				i, merged[i].Timestamp, i+1, merged[i+1].Timestamp)
		}
	}
}

func TestMergeAndSort_deduplication(t *testing.T) {
	ts := t0(0)
	a := []models.Candle{makeCandle(ts)}
	b := []models.Candle{makeCandle(ts)} // duplicate

	merged := mergeAndSort(a, b)
	if len(merged) != 1 {
		t.Errorf("expected 1 candle after dedup, got %d", len(merged))
	}
}
