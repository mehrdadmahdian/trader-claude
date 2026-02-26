package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/trader-claude/backend/internal/models"
	"github.com/trader-claude/backend/internal/portfolio"
	"github.com/trader-claude/backend/internal/price"
)

func newPortfolioTestApp(t *testing.T) *fiber.App {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&models.Portfolio{}, &models.Position{}, &models.Transaction{}); err != nil {
		t.Fatalf("auto-migrate: %v", err)
	}

	priceSvc := price.NewService(nil, "http://127.0.0.1:1", "http://127.0.0.1:1")
	svc := portfolio.NewService(db, priceSvc)
	h := newPortfolioHandler(svc)

	app := fiber.New(fiber.Config{ErrorHandler: func(c *fiber.Ctx, err error) error {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}})
	v1 := app.Group("/api/v1")
	h.registerRoutes(v1)
	return app
}

func createTestPortfolio(t *testing.T, app *fiber.App) int {
	t.Helper()
	body, _ := json.Marshal(map[string]interface{}{
		"name": "Test", "type": "manual", "currency": "USD", "initial_cash": 1000,
	})
	req := httptest.NewRequest("POST", "/api/v1/portfolios", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil || resp.StatusCode != 201 {
		t.Fatalf("create portfolio failed: status=%d err=%v", resp.StatusCode, err)
	}
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	data := result["data"].(map[string]interface{})
	return int(data["id"].(float64))
}

func TestPortfolioAPI_CreateAndList(t *testing.T) {
	app := newPortfolioTestApp(t)

	id := createTestPortfolio(t, app)
	if id == 0 {
		t.Fatal("expected non-zero id")
	}

	req := httptest.NewRequest("GET", "/api/v1/portfolios", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("list: expected 200, got %d", resp.StatusCode)
	}
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	data := result["data"].([]interface{})
	if len(data) != 1 {
		t.Errorf("expected 1 portfolio, got %d", len(data))
	}
}

func TestPortfolioAPI_GetByID(t *testing.T) {
	app := newPortfolioTestApp(t)
	id := createTestPortfolio(t, app)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/portfolios/%d", id), nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("get: expected 200, got %d", resp.StatusCode)
	}
}

func TestPortfolioAPI_AddPosition(t *testing.T) {
	app := newPortfolioTestApp(t)
	id := createTestPortfolio(t, app)

	posBody, _ := json.Marshal(map[string]interface{}{
		"adapter_id": "binance", "symbol": "BTCUSDT", "market": "crypto",
		"quantity": 1.0, "avg_cost": 40000.0, "opened_at": "2024-01-01T00:00:00Z",
	})
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/portfolios/%d/positions", id), bytes.NewReader(posBody))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 201 {
		t.Errorf("add position: expected 201, got %d", resp.StatusCode)
	}
}

func TestPortfolioAPI_Transactions(t *testing.T) {
	app := newPortfolioTestApp(t)
	id := createTestPortfolio(t, app)

	txBody, _ := json.Marshal(map[string]interface{}{
		"type": "deposit", "price": 500.0, "quantity": 1.0,
		"executed_at": "2024-01-01T00:00:00Z",
	})
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/portfolios/%d/transactions", id), bytes.NewReader(txBody))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 201 {
		t.Errorf("add tx: expected 201, got %d", resp.StatusCode)
	}

	req2 := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/portfolios/%d/transactions", id), nil)
	resp2, _ := app.Test(req2)
	if resp2.StatusCode != 200 {
		t.Errorf("list tx: expected 200, got %d", resp2.StatusCode)
	}
	var result map[string]interface{}
	json.NewDecoder(resp2.Body).Decode(&result)
	if result["total"].(float64) != 1 {
		t.Errorf("expected total=1, got %v", result["total"])
	}
}

func TestPortfolioAPI_EquityCurve(t *testing.T) {
	app := newPortfolioTestApp(t)
	id := createTestPortfolio(t, app)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/portfolios/%d/equity-curve", id), nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("equity-curve: expected 200, got %d", resp.StatusCode)
	}
}

func TestPortfolioAPI_Delete(t *testing.T) {
	app := newPortfolioTestApp(t)
	id := createTestPortfolio(t, app)

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/portfolios/%d", id), nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 204 {
		t.Errorf("delete: expected 204, got %d", resp.StatusCode)
	}
}
