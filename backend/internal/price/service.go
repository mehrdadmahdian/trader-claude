package price

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrPriceUnavailable = errors.New("price unavailable")

const cacheTTL = 30 * time.Second

type Service struct {
	rdb        *redis.Client
	binanceURL string
	yahooURL   string
	httpClient *http.Client
}

// NewService creates a PriceService.
// Pass empty strings for binanceURL/yahooURL to use production defaults.
func NewService(rdb *redis.Client, binanceURL, yahooURL string) *Service {
	if binanceURL == "" {
		binanceURL = "https://api.binance.com"
	}
	if yahooURL == "" {
		yahooURL = "https://query1.finance.yahoo.com"
	}
	return &Service{
		rdb:        rdb,
		binanceURL: binanceURL,
		yahooURL:   yahooURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// GetPrice returns the current price for symbol via adapterID ("binance" or "yahoo").
// Checks Redis cache first (TTL 30s), then fetches live.
func (s *Service) GetPrice(ctx context.Context, adapterID, symbol string) (float64, error) {
	// Check cache
	if s.rdb != nil {
		key := fmt.Sprintf("price:%s:%s", adapterID, symbol)
		val, err := s.rdb.Get(ctx, key).Result()
		if err == nil {
			if price, parseErr := strconv.ParseFloat(val, 64); parseErr == nil {
				return price, nil
			}
		}
	}

	var price float64
	var err error

	switch adapterID {
	case "binance":
		price, err = s.fetchBinancePrice(ctx, symbol)
	case "yahoo":
		price, err = s.fetchYahooPrice(ctx, symbol)
	default:
		return 0, fmt.Errorf("%w: unknown adapter %q", ErrPriceUnavailable, adapterID)
	}

	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrPriceUnavailable, err)
	}

	// Cache result
	if s.rdb != nil {
		key := fmt.Sprintf("price:%s:%s", adapterID, symbol)
		s.rdb.Set(ctx, key, strconv.FormatFloat(price, 'f', 8, 64), cacheTTL)
	}

	return price, nil
}

func (s *Service) fetchBinancePrice(ctx context.Context, symbol string) (float64, error) {
	url := fmt.Sprintf("%s/api/v3/ticker/price?symbol=%s", s.binanceURL, symbol)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HTTP %d from Binance", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var result struct {
		Price string `json:"price"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}
	return strconv.ParseFloat(result.Price, 64)
}

func (s *Service) fetchYahooPrice(ctx context.Context, symbol string) (float64, error) {
	url := fmt.Sprintf("%s/v8/finance/chart/%s?range=1d&interval=1m", s.yahooURL, symbol)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HTTP %d from Yahoo Finance", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var result struct {
		Chart struct {
			Result []struct {
				Meta struct {
					RegularMarketPrice float64 `json:"regularMarketPrice"`
				} `json:"meta"`
			} `json:"result"`
		} `json:"chart"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}
	if len(result.Chart.Result) == 0 {
		return 0, fmt.Errorf("no result for symbol %s", symbol)
	}
	price := result.Chart.Result[0].Meta.RegularMarketPrice
	if price == 0 {
		return 0, fmt.Errorf("zero price returned for symbol %s", symbol)
	}
	return price, nil
}
