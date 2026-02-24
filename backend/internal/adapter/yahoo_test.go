package adapter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestYahooAdapter(serverURL string) *YahooAdapter {
	return &YahooAdapter{
		client:  &http.Client{Timeout: 5 * time.Second},
		baseURL: serverURL,
	}
}

func TestYahooFetchCandles_success(t *testing.T) {
	const body = `{
		"chart": {
			"result": [{
				"timestamp": [1700000000, 1700003600],
				"indicators": {
					"quote": [{
						"open":   [150.0, 151.0],
						"high":   [155.0, 156.0],
						"low":    [149.0, 150.0],
						"close":  [153.0, 154.0],
						"volume": [1000000.0, 1100000.0]
					}]
				}
			}],
			"error": null
		}
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(body))
	}))
	defer srv.Close()

	a := newTestYahooAdapter(srv.URL)
	from := time.Unix(1699999000, 0)
	to := time.Unix(1700010000, 0)

	candles, err := a.FetchCandles(context.Background(), "AAPL", "stock", "1d", from, to)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candles) != 2 {
		t.Fatalf("expected 2 candles, got %d", len(candles))
	}

	c := candles[0]
	if c.Symbol != "AAPL" {
		t.Errorf("expected symbol AAPL, got %q", c.Symbol)
	}
	if c.Market != "stock" {
		t.Errorf("expected market stock, got %q", c.Market)
	}
	if c.Timeframe != "1d" {
		t.Errorf("expected timeframe 1d, got %q", c.Timeframe)
	}
	if c.Open != 150.0 {
		t.Errorf("expected Open 150.0, got %f", c.Open)
	}
	if c.High != 155.0 {
		t.Errorf("expected High 155.0, got %f", c.High)
	}
	if c.Low != 149.0 {
		t.Errorf("expected Low 149.0, got %f", c.Low)
	}
	if c.Close != 153.0 {
		t.Errorf("expected Close 153.0, got %f", c.Close)
	}
	if c.Volume != 1000000.0 {
		t.Errorf("expected Volume 1000000.0, got %f", c.Volume)
	}
	if c.Timestamp != time.Unix(1700000000, 0).UTC() {
		t.Errorf("unexpected timestamp: %v", c.Timestamp)
	}
}

func TestYahooFetchCandles_nullFiltering(t *testing.T) {
	const body = `{
		"chart": {
			"result": [{
				"timestamp": [1700000000, 1700003600, 1700007200],
				"indicators": {
					"quote": [{
						"open":   [150.0, null, 152.0],
						"high":   [155.0, null, 157.0],
						"low":    [149.0, null, 151.0],
						"close":  [153.0, null, 154.0],
						"volume": [1000000.0, null, 900000.0]
					}]
				}
			}],
			"error": null
		}
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(body))
	}))
	defer srv.Close()

	a := newTestYahooAdapter(srv.URL)
	from := time.Unix(1699999000, 0)
	to := time.Unix(1700010000, 0)

	candles, err := a.FetchCandles(context.Background(), "MSFT", "stock", "1h", from, to)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candles) != 2 {
		t.Fatalf("expected 2 candles after null filtering, got %d", len(candles))
	}
	if candles[0].Open != 150.0 {
		t.Errorf("first candle: expected Open 150.0, got %f", candles[0].Open)
	}
	if candles[1].Open != 152.0 {
		t.Errorf("second candle: expected Open 152.0, got %f", candles[1].Open)
	}
}

func TestYahooFetchSymbols_stock(t *testing.T) {
	a := NewYahooAdapter()
	syms, err := a.FetchSymbols(context.Background(), "stock")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(syms) != 50 {
		t.Errorf("expected 50 stock symbols, got %d", len(syms))
	}
	found := make(map[string]bool)
	for _, s := range syms {
		found[s.ID] = true
		if s.Market != "stock" {
			t.Errorf("symbol %q: expected market stock, got %q", s.ID, s.Market)
		}
		if !s.Active {
			t.Errorf("symbol %q should be active", s.ID)
		}
	}
	for _, ticker := range []string{"AAPL", "MSFT", "GOOGL", "NVDA", "BRK-B"} {
		if !found[ticker] {
			t.Errorf("expected ticker %q in stock list", ticker)
		}
	}
}

func TestYahooFetchSymbols_etf(t *testing.T) {
	a := NewYahooAdapter()
	syms, err := a.FetchSymbols(context.Background(), "etf")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(syms) != 30 {
		t.Errorf("expected 30 ETF symbols, got %d", len(syms))
	}
	found := make(map[string]bool)
	for _, s := range syms {
		found[s.ID] = true
	}
	for _, ticker := range []string{"SPY", "QQQ", "IWM", "GLD", "ARKK"} {
		if !found[ticker] {
			t.Errorf("expected ticker %q in ETF list", ticker)
		}
	}
}

func TestYahooFetchSymbols_forex(t *testing.T) {
	a := NewYahooAdapter()
	syms, err := a.FetchSymbols(context.Background(), "forex")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(syms) != 10 {
		t.Errorf("expected 10 forex symbols, got %d", len(syms))
	}
	found := make(map[string]bool)
	for _, s := range syms {
		found[s.ID] = true
	}
	for _, ticker := range []string{"EURUSD=X", "GBPUSD=X", "USDJPY=X"} {
		if !found[ticker] {
			t.Errorf("expected ticker %q in forex list", ticker)
		}
	}
}

func TestYahooSubscribeTicks_returnsError(t *testing.T) {
	a := NewYahooAdapter()
	ch, err := a.SubscribeTicks(context.Background(), []string{"AAPL"}, "stock")
	if err == nil {
		t.Fatal("expected error from SubscribeTicks, got nil")
	}
	if ch != nil {
		t.Error("expected nil channel from SubscribeTicks")
	}
	const want = "yahoo adapter does not support streaming"
	if err.Error() != want {
		t.Errorf("expected error %q, got %q", want, err.Error())
	}
}

func TestYahooIsHealthy_success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := newTestYahooAdapter(srv.URL)
	if !a.IsHealthy(context.Background()) {
		t.Error("expected IsHealthy to return true for 200 response")
	}
}

func TestYahooIsHealthy_failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	a := newTestYahooAdapter(srv.URL)
	if a.IsHealthy(context.Background()) {
		t.Error("expected IsHealthy to return false for non-200 response")
	}
}
