package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/models"
	"github.com/trader-claude/backend/internal/monitor"
	"github.com/trader-claude/backend/internal/registry"
)

// registerMonitorOnce ensures "test_strat" is registered exactly once,
// eliminating the TOCTOU race between Exists and Register under -race.
var registerMonitorOnce sync.Once

func registerTestMonitorStrategies() {
	registerMonitorOnce.Do(func() {
		if !registry.Strategies().Exists("test_strat") {
			registry.Strategies().Register("test_strat", func() registry.Strategy { return nil }) //nolint:errcheck
		}
	})
}

// openTestDB opens a connection to the MySQL instance available in the Docker
// environment. It also runs AutoMigrate for monitor-related tables so the tests
// are self-contained and do not depend on the server having started first.
// Returns (db, true) on success, (nil, false) when the DB is not reachable —
// callers must skip the test in that case.
func openTestDB(t *testing.T) (*gorm.DB, bool) {
	t.Helper()

	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "mysql"
	}
	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "3306"
	}
	user := os.Getenv("DB_USER")
	if user == "" {
		user = "trader"
	}
	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		password = "traderpassword"
	}
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "trader"
	}

	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=UTC",
		user, password, host, port, dbName,
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Logf("skipping DB test: cannot open DB: %v", err)
		return nil, false
	}

	// Ensure the monitor tables exist — this is idempotent and safe to run
	// in tests even when the server has not been started yet.
	if err := db.AutoMigrate(&models.Monitor{}, &models.MonitorSignal{}); err != nil {
		t.Logf("skipping DB test: AutoMigrate failed: %v", err)
		return nil, false
	}

	return db, true
}

// cleanMonitors deletes all test-inserted monitor rows by name prefix to keep
// tests idempotent. It also removes any associated signals.
func cleanMonitors(db *gorm.DB, namePrefix string) {
	var ids []int64
	db.Unscoped().
		Model(&models.Monitor{}).
		Where("name LIKE ?", namePrefix+"%").
		Pluck("id", &ids)
	if len(ids) > 0 {
		db.Unscoped().Where("monitor_id IN ?", ids).Delete(&models.MonitorSignal{})
	}
	db.Unscoped().Where("name LIKE ?", namePrefix+"%").Delete(&models.Monitor{})
}

// setupMonitorTestApp creates a Fiber app backed by the real MySQL DB and a
// no-op Manager (pool is nil — timers are set to far-future intervals so
// pool.Submit is never called during the short test run).
func setupMonitorTestApp(t *testing.T) (*fiber.App, *gorm.DB, bool) {
	t.Helper()

	db, ok := openTestDB(t)
	if !ok {
		return nil, nil, false
	}

	// Register a dummy strategy so validation passes — uses sync.Once to be race-safe.
	registerTestMonitorStrategies()

	mgr := monitor.NewManager(db, nil, nil, nil)

	app := fiber.New()
	mnh := newMonitorHandler(db, mgr)
	v1 := app.Group("/api/v1")
	v1.Post("/monitors", mnh.createMonitor)
	v1.Get("/monitors", mnh.listMonitors)
	v1.Get("/monitors/:id", mnh.getMonitor)
	v1.Put("/monitors/:id", mnh.updateMonitor)
	v1.Delete("/monitors/:id", mnh.deleteMonitor)
	v1.Patch("/monitors/:id/toggle", mnh.toggleMonitor)
	v1.Get("/monitors/:id/signals", mnh.listSignals)

	return app, db, true
}

// ---- Validation-only tests (no DB needed) -----------------------------------

