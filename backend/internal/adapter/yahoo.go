package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/trader-claude/backend/internal/registry"
)

const (
	yahooBaseURL     = "https://query1.finance.yahoo.com/v8/finance/chart"
	yahooUserAgent   = "Mozilla/5.0 (compatible; trader-claude/1.0)"
	yahooHTTPTimeout = 15 * time.Second
)

// yahooChartResponse mirrors the Yahoo Finance v8 chart API response.
type yahooChartResponse struct {
	Chart struct {
		Result []struct {
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Open   []*float64 `json:"open"`
					High   []*float64 `json:"high"`
					Low    []*float64 `json:"low"`
					Close  []*float64 `json:"close"`
					Volume []*float64 `json:"volume"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
		Error *struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"error"`
	} `json:"chart"`
}

// YahooAdapter is a MarketAdapter implementation backed by Yahoo Finance.
type YahooAdapter struct {
	client  *http.Client
	baseURL string // overridable for testing
}

// NewYahooAdapter constructs a YahooAdapter with a default HTTP client.
func NewYahooAdapter() *YahooAdapter {
	return &YahooAdapter{
		client: &http.Client{
			Timeout: yahooHTTPTimeout,
		},
		baseURL: yahooBaseURL,
	}
}

func (a *YahooAdapter) Name() string       { return "yahoo" }
func (a *YahooAdapter) Markets() []string  { return []string{"stock", "etf", "forex"} }

func (a *YahooAdapter) IsHealthy(ctx context.Context) bool {
	healthURL := a.baseURL + "/AAPL?interval=1d&range=1d"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", yahooUserAgent)
	resp, err := a.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (a *YahooAdapter) FetchCandles(
	ctx context.Context,
	symbol, market, timeframe string,
	from, to time.Time,
) ([]registry.Candle, error) {
	interval, err := yahooMapTimeframe(timeframe)
	if err != nil {
		return nil, fmt.Errorf("yahoo FetchCandles: %w", err)
	}

	url := fmt.Sprintf(
		"%s/%s?interval=%s&period1=%d&period2=%d",
		a.baseURL, symbol, interval, from.Unix(), to.Unix(),
	)

	body, err := a.get(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("yahoo FetchCandles: %w", err)
	}

	var chartResp yahooChartResponse
	if err := json.Unmarshal(body, &chartResp); err != nil {
		return nil, fmt.Errorf("yahoo FetchCandles: parse response: %w", err)
	}

	if chartResp.Chart.Error != nil {
		return nil, fmt.Errorf("yahoo FetchCandles: API error %s: %s",
			chartResp.Chart.Error.Code, chartResp.Chart.Error.Description)
	}
	if len(chartResp.Chart.Result) == 0 {
		return nil, fmt.Errorf("yahoo FetchCandles: empty result for symbol %q", symbol)
	}

	result := chartResp.Chart.Result[0]
	if len(result.Indicators.Quote) == 0 {
		return nil, fmt.Errorf("yahoo FetchCandles: no quote data for symbol %q", symbol)
	}

	quotes := result.Indicators.Quote[0]
	timestamps := result.Timestamp
	n := len(timestamps)

	candles := make([]registry.Candle, 0, n)
	for i := 0; i < n; i++ {
		if i >= len(quotes.Open) || i >= len(quotes.High) ||
			i >= len(quotes.Low) || i >= len(quotes.Close) ||
			i >= len(quotes.Volume) {
			continue
		}
		if quotes.Open[i] == nil || quotes.High[i] == nil ||
			quotes.Low[i] == nil || quotes.Close[i] == nil ||
			quotes.Volume[i] == nil {
			continue
		}
		candles = append(candles, registry.Candle{
			Symbol:    symbol,
			Market:    market,
			Timeframe: timeframe,
			Timestamp: time.Unix(timestamps[i], 0).UTC(),
			Open:      *quotes.Open[i],
			High:      *quotes.High[i],
			Low:       *quotes.Low[i],
			Close:     *quotes.Close[i],
			Volume:    *quotes.Volume[i],
		})
	}
	return candles, nil
}

func (a *YahooAdapter) FetchSymbols(ctx context.Context, market string) ([]registry.Symbol, error) {
	tickers, ok := yahooCuratedSymbols[market]
	if !ok {
		return nil, fmt.Errorf("yahoo FetchSymbols: unsupported market %q", market)
	}
	symbols := make([]registry.Symbol, 0, len(tickers))
	for _, ticker := range tickers {
		symbols = append(symbols, registry.Symbol{ID: ticker, Market: market, Active: true})
	}
	return symbols, nil
}

func (a *YahooAdapter) SubscribeTicks(ctx context.Context, symbols []string, market string) (<-chan registry.Tick, error) {
	return nil, fmt.Errorf("yahoo adapter does not support streaming")
}

func (a *YahooAdapter) get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", yahooUserAgent)
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP GET %s: unexpected status %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	return body, nil
}

func yahooMapTimeframe(tf string) (string, error) {
	mapping := map[string]string{
		"1m": "1m", "5m": "5m", "15m": "15m", "30m": "30m",
		"1h": "60m", "4h": "60m", "1d": "1d", "1w": "1wk",
	}
	interval, ok := mapping[tf]
	if !ok {
		return "", fmt.Errorf("unsupported timeframe %q", tf)
	}
	return interval, nil
}

var yahooCuratedSymbols = map[string][]string{
	"stock": {
		"AAPL", "MSFT", "GOOGL", "AMZN", "NVDA", "META", "TSLA", "BRK-B",
		"LLY", "UNH", "JPM", "V", "XOM", "MA", "PG", "AVGO", "JNJ", "HD",
		"ABBV", "MRK", "CVX", "COST", "BAC", "KO", "PEP", "CRM", "WMT",
		"TMO", "CSCO", "ACN", "MCD", "ABT", "LIN", "DHR", "DIS", "NEE",
		"ADBE", "TXN", "CMCSA", "NKE", "ORCL", "PM", "INTC", "RTX", "AMGN",
		"LOW", "QCOM", "T", "GS", "CAT",
	},
	"etf": {
		"SPY", "QQQ", "IWM", "VTI", "VEA", "VWO", "AGG", "BND", "GLD",
		"SLV", "USO", "TLT", "IEF", "LQD", "HYG", "VNQ", "XLE", "XLF",
		"XLK", "XLV", "XLI", "XLC", "XLY", "XLP", "XLU", "XLB", "XLRE",
		"ARKK", "ARKW", "ARKG",
	},
	"forex": {
		"EURUSD=X", "GBPUSD=X", "USDJPY=X", "USDCHF=X", "AUDUSD=X",
		"USDCAD=X", "NZDUSD=X", "EURGBP=X", "EURJPY=X", "GBPJPY=X",
	},
}
