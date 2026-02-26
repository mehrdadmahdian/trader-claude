package price

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestGetPrice_Binance(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3/ticker/price" {
			json.NewEncoder(w).Encode(map[string]string{"symbol": "BTCUSDT", "price": "42000.5"})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	svc := NewService(nil, srv.URL, "")
	price, err := svc.GetPrice(context.Background(), "binance", "BTCUSDT")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if price != 42000.5 {
		t.Errorf("expected 42000.5, got %f", price)
	}
}

func TestGetPrice_Yahoo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"chart": map[string]interface{}{
				"result": []interface{}{
					map[string]interface{}{
						"meta": map[string]interface{}{
							"regularMarketPrice": 185.5,
						},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	svc := NewService(nil, "", srv.URL)
	price, err := svc.GetPrice(context.Background(), "yahoo", "AAPL")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if price != 185.5 {
		t.Errorf("expected 185.5, got %f", price)
	}
}

func TestGetPrice_UnknownAdapter(t *testing.T) {
	svc := NewService(nil, "", "")
	_, err := svc.GetPrice(context.Background(), "unknown", "XYZ")
	if err == nil {
		t.Fatal("expected error for unknown adapter")
	}
	if !errors.Is(err, ErrPriceUnavailable) {
		t.Errorf("expected ErrPriceUnavailable, got %v", err)
	}
}

func TestGetPrice_CacheHit(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skip("redis not available, skipping cache test")
	}

	ctx2 := context.Background()
	key := "price:binance:TESTCACHEBTC"
	rdb.Set(ctx2, key, "99000.0", 30*time.Second)
	defer rdb.Del(ctx2, key)

	svc := NewService(rdb, "http://127.0.0.1:1", "http://127.0.0.1:1") // unreachable URLs
	price, err := svc.GetPrice(ctx2, "binance", "TESTCACHEBTC")
	if err != nil {
		t.Fatalf("expected no error from cache, got %v", err)
	}
	if price != 99000.0 {
		t.Errorf("expected 99000.0 from cache, got %f", price)
	}
}
