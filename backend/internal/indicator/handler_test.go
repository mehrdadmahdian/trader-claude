package indicator

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func testApp() *fiber.App {
	app := fiber.New()
	h := NewHandler()
	app.Get("/api/v1/indicators", h.ListIndicators)
	app.Post("/api/v1/indicators/calculate", h.Calculate)
	return app
}

func TestListIndicators(t *testing.T) {
	app := testApp()
	req := httptest.NewRequest("GET", "/api/v1/indicators", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status %d, want 200", resp.StatusCode)
	}
	var body struct {
		Indicators []IndicatorMeta `json:"indicators"`
	}
	data, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("unmarshal error: %v\nbody: %s", err, data)
	}
	if len(body.Indicators) != 13 {
		t.Errorf("expected 13 indicators, got %d", len(body.Indicators))
	}
	// Verify all expected IDs are present
	ids := make(map[string]bool)
	for _, ind := range body.Indicators {
		ids[ind.ID] = true
	}
	for _, want := range []string{"sma", "ema", "wma", "bollinger_bands", "vwap", "parabolic_sar", "ichimoku",
		"rsi", "macd", "stochastic", "atr", "obv", "volume"} {
		if !ids[want] {
			t.Errorf("missing indicator %q in catalog", want)
		}
	}
}

func TestCalculateEMA(t *testing.T) {
	app := testApp()
	payload := map[string]interface{}{
		"indicator_id": "ema",
		"params":       map[string]interface{}{"period": 3},
		"candles": []map[string]interface{}{
			{"timestamp": 0, "open": 1, "high": 1, "low": 1, "close": 1, "volume": 100},
			{"timestamp": 3600, "open": 2, "high": 2, "low": 2, "close": 2, "volume": 100},
			{"timestamp": 7200, "open": 3, "high": 3, "low": 3, "close": 3, "volume": 100},
			{"timestamp": 10800, "open": 4, "high": 4, "low": 4, "close": 4, "volume": 100},
			{"timestamp": 14400, "open": 5, "high": 5, "low": 5, "close": 5, "volume": 100},
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/indicators/calculate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, data)
	}
	var result struct {
		Timestamps []int64                  `json:"timestamps"`
		Series     map[string][]interface{} `json:"series"`
	}
	data, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(result.Timestamps) != 5 {
		t.Errorf("expected 5 timestamps, got %d", len(result.Timestamps))
	}
	vals, ok := result.Series["value"]
	if !ok {
		t.Fatal("missing 'value' series")
	}
	// First two are null (warm-up NaN → null)
	if vals[0] != nil || vals[1] != nil {
		t.Errorf("expected null for warm-up, got %v %v", vals[0], vals[1])
	}
	// Third value ≈ 2.0 (SMA seed of [1,2,3])
	if v, ok := vals[2].(float64); !ok || v < 1.9 || v > 2.1 {
		t.Errorf("vals[2]=%v, want ~2.0", vals[2])
	}
}

func TestCalculate_UnknownIndicator(t *testing.T) {
	app := testApp()
	payload := map[string]interface{}{
		"indicator_id": "not_real",
		"params":       map[string]interface{}{},
		"candles": []map[string]interface{}{
			{"timestamp": 0, "open": 1, "high": 1, "low": 1, "close": 1, "volume": 100},
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/indicators/calculate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCalculate_MissingIndicatorID(t *testing.T) {
	app := testApp()
	payload := map[string]interface{}{
		"params":  map[string]interface{}{},
		"candles": []map[string]interface{}{{"timestamp": 0, "open": 1, "high": 1, "low": 1, "close": 1, "volume": 100}},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/indicators/calculate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCalculate_EmptyCandles(t *testing.T) {
	app := testApp()
	payload := map[string]interface{}{
		"indicator_id": "ema",
		"params":       map[string]interface{}{"period": 20},
		"candles":      []map[string]interface{}{},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/indicators/calculate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}
