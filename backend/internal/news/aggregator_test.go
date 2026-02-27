package news

import (
	"testing"
)

func TestAggregatorPureFunctions(t *testing.T) {
	title := "Bitcoin surges as BTC rally continues to record highs"
	summary := "Ethereum also rising amid market recovery"

	combined := title + " " + summary

	syms := ExtractSymbols(combined)
	foundBTC, foundETH := false, false
	for _, s := range syms {
		if s == "BTCUSDT" {
			foundBTC = true
		}
		if s == "ETHUSDT" {
			foundETH = true
		}
	}
	if !foundBTC {
		t.Error("expected BTCUSDT to be tagged")
	}
	if !foundETH {
		t.Error("expected ETHUSDT to be tagged")
	}

	score := Score(combined)
	if score <= 0 {
		t.Errorf("expected positive sentiment score, got %f", score)
	}
}

func TestNewAggregator(t *testing.T) {
	agg := NewAggregator(nil, DefaultFeeds)
	if agg == nil {
		t.Fatal("NewAggregator returned nil")
	}
	if len(agg.feeds) != len(DefaultFeeds) {
		t.Errorf("expected %d feeds, got %d", len(DefaultFeeds), len(agg.feeds))
	}
}