func TestCreateMonitor_UnknownStrategy_Returns400(t *testing.T) {
	// Validation happens before the DB is touched — no real DB required.
	registerTestStrategies()

	app := fiber.New()
	mnh := newMonitorHandler(nil, monitor.NewManager(nil, nil, nil, nil))
	v1 := app.Group("/api/v1")
	v1.Post("/monitors", mnh.createMonitor)

	body := map[string]interface{}{
		"adapter_id":    "binance",
		"symbol":        "BTCUSDT",
		"market":        "crypto",
		"timeframe":     "1h",
		"strategy_name": "does_not_exist_xyz",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/monitors", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateMonitor_MissingRequired_Returns400(t *testing.T) {
	registerTestStrategies()

	app := fiber.New()
	mnh := newMonitorHandler(nil, monitor.NewManager(nil, nil, nil, nil))
	v1 := app.Group("/api/v1")
	v1.Post("/monitors", mnh.createMonitor)

	// Missing strategy_name
	body := map[string]interface{}{
		"adapter_id": "binance",
		"symbol":     "BTCUSDT",
		"market":     "crypto",
		"timeframe":  "1h",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/monitors", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestGetMonitor_InvalidID_Returns400(t *testing.T) {
	registerTestStrategies()

	app := fiber.New()
	mnh := newMonitorHandler(nil, monitor.NewManager(nil, nil, nil, nil))
	v1 := app.Group("/api/v1")
	v1.Get("/monitors/:id", mnh.getMonitor)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/monitors/not-a-number", nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestToggleMonitor_InvalidID_Returns400(t *testing.T) {
	registerTestStrategies()

	app := fiber.New()
	mnh := newMonitorHandler(nil, monitor.NewManager(nil, nil, nil, nil))
	v1 := app.Group("/api/v1")
	v1.Patch("/monitors/:id/toggle", mnh.toggleMonitor)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/monitors/abc/toggle", nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestListSignals_InvalidID_Returns400(t *testing.T) {
	registerTestStrategies()

	app := fiber.New()
	mnh := newMonitorHandler(nil, monitor.NewManager(nil, nil, nil, nil))
	v1 := app.Group("/api/v1")
	v1.Get("/monitors/:id/signals", mnh.listSignals)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/monitors/nope/signals", nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// ---- DB-backed integration tests --------------------------------------------

func TestCreateMonitor_DB(t *testing.T) {
	app, db, ok := setupMonitorTestApp(t)
	if !ok {
		t.Skip("MySQL not available")
	}
	prefix := "test_mon_create_"
	t.Cleanup(func() { cleanMonitors(db, prefix) })

	body := map[string]interface{}{
		"name":          prefix + "001",
		"adapter_id":    "binance",
		"symbol":        "BTCUSDT",
		"market":        "crypto",
		"timeframe":     "1h",
		"strategy_name": "test_strat",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/monitors", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var mon models.Monitor
	json.NewDecoder(resp.Body).Decode(&mon)
	if mon.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if mon.Status != models.MonitorStatusActive {
		t.Errorf("expected status active, got %s", mon.Status)
	}
	if mon.Name == "" {
		t.Error("expected non-empty name")
	}
}

func TestListMonitors_DB(t *testing.T) {
	app, db, ok := setupMonitorTestApp(t)
	if !ok {
		t.Skip("MySQL not available")
	}
	prefix := "test_mon_list_"
	t.Cleanup(func() { cleanMonitors(db, prefix) })

	db.Create(&models.Monitor{Name: prefix + "M1", AdapterID: "binance", Symbol: "BTCUSDT", Market: "crypto", Timeframe: "1h", StrategyName: "ema_crossover", Status: models.MonitorStatusActive})
	db.Create(&models.Monitor{Name: prefix + "M2", AdapterID: "binance", Symbol: "ETHUSDT", Market: "crypto", Timeframe: "1h", StrategyName: "rsi", Status: models.MonitorStatusPaused})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/monitors", nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result struct {
		Data []models.Monitor `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	// We inserted 2 rows with our prefix; there may be others in the DB.
	if len(result.Data) < 2 {
		t.Errorf("expected at least 2 monitors, got %d", len(result.Data))
	}
}

func TestGetMonitor_DB(t *testing.T) {
	app, db, ok := setupMonitorTestApp(t)
	if !ok {
		t.Skip("MySQL not available")
	}
	prefix := "test_mon_get_"
	t.Cleanup(func() { cleanMonitors(db, prefix) })

	mon := models.Monitor{Name: prefix + "M1", AdapterID: "binance", Symbol: "BTCUSDT", Market: "crypto", Timeframe: "1h", StrategyName: "ema_crossover", Status: models.MonitorStatusActive}
	db.Create(&mon)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/monitors/%d", mon.ID), nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var fetched models.Monitor
	json.NewDecoder(resp.Body).Decode(&fetched)
	if fetched.ID != mon.ID {
		t.Errorf("expected ID %d, got %d", mon.ID, fetched.ID)
	}
}

func TestGetMonitor_NotFound_Returns404(t *testing.T) {
	app, _, ok := setupMonitorTestApp(t)
	if !ok {
		t.Skip("MySQL not available")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/monitors/999999999", nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestToggleMonitor_DB(t *testing.T) {
	app, db, ok := setupMonitorTestApp(t)
	if !ok {
		t.Skip("MySQL not available")
	}
	prefix := "test_mon_toggle_"
	t.Cleanup(func() { cleanMonitors(db, prefix) })

	mon := models.Monitor{Name: prefix + "M1", AdapterID: "binance", Symbol: "BTCUSDT", Market: "crypto", Timeframe: "1h", StrategyName: "ema_crossover", Status: models.MonitorStatusActive}
	db.Create(&mon)

	// Toggle: active → paused
	req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/monitors/%d/toggle", mon.ID), nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("toggle 1 expected 200, got %d", resp.StatusCode)
	}

	var toggled models.Monitor
	json.NewDecoder(resp.Body).Decode(&toggled)
	if toggled.Status != models.MonitorStatusPaused {
		t.Errorf("expected paused, got %s", toggled.Status)
	}

	// Toggle again: paused → active
	req = httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/monitors/%d/toggle", mon.ID), nil)
	resp, _ = app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("toggle 2 expected 200, got %d", resp.StatusCode)
	}
	json.NewDecoder(resp.Body).Decode(&toggled)
	if toggled.Status != models.MonitorStatusActive {
		t.Errorf("expected active, got %s", toggled.Status)
	}
}

func TestDeleteMonitor_DB(t *testing.T) {
	app, db, ok := setupMonitorTestApp(t)
	if !ok {
		t.Skip("MySQL not available")
	}
	prefix := "test_mon_delete_"
	t.Cleanup(func() { cleanMonitors(db, prefix) })

	mon := models.Monitor{Name: prefix + "ToDelete", AdapterID: "binance", Symbol: "BTCUSDT", Market: "crypto", Timeframe: "1h", StrategyName: "ema_crossover", Status: models.MonitorStatusActive}
	db.Create(&mon)

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/monitors/%d", mon.ID), nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	var count int64
	db.Unscoped().Model(&models.Monitor{}).
		Where("id = ?", mon.ID).
		Where("deleted_at IS NOT NULL").
		Count(&count)
	if count != 1 {
		t.Error("expected monitor to be soft-deleted")
	}
}

func TestListSignals_DB(t *testing.T) {
	app, db, ok := setupMonitorTestApp(t)
	if !ok {
		t.Skip("MySQL not available")
	}
	prefix := "test_mon_signals_"
	t.Cleanup(func() { cleanMonitors(db, prefix) })

	mon := models.Monitor{Name: prefix + "M1", AdapterID: "binance", Symbol: "BTCUSDT", Market: "crypto", Timeframe: "1h", StrategyName: "ema_crossover", Status: models.MonitorStatusActive}
	db.Create(&mon)
	db.Create(&models.MonitorSignal{MonitorID: mon.ID, Direction: "long", Price: 50000.0, Strength: 0.8})
	db.Create(&models.MonitorSignal{MonitorID: mon.ID, Direction: "short", Price: 48000.0, Strength: 0.6})

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/monitors/%d/signals", mon.ID), nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result struct {
		Data  []models.MonitorSignal `json:"data"`
		Total int64                  `json:"total"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Total != 2 {
		t.Errorf("expected total 2, got %d", result.Total)
	}
}
