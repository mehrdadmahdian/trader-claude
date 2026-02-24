package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/trader-claude/backend/internal/registry"
)

const (
	binanceBaseURL    = "https://api.binance.com"
	binanceWSBaseURL  = "wss://stream.binance.com/stream"
	binanceHTTPTimeout = 15 * time.Second
	pingInterval      = 30 * time.Second
	maxRetries        = 3
)

// BinanceAdapter implements registry.MarketAdapter for Binance crypto exchange.
type BinanceAdapter struct {
	client  *http.Client
	baseURL string // overridable for testing
}

// NewBinanceAdapter creates a BinanceAdapter. If baseURL is empty, uses the
// default Binance REST base URL.
func NewBinanceAdapter(baseURL string) *BinanceAdapter {
	if baseURL == "" {
		baseURL = binanceBaseURL
	}
	return &BinanceAdapter{
		client:  &http.Client{Timeout: binanceHTTPTimeout},
		baseURL: baseURL,
	}
}

func (b *BinanceAdapter) Name() string       { return "binance" }
func (b *BinanceAdapter) Markets() []string  { return []string{"crypto"} }

func (b *BinanceAdapter) IsHealthy(ctx context.Context) bool {
	url := b.baseURL + "/api/v3/ping"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := b.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// FetchCandles fetches historical OHLCV data from Binance /api/v3/klines.
// It paginates automatically when the requested range exceeds 1000 candles.
func (b *BinanceAdapter) FetchCandles(
	ctx context.Context,
	symbol, market, timeframe string,
	from, to time.Time,
) ([]registry.Candle, error) {
	interval, err := binanceMapTimeframe(timeframe)
	if err != nil {
		return nil, fmt.Errorf("binance FetchCandles: %w", err)
	}

	binanceSym := toBinanceSymbol(symbol)
	var all []registry.Candle

	startMs := from.UnixMilli()
	endMs := to.UnixMilli()

	for startMs < endMs {
		url := fmt.Sprintf(
			"%s/api/v3/klines?symbol=%s&interval=%s&startTime=%d&endTime=%d&limit=1000",
			b.baseURL, binanceSym, interval, startMs, endMs,
		)

		body, err := b.getWithRetry(ctx, url)
		if err != nil {
			return nil, fmt.Errorf("binance FetchCandles: %w", err)
		}

		batch, err := parseKlines(body, symbol, market, timeframe)
		if err != nil {
			return nil, fmt.Errorf("binance FetchCandles: %w", err)
		}

		if len(batch) == 0 {
			break
		}

		all = append(all, batch...)

		// Advance past the last returned candle
		lastOpenMs := batch[len(batch)-1].Timestamp.UnixMilli()
		startMs = lastOpenMs + 1

		// Fewer than 1000 means we've reached the end
		if len(batch) < 1000 {
			break
		}
	}

	return all, nil
}

// FetchSymbols returns all actively-traded USDT pairs from /api/v3/exchangeInfo.
func (b *BinanceAdapter) FetchSymbols(ctx context.Context, market string) ([]registry.Symbol, error) {
	url := b.baseURL + "/api/v3/exchangeInfo"
	body, err := b.getWithRetry(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("binance FetchSymbols: %w", err)
	}

	var info struct {
		Symbols []struct {
			Symbol     string `json:"symbol"`
			Status     string `json:"status"`
			BaseAsset  string `json:"baseAsset"`
			QuoteAsset string `json:"quoteAsset"`
		} `json:"symbols"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("binance FetchSymbols: parse response: %w", err)
	}

	symbols := make([]registry.Symbol, 0, len(info.Symbols))
	for _, s := range info.Symbols {
		if s.Status != "TRADING" || s.QuoteAsset != "USDT" {
			continue
		}
		symbols = append(symbols, registry.Symbol{
			ID:          s.BaseAsset + "/USDT",
			Market:      market,
			BaseAsset:   s.BaseAsset,
			QuoteAsset:  "USDT",
			Description: s.BaseAsset + "/USDT",
			Active:      true,
		})
	}
	return symbols, nil
}

// SubscribeTicks opens a Binance combined-stream WebSocket and forwards
// 24h-ticker events as registry.Tick values on the returned channel.
// The channel is closed when ctx is cancelled or the connection drops.
func (b *BinanceAdapter) SubscribeTicks(
	ctx context.Context,
	symbols []string,
	market string,
) (<-chan registry.Tick, error) {
	if len(symbols) == 0 {
		return nil, fmt.Errorf("binance SubscribeTicks: at least one symbol required")
	}

	streams := make([]string, 0, len(symbols))
	for _, sym := range symbols {
		streams = append(streams, strings.ToLower(toBinanceSymbol(sym))+"@ticker")
	}
	wsURL := binanceWSBaseURL + "?streams=" + strings.Join(streams, "/")

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("binance SubscribeTicks: websocket dial: %w", err)
	}

	ch := make(chan registry.Tick, 256)

	go func() {
		defer func() {
			conn.Close()
			close(ch)
		}()

		pingTicker := time.NewTicker(pingInterval)
		defer pingTicker.Stop()

		type wsMsg struct {
			data []byte
			err  error
		}
		msgCh := make(chan wsMsg, 64)

		go func() {
			for {
				_, data, err := conn.ReadMessage()
				msgCh <- wsMsg{data: data, err: err}
				if err != nil {
					return
				}
			}
		}()

		for {
			select {
			case <-ctx.Done():
				_ = conn.WriteMessage(
					websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
				)
				return

			case <-pingTicker.C:
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}

			case msg := <-msgCh:
				if msg.err != nil {
					return
				}
				tick, err := parseCombinedTicker(msg.data, market)
				if err != nil {
					continue
				}
				select {
				case ch <- tick:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return ch, nil
}

// --- helpers ---

func (b *BinanceAdapter) getWithRetry(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	backoff := time.Second

	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}

		resp, err := b.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			lastErr = fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
		}

		return body, nil
	}

	return nil, fmt.Errorf("after %d retries: %w", maxRetries, lastErr)
}

// parseKlines decodes a Binance /api/v3/klines response.
// Each element is a JSON array: [openTime, open, high, low, close, volume, ...].
func parseKlines(body []byte, symbol, market, timeframe string) ([]registry.Candle, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse klines: %w", err)
	}

	candles := make([]registry.Candle, 0, len(raw))
	for _, row := range raw {
		var fields []json.RawMessage
		if err := json.Unmarshal(row, &fields); err != nil || len(fields) < 6 {
			continue
		}

		openTimeMs, err := parseRawInt64(fields[0])
		if err != nil {
			continue
		}
		open, err := parseRawFloat(fields[1])
		if err != nil {
			continue
		}
		high, err := parseRawFloat(fields[2])
		if err != nil {
			continue
		}
		low, err := parseRawFloat(fields[3])
		if err != nil {
			continue
		}
		close_, err := parseRawFloat(fields[4])
		if err != nil {
			continue
		}
		volume, err := parseRawFloat(fields[5])
		if err != nil {
			continue
		}

		candles = append(candles, registry.Candle{
			Symbol:    symbol,
			Market:    market,
			Timeframe: timeframe,
			Timestamp: time.UnixMilli(openTimeMs).UTC(),
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close_,
			Volume:    volume,
		})
	}

	return candles, nil
}

type combinedStreamEnvelope struct {
	Stream string          `json:"stream"`
	Data   json.RawMessage `json:"data"`
}

type tickerPayload struct {
	Symbol    string `json:"s"` // e.g. "BTCUSDT"
	LastPrice string `json:"c"` // last price
	Volume    string `json:"v"` // base asset volume (24h)
	CloseTime int64  `json:"T"` // last trade time (ms)
	BidPrice  string `json:"b"` // best bid
	AskPrice  string `json:"a"` // best ask
}

func parseCombinedTicker(data []byte, market string) (registry.Tick, error) {
	var env combinedStreamEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return registry.Tick{}, err
	}

	var payload tickerPayload
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		return registry.Tick{}, err
	}

	price, err := strconv.ParseFloat(payload.LastPrice, 64)
	if err != nil {
		return registry.Tick{}, fmt.Errorf("parse price: %w", err)
	}
	volume, err := strconv.ParseFloat(payload.Volume, 64)
	if err != nil {
		return registry.Tick{}, fmt.Errorf("parse volume: %w", err)
	}

	var bid, ask float64
	if payload.BidPrice != "" {
		bid, _ = strconv.ParseFloat(payload.BidPrice, 64)
	}
	if payload.AskPrice != "" {
		ask, _ = strconv.ParseFloat(payload.AskPrice, 64)
	}

	return registry.Tick{
		Symbol:    fromBinanceSymbol(payload.Symbol),
		Market:    market,
		Price:     price,
		Volume:    volume,
		Timestamp: time.UnixMilli(payload.CloseTime).UTC(),
		Bid:       bid,
		Ask:       ask,
	}, nil
}

func binanceMapTimeframe(tf string) (string, error) {
	mapping := map[string]string{
		"1m": "1m", "5m": "5m", "15m": "15m", "30m": "30m",
		"1h": "1h", "4h": "4h", "1d": "1d", "1w": "1w",
	}
	interval, ok := mapping[tf]
	if !ok {
		return "", fmt.Errorf("unsupported timeframe %q", tf)
	}
	return interval, nil
}

// toBinanceSymbol converts "BTC/USDT" → "BTCUSDT".
func toBinanceSymbol(symbol string) string {
	return strings.ToUpper(strings.ReplaceAll(symbol, "/", ""))
}

// fromBinanceSymbol converts "BTCUSDT" → "BTC/USDT" by stripping known quote suffixes.
func fromBinanceSymbol(symbol string) string {
	for _, quote := range []string{"USDT", "BTC", "ETH", "BNB"} {
		if strings.HasSuffix(symbol, quote) {
			base := strings.TrimSuffix(symbol, quote)
			return base + "/" + quote
		}
	}
	return symbol
}

// parseRawFloat decodes a json.RawMessage that is either a quoted decimal
// string (as Binance encodes prices) or a bare JSON number.
func parseRawFloat(raw json.RawMessage) (float64, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strconv.ParseFloat(s, 64)
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err != nil {
		return 0, err
	}
	return f, nil
}

// parseRawInt64 decodes a json.RawMessage that is a bare JSON integer.
func parseRawInt64(raw json.RawMessage) (int64, error) {
	var n int64
	if err := json.Unmarshal(raw, &n); err != nil {
		return 0, err
	}
	return n, nil
}
