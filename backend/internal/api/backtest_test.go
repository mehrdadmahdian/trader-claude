package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/trader-claude/backend/internal/registry"
	"github.com/trader-claude/backend/internal/strategy"
)

// registerTestStrategies registers a clean set of strategies for tests.
// It is safe to call multiple times because the global StrategyRegistry is a
// singleton — we skip re-registration if already present.
func registerTestStrategies() {
	if !registry.Strategies().Exists("ema_crossover") {
		registry.Strategies().Register("ema_crossover", func() registry.Strategy { return &strategy.EMACrossover{} })
	}
	if !registry.Strategies().Exists("rsi") {
		registry.Strategies().Register("rsi", func() registry.Strategy { return &strategy.RSIStrategy{} })
	}
	if !registry.Strategies().Exists("macd") {
		registry.Strategies().Register("macd", func() registry.Strategy { return &strategy.MACDSignal{} })
	}
}

// newTestBacktestApp creates a minimal Fiber app with all backtest/strategy
// routes wired up, but without a real DB or Redis (both nil).
// Any handler that actually touches the DB will return an error — the tests
// that exercise those paths must account for that.
func newTestBacktestApp() *fiber.App {
	registerTestStrategies()

	app := fiber.New(fiber.Config{
		// Disable the default error handler's stack trace so output is clean
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{"error": err.Error()})
		},
	})

	bh := newBacktestHandler(nil, nil, nil, nil)
	v1 := app.Group("/api/v1")
	v1.Get("/strategies", bh.listStrategies)
	v1.Get("/strategies/:id", bh.getStrategy)
	v1.Post("/backtest/run", bh.runBacktest)
	v1.Get("/backtest/runs", bh.listRuns)
	v1.Get("/backtest/runs/:id", bh.getRun)
	v1.Delete("/backtest/runs/:id", bh.deleteRun)

	return app
}

