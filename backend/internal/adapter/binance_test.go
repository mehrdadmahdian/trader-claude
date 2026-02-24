package adapter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestBinanceAdapter(serverURL string) *BinanceAdapter {
	return &BinanceAdapter{
		client:  &http.Client{Timeout: 5 * time.Second},
		baseURL: serverURL,
	}
}

// buildKlinesJSON builds a valid Binance klines JSON response.
// Each row: [openTime, open, high, low, close, volume, closeTime, ...].
func buildKlinesJSON(rows [][]interface{}) []byte {
	b, _ := json.Marshal(rows)
	return b
}

func TestBinanceFetchCandles_success(t *testing.T) {
	// Two 1-minute candles starting at epoch 0
	rows := [][]interface{}{
		{int64(0), "100.0", "105.0", "99.0", "103.0", "500.0", int64(59999), "0", 0, "0", "0", "0"},
		{int64(60000), "103.0", "108.0", "102.0", "107.0", "600.0", int64(119999), "0", 0, "0", "0", "0"},
	}
	body := buildKlinesJSON(rows)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/klines" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer srv.Close()

	a := newTestBinanceAdapter(srv.URL)
	from := time.Unix(0, 0).UTC()
	to := time.Unix(120, 0).UTC()

	candles, err := a.FetchCandles(context.Background(), "BTC/USDT", "crypto", "1m", from, to)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candles) != 2 {
		t.Fatalf("expected 2 candles, got %d", len(candles))
	}

	c := candles[0]
	if c.Symbol != "BTC/USDT" {
		t.Errorf("expected symbol BTC/USDT, got %q", c.Symbol)
	}
	if c.Market != "crypto" {
		t.Errorf("expected market crypto, got %q", c.Market)
	}
	if c.Timeframe != "1m" {
		t.Errorf("expected timeframe 1m, got %q", c.Timeframe)
	}
	if c.Open != 100.0 {
		t.Errorf("expected Open 100.0, got %f", c.Open)
	}
	if c.High != 105.0 {
		t.Errorf("expected High 105.0, got %f", c.High)
	}
	if c.Low != 99.0 {
		t.Errorf("expected Low 99.0, got %f", c.Low)
	}
	if c.Close != 103.0 {
		t.Errorf("expected Close 103.0, got %f", c.Close)
	}
	if c.Volume != 500.0 {
		t.Errorf("expected Volume 500.0, got %f", c.Volume)
	}
	if c.Timestamp != time.UnixMilli(0).UTC() {
		t.Errorf("unexpected timestamp: %v", c.Timestamp)
	}
}

func TestBinanceFetchCandles_rateLimitRetry(t *testing.T) {
	calls := 0
	rows := [][]interface{}{
		{int64(0), "50000.0", "51000.0", "49000.0", "50500.0", "10.0", int64(59999), "0", 0, "0", "0", "0"},
	}
	body := buildKlinesJSON(rows)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer srv.Close()

	a := newTestBinanceAdapter(srv.URL)
	// Shorten backoff for test speed by overriding client
	a.client = &http.Client{Timeout: 5 * time.Second}

	from := time.Unix(0, 0)
	to := time.Unix(60, 0)

	candles, err := a.FetchCandles(context.Background(), "BTC/USDT", "crypto", "1m", from, to)
	if err != nil {
		t.Fatalf("expected retry to succeed, got error: %v", err)
	}
	if len(candles) != 1 {
		t.Errorf("expected 1 candle, got %d", len(candles))
	}
	if calls < 2 {
		t.Errorf("expected at least 2 calls (1 retry), got %d", calls)
	}
}

func TestBinanceFetchCandles_timeframeMapping(t *testing.T) {
	cases := []struct {
		tf       string
		expected string
	}{
		{"1m", "1m"}, {"5m", "5m"}, {"15m", "15m"}, {"30m", "30m"},
		{"1h", "1h"}, {"4h", "4h"}, {"1d", "1d"}, {"1w", "1w"},
	}

	for _, tc := range cases {
		t.Run(tc.tf, func(t *testing.T) {
			got := ""
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.URL.Query().Get("interval")
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte("[]")) // empty candles
			}))
			defer srv.Close()

			a := newTestBinanceAdapter(srv.URL)
			_, _ = a.FetchCandles(context.Background(), "ETH/USDT", "crypto", tc.tf,
				time.Unix(0, 0), time.Unix(60, 0))

			if got != tc.expected {
				t.Errorf("timeframe %q: expected interval param %q, got %q", tc.tf, tc.expected, got)
			}
		})
	}
}

func TestBinanceFetchSymbols_success(t *testing.T) {
	exchangeInfo := map[string]interface{}{
		"symbols": []map[string]interface{}{
			{"symbol": "BTCUSDT", "status": "TRADING", "baseAsset": "BTC", "quoteAsset": "USDT"},
			{"symbol": "ETHUSDT", "status": "TRADING", "baseAsset": "ETH", "quoteAsset": "USDT"},
			{"symbol": "BNBBTC", "status": "TRADING", "baseAsset": "BNB", "quoteAsset": "BTC"},  // not USDT
			{"symbol": "SOLUSDT", "status": "BREAK", "baseAsset": "SOL", "quoteAsset": "USDT"},  // not TRADING
		},
	}
	body, _ := json.Marshal(exchangeInfo)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer srv.Close()

	a := newTestBinanceAdapter(srv.URL)
	syms, err := a.FetchSymbols(context.Background(), "crypto")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(syms) != 2 {
		t.Fatalf("expected 2 USDT+TRADING symbols, got %d", len(syms))
	}

	ids := map[string]bool{}
	for _, s := range syms {
		ids[s.ID] = true
		if s.Market != "crypto" {
			t.Errorf("symbol %q: expected market crypto, got %q", s.ID, s.Market)
		}
		if !s.Active {
			t.Errorf("symbol %q should be active", s.ID)
		}
	}
	if !ids["BTC/USDT"] {
		t.Error("expected BTC/USDT in results")
	}
	if !ids["ETH/USDT"] {
		t.Error("expected ETH/USDT in results")
	}
}

func TestBinanceFetchSymbols_filtersNonTrading(t *testing.T) {
	exchangeInfo := map[string]interface{}{
		"symbols": []map[string]interface{}{
			{"symbol": "BTCUSDT", "status": "BREAK", "baseAsset": "BTC", "quoteAsset": "USDT"},
			{"symbol": "ETHUSDT", "status": "PRE_DELIVERING", "baseAsset": "ETH", "quoteAsset": "USDT"},
		},
	}
	body, _ := json.Marshal(exchangeInfo)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer srv.Close()

	a := newTestBinanceAdapter(srv.URL)
	syms, err := a.FetchSymbols(context.Background(), "crypto")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(syms) != 0 {
		t.Errorf("expected 0 symbols (all non-TRADING), got %d", len(syms))
	}
}

func TestBinanceIsHealthy_success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	}))
	defer srv.Close()

	a := newTestBinanceAdapter(srv.URL)
	if !a.IsHealthy(context.Background()) {
		t.Error("expected IsHealthy to return true")
	}
}

func TestBinanceIsHealthy_failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	a := newTestBinanceAdapter(srv.URL)
	if a.IsHealthy(context.Background()) {
		t.Error("expected IsHealthy to return false for 500")
	}
}

func TestBinanceSubscribeTicks_emptySymbols(t *testing.T) {
	a := NewBinanceAdapter("")
	ch, err := a.SubscribeTicks(context.Background(), []string{}, "crypto")
	if err == nil {
		t.Fatal("expected error for empty symbols list")
	}
	if ch != nil {
		t.Error("expected nil channel")
	}
}
