package news

import "testing"

func TestExtractSymbols_Crypto(t *testing.T) {
	text := "Bitcoin and Ethereum continue to rally as BTC breaks 100k"
	syms := ExtractSymbols(text)
	want := map[string]bool{"BTCUSDT": true, "ETHUSDT": true}
	for _, s := range syms {
		delete(want, s)
	}
	if len(want) > 0 {
		t.Errorf("missing symbols: %v (got %v)", want, syms)
	}
}

func TestExtractSymbols_Stock(t *testing.T) {
	syms := ExtractSymbols("Apple reports record earnings beating Microsoft")
	found := map[string]bool{}
	for _, s := range syms {
		found[s] = true
	}
	if !found["AAPL"] {
		t.Error("expected AAPL")
	}
	if !found["MSFT"] {
		t.Error("expected MSFT")
	}
}

func TestExtractSymbols_NoMatch(t *testing.T) {
	syms := ExtractSymbols("The weather is nice today")
	if len(syms) != 0 {
		t.Errorf("expected no symbols, got %v", syms)
	}
}

func TestExtractSymbols_Dedup(t *testing.T) {
	syms := ExtractSymbols("Bitcoin (BTC) is the largest cryptocurrency")
	count := 0
	for _, s := range syms {
		if s == "BTCUSDT" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected BTCUSDT exactly once, got %d", count)
	}
}