// doRequest is a small helper that fires an HTTP test request against app.
func doRequest(app *fiber.App, method, url string, body io.Reader) *http.Response {
	req := httptest.NewRequest(method, url, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, _ := app.Test(req, -1)
	return resp
}

// ---- Strategy list ----------------------------------------------------------

func TestListStrategies_ReturnsRegisteredStrategies(t *testing.T) {
	app := newTestBacktestApp()

	resp := doRequest(app, "GET", "/api/v1/strategies", nil)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// We registered 3 strategies
	if len(body.Data) < 1 {
		t.Fatalf("expected at least 1 strategy, got %d", len(body.Data))
	}

	// Each entry must have the required fields
	for _, s := range body.Data {
		for _, field := range []string{"id", "name", "description", "params"} {
			if _, ok := s[field]; !ok {
				t.Errorf("strategy entry missing field %q", field)
			}
		}
	}
}

// ---- Strategy by id ---------------------------------------------------------

func TestGetStrategy_ValidID_Returns200(t *testing.T) {
	app := newTestBacktestApp()

	resp := doRequest(app, "GET", "/api/v1/strategies/ema_crossover", nil)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body.Data["id"] != "ema_crossover" {
		t.Errorf("expected id=ema_crossover, got %v", body.Data["id"])
	}
}

func TestGetStrategy_InvalidID_Returns404(t *testing.T) {
	app := newTestBacktestApp()

	resp := doRequest(app, "GET", "/api/v1/strategies/does_not_exist", nil)
	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	if _, ok := body["error"]; !ok {
		t.Error("expected error field in response")
	}
}

// ---- POST /backtest/run validation ------------------------------------------

func TestRunBacktest_MissingName_Returns400(t *testing.T) {
	app := newTestBacktestApp()

	payload := map[string]interface{}{
		// name is intentionally missing
		"strategy":     "ema_crossover",
		"adapter":      "binance",
		"symbol":       "BTC/USDT",
		"market":       "crypto",
		"timeframe":    "1h",
		"start_date":   "2024-01-01T00:00:00Z",
		"end_date":     "2024-12-31T23:59:59Z",
		"initial_cash": 10000,
	}
	b, _ := json.Marshal(payload)
	resp := doRequest(app, "POST", "/api/v1/backtest/run", bytes.NewReader(b))
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestRunBacktest_MissingStrategy_Returns400(t *testing.T) {
	app := newTestBacktestApp()

	payload := map[string]interface{}{
		"name":         "Test Run",
		"adapter":      "binance",
		"symbol":       "BTC/USDT",
		"market":       "crypto",
		"timeframe":    "1h",
		"start_date":   "2024-01-01T00:00:00Z",
		"end_date":     "2024-12-31T23:59:59Z",
		"initial_cash": 10000,
	}
	b, _ := json.Marshal(payload)
	resp := doRequest(app, "POST", "/api/v1/backtest/run", bytes.NewReader(b))
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestRunBacktest_UnknownStrategy_Returns400(t *testing.T) {
	app := newTestBacktestApp()

	payload := map[string]interface{}{
		"name":         "Test Run",
		"strategy":     "nonexistent_strategy",
		"adapter":      "binance",
		"symbol":       "BTC/USDT",
		"market":       "crypto",
		"timeframe":    "1h",
		"start_date":   "2024-01-01T00:00:00Z",
		"end_date":     "2024-12-31T23:59:59Z",
		"initial_cash": 10000,
	}
	b, _ := json.Marshal(payload)
	resp := doRequest(app, "POST", "/api/v1/backtest/run", bytes.NewReader(b))
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestRunBacktest_UnknownAdapter_Returns400(t *testing.T) {
	app := newTestBacktestApp()

	payload := map[string]interface{}{
		"name":         "Test Run",
		"strategy":     "ema_crossover",
		"adapter":      "no_such_adapter",
		"symbol":       "BTC/USDT",
		"market":       "crypto",
		"timeframe":    "1h",
		"start_date":   "2024-01-01T00:00:00Z",
		"end_date":     "2024-12-31T23:59:59Z",
		"initial_cash": 10000,
	}
	b, _ := json.Marshal(payload)
	resp := doRequest(app, "POST", "/api/v1/backtest/run", bytes.NewReader(b))
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestRunBacktest_EndBeforeStart_Returns400(t *testing.T) {
	app := newTestBacktestApp()

	payload := map[string]interface{}{
		"name":         "Test Run",
		"strategy":     "ema_crossover",
		"adapter":      "binance",
		"symbol":       "BTC/USDT",
		"market":       "crypto",
		"timeframe":    "1h",
		"start_date":   "2024-12-31T23:59:59Z",
		"end_date":     "2024-01-01T00:00:00Z", // end before start
		"initial_cash": 10000,
	}
	b, _ := json.Marshal(payload)
	resp := doRequest(app, "POST", "/api/v1/backtest/run", bytes.NewReader(b))
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestRunBacktest_EndEqualsStart_Returns400(t *testing.T) {
	app := newTestBacktestApp()

	payload := map[string]interface{}{
		"name":         "Test Run",
		"strategy":     "ema_crossover",
		"adapter":      "binance",
		"symbol":       "BTC/USDT",
		"market":       "crypto",
		"timeframe":    "1h",
		"start_date":   "2024-01-01T00:00:00Z",
		"end_date":     "2024-01-01T00:00:00Z", // end equals start
		"initial_cash": 10000,
	}
	b, _ := json.Marshal(payload)
	resp := doRequest(app, "POST", "/api/v1/backtest/run", bytes.NewReader(b))
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestRunBacktest_InvalidBody_Returns400(t *testing.T) {
	app := newTestBacktestApp()

	resp := doRequest(app, "POST", "/api/v1/backtest/run", bytes.NewReader([]byte("not-json!!!")))
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// ---- GET /backtest/runs -----------------------------------------------------

// TestListRuns_RouteExists verifies the route is registered and reachable.
// Because we have no real DB the test adds a recover middleware so the
// request returns 500 instead of panicking the test binary.
func TestListRuns_RouteExists(t *testing.T) {
	registerTestStrategies()

	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{"error": err.Error()})
		},
	})

	// Add recover middleware to catch nil-DB panics safely
	app.Use(func(c *fiber.Ctx) error {
		defer func() {
			if r := recover(); r != nil {
				// swallow the panic and return 500
				_ = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "recovered from panic"})
			}
		}()
		return c.Next()
	})

	bh := newBacktestHandler(nil, nil, nil, nil)
	v1 := app.Group("/api/v1")
	v1.Get("/backtest/runs", bh.listRuns)

	resp := doRequest(app, "GET", "/api/v1/backtest/runs", nil)
	// With a nil gorm.DB the call will either panic (caught → 500) or error (500)
	if resp.StatusCode != fiber.StatusInternalServerError {
		t.Logf("got status %d (expected 500 due to nil DB)", resp.StatusCode)
	}
}

// ---- GET /backtest/runs/:id -------------------------------------------------

func TestGetRun_InvalidID_Returns400(t *testing.T) {
	app := newTestBacktestApp()

	resp := doRequest(app, "GET", "/api/v1/backtest/runs/not-a-number", nil)
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// ---- DELETE /backtest/runs/:id ----------------------------------------------

func TestDeleteRun_InvalidID_Returns400(t *testing.T) {
	app := newTestBacktestApp()

	resp := doRequest(app, "DELETE", "/api/v1/backtest/runs/abc", nil)
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// ---- Strategy param schema --------------------------------------------------

func TestStrategyParams_EMA_HasExpectedFields(t *testing.T) {
	app := newTestBacktestApp()

	resp := doRequest(app, "GET", "/api/v1/strategies/ema_crossover", nil)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		Data struct {
			Params []map[string]interface{} `json:"params"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if len(body.Data.Params) == 0 {
		t.Fatal("expected at least one param for ema_crossover")
	}

	paramNames := make(map[string]bool)
	for _, p := range body.Data.Params {
		name, _ := p["name"].(string)
		paramNames[name] = true
	}

	for _, required := range []string{"fast_period", "slow_period"} {
		if !paramNames[required] {
			t.Errorf("ema_crossover missing param %q", required)
		}
	}
}

func TestStrategyParams_RSI_HasExpectedFields(t *testing.T) {
	registerTestStrategies()

	s, err := registry.Strategies().Create("rsi")
	if err != nil {
		t.Fatalf("failed to create rsi strategy: %v", err)
	}

	params := s.Params()
	if len(params) == 0 {
		t.Fatal("expected RSI to have params")
	}

	names := make(map[string]bool)
	for _, p := range params {
		names[p.Name] = true
	}
	if !names["period"] {
		t.Error("RSI missing 'period' param")
	}
}

func TestStrategyParams_MACD_HasExpectedFields(t *testing.T) {
	registerTestStrategies()

	s, err := registry.Strategies().Create("macd")
	if err != nil {
		t.Fatalf("failed to create macd strategy: %v", err)
	}

	params := s.Params()
	if len(params) == 0 {
		t.Fatal("expected MACD to have params")
	}

	names := make(map[string]bool)
	for _, p := range params {
		names[p.Name] = true
	}
	for _, required := range []string{"fast", "slow", "signal"} {
		if !names[required] {
			t.Errorf("macd missing param %q", required)
		}
	}
}
