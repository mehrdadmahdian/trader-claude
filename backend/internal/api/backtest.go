package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/adapter"
	"github.com/trader-claude/backend/internal/backtest"
	"github.com/trader-claude/backend/internal/models"
	"github.com/trader-claude/backend/internal/registry"
	"github.com/trader-claude/backend/internal/worker"
)

// backtestHandler holds dependencies for all backtest and strategy endpoints.
type backtestHandler struct {
	db   *gorm.DB
	rdb  *redis.Client
	pool *worker.WorkerPool
	ds   *adapter.DataService
}

func newBacktestHandler(db *gorm.DB, rdb *redis.Client, pool *worker.WorkerPool, ds *adapter.DataService) *backtestHandler {
	return &backtestHandler{db: db, rdb: rdb, pool: pool, ds: ds}
}

// ---- Strategy endpoints -------------------------------------------------

// listStrategies handles GET /api/v1/strategies
func (h *backtestHandler) listStrategies(c *fiber.Ctx) error {
	names := registry.Strategies().Names()
	result := make([]fiber.Map, 0, len(names))

	for _, name := range names {
		s, err := registry.Strategies().Create(name)
		if err != nil {
			continue
		}
		result = append(result, strategyToMap(name, s))
	}

	return c.JSON(fiber.Map{"data": result})
}

// getStrategy handles GET /api/v1/strategies/:id
func (h *backtestHandler) getStrategy(c *fiber.Ctx) error {
	id := c.Params("id")
	if !registry.Strategies().Exists(id) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "strategy not found"})
	}

	s, err := registry.Strategies().Create(id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to instantiate strategy"})
	}

	return c.JSON(fiber.Map{"data": strategyToMap(id, s)})
}

// strategyToMap converts a Strategy instance into a response map.
func strategyToMap(id string, s registry.Strategy) fiber.Map {
	params := make([]fiber.Map, 0, len(s.Params()))
	for _, p := range s.Params() {
		pm := fiber.Map{
			"name":        p.Name,
			"type":        p.Type,
			"default":     p.Default,
			"description": p.Description,
			"required":    p.Required,
		}
		if p.Min != nil {
			pm["min"] = p.Min
		}
		if p.Max != nil {
			pm["max"] = p.Max
		}
		if len(p.Options) > 0 {
			pm["options"] = p.Options
		}
		params = append(params, pm)
	}

	return fiber.Map{
		"id":          id,
		"name":        s.Name(),
		"description": s.Description(),
		"params":      params,
	}
}

// ---- Backtest run request -----------------------------------------------

// runBacktestRequest is the body accepted by POST /api/v1/backtest/run.
type runBacktestRequest struct {
	Name        string                 `json:"name"`
	Strategy    string                 `json:"strategy"`
	Adapter     string                 `json:"adapter"`
	Symbol      string                 `json:"symbol"`
	Market      string                 `json:"market"`
	Timeframe   string                 `json:"timeframe"`
	StartDate   time.Time              `json:"start_date"`
	EndDate     time.Time              `json:"end_date"`
	Params      map[string]interface{} `json:"params"`
	InitialCash float64                `json:"initial_cash"`
	Commission  float64                `json:"commission"`
}

// ---- Backtest endpoints -------------------------------------------------

// runBacktest handles POST /api/v1/backtest/run
func (h *backtestHandler) runBacktest(c *fiber.Ctx) error {
	var req runBacktestRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	// Validate required fields
	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name is required"})
	}
	if req.Strategy == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "strategy is required"})
	}
	if req.Adapter == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "adapter is required"})
	}
	if req.Symbol == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "symbol is required"})
	}
	if req.Market == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "market is required"})
	}
	if req.Timeframe == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "timeframe is required"})
	}
	if req.StartDate.IsZero() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "start_date is required"})
	}
	if req.EndDate.IsZero() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "end_date is required"})
	}
	if !req.EndDate.After(req.StartDate) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "end_date must be after start_date"})
	}

	// Validate strategy exists
	if !registry.Strategies().Exists(req.Strategy) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("strategy %q not found", req.Strategy)})
	}

	// Validate adapter exists
	a, err := registry.Adapters().Get(req.Adapter)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("adapter %q not found", req.Adapter)})
	}

	// Apply defaults
	if req.InitialCash <= 0 {
		req.InitialCash = 10000
	}
	if req.Commission <= 0 {
		req.Commission = 0.001
	}

	// Build params JSON for storage
	paramsJSON := models.JSON{}
	for k, v := range req.Params {
		paramsJSON[k] = v
	}

	// Create backtest record in DB
	bt := models.Backtest{
		Name:         req.Name,
		StrategyName: req.Strategy,
		Symbol:       req.Symbol,
		Market:       req.Market,
		Timeframe:    req.Timeframe,
		StartDate:    req.StartDate,
		EndDate:      req.EndDate,
		Params:       paramsJSON,
		Status:       models.BacktestStatusPending,
	}

	if err := h.db.WithContext(c.Context()).Create(&bt).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create backtest record"})
	}

	// Capture values for the goroutine closure
	btID := bt.ID
	adapterCopy := a
	reqCopy := req

	// Submit async job to worker pool
	submitted := h.pool.Submit(worker.Job{
		Name: fmt.Sprintf("backtest-%d", btID),
		Task: func(ctx context.Context) error {
			return h.executeBacktest(ctx, btID, adapterCopy, reqCopy)
		},
	})

	if !submitted {
		// Mark as failed immediately if pool is full
		h.db.Model(&models.Backtest{}).Where("id = ?", btID).Updates(map[string]interface{}{
			"status":        models.BacktestStatusFailed,
			"error_message": "worker pool is full, please try again later",
		})
		// Set Redis progress key to 100 so WS clients don't hang
		if h.rdb != nil {
			progressKey := fmt.Sprintf("backtest:%d:progress", btID)
			_ = h.rdb.Set(c.Context(), progressKey, "100", 24*time.Hour)
		}
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "worker pool is full"})
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"run_id": btID,
		"status": string(models.BacktestStatusPending),
	})
}

// executeBacktest is the actual backtest logic run in the worker pool.
func (h *backtestHandler) executeBacktest(ctx context.Context, btID int64, a registry.MarketAdapter, req runBacktestRequest) error {
	// Mark as running
	now := time.Now()
	if err := h.db.WithContext(ctx).Model(&models.Backtest{}).Where("id = ?", btID).Updates(map[string]interface{}{
		"status":     models.BacktestStatusRunning,
		"started_at": &now,
	}).Error; err != nil {
		log.Printf("[backtest %d] failed to update status to running: %v", btID, err)
	}

	// Create strategy instance
	strat, err := registry.Strategies().Create(req.Strategy)
	if err != nil {
		return h.failBacktest(ctx, btID, fmt.Sprintf("failed to create strategy: %v", err))
	}

	// Init strategy params
	if err := strat.Init(req.Params); err != nil {
		return h.failBacktest(ctx, btID, fmt.Sprintf("failed to init strategy: %v", err))
	}

	// Fetch candles
	modelCandles, err := h.ds.GetCandles(ctx, a, req.Symbol, req.Market, req.Timeframe, req.StartDate, req.EndDate)
	if err != nil {
		return h.failBacktest(ctx, btID, fmt.Sprintf("failed to fetch candles: %v", err))
	}

	if len(modelCandles) == 0 {
		return h.failBacktest(ctx, btID, "no candles found for the given parameters")
	}

	// Convert models.Candle to registry.Candle
	regCandles := make([]registry.Candle, 0, len(modelCandles))
	for _, mc := range modelCandles {
		regCandles = append(regCandles, registry.Candle{
			Symbol:    mc.Symbol,
			Market:    mc.Market,
			Timeframe: mc.Timeframe,
			Timestamp: mc.Timestamp,
			Open:      mc.Open,
			High:      mc.High,
			Low:       mc.Low,
			Close:     mc.Close,
			Volume:    mc.Volume,
		})
	}

	// Use a detached context for final writes so shutdown doesn't corrupt state
	saveCtx, saveCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer saveCancel()

	// Run backtest engine
	cfg := backtest.RunConfig{
		BacktestID:  btID,
		Strategy:    strat,
		Candles:     regCandles,
		InitialCash: req.InitialCash,
		Commission:  req.Commission,
		Timeframe:   req.Timeframe,
	}

	result, err := backtest.Run(saveCtx, cfg, h.db, h.rdb)
	if err != nil {
		return h.failBacktest(ctx, btID, fmt.Sprintf("backtest engine error: %v", err))
	}

	// Build equity curve as JSONArray
	equityArr := make(models.JSONArray, 0, len(result.EquityCurve))
	for _, ep := range result.EquityCurve {
		equityArr = append(equityArr, map[string]interface{}{
			"timestamp": ep.Timestamp.UTC().Format(time.RFC3339),
			"value":     ep.Value,
		})
	}

	// Build metrics as JSON
	metricsJSON := models.JSON{}
	if b, err := json.Marshal(result.Metrics); err == nil {
		var m map[string]interface{}
		if err := json.Unmarshal(b, &m); err == nil {
			metricsJSON = models.JSON(m)
		}
	}

	// Mark as completed
	completedAt := time.Now()
	if err := h.db.WithContext(saveCtx).Model(&models.Backtest{}).Where("id = ?", btID).Updates(map[string]interface{}{
		"status":       models.BacktestStatusCompleted,
		"metrics":      metricsJSON,
		"equity_curve": equityArr,
		"completed_at": &completedAt,
	}).Error; err != nil {
		log.Printf("[backtest %d] failed to save results: %v", btID, err)
		return err
	}

	log.Printf("[backtest %d] completed: %d trades", btID, len(result.Trades))
	return nil
}

// failBacktest marks a backtest as failed with the given error message.
func (h *backtestHandler) failBacktest(ctx context.Context, btID int64, msg string) error {
	log.Printf("[backtest %d] failed: %s", btID, msg)
	result := h.db.WithContext(ctx).Model(&models.Backtest{}).Where("id = ?", btID).Updates(map[string]interface{}{
		"status":        models.BacktestStatusFailed,
		"error_message": msg,
	})
	if result.Error != nil {
		log.Printf("[backtest %d] failed to update DB on failure: %v", btID, result.Error)
		return result.Error
	}
	// Also set progress to 100 so WS clients close
	if h.rdb != nil {
		progressKey := fmt.Sprintf("backtest:%d:progress", btID)
		_ = h.rdb.Set(ctx, progressKey, 100, 24*time.Hour)
	}
	return fmt.Errorf("%s", msg)
}

// listRuns handles GET /api/v1/backtest/runs
func (h *backtestHandler) listRuns(c *fiber.Ctx) error {
	var runs []models.Backtest
	if err := h.db.WithContext(c.Context()).
		Select("id, name, strategy_name, symbol, market, timeframe, start_date, end_date, params, status, metrics, error_message, started_at, completed_at, created_at, updated_at").
		Order("created_at DESC").
		Limit(50).
		Find(&runs).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list backtests"})
	}

	// Ensure null slice becomes empty array in JSON
	if runs == nil {
		runs = []models.Backtest{}
	}

	return c.JSON(fiber.Map{"data": runs})
}

// getRun handles GET /api/v1/backtest/runs/:id
func (h *backtestHandler) getRun(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	var bt models.Backtest
	if err := h.db.WithContext(c.Context()).First(&bt, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "backtest not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to fetch backtest"})
	}

	var trades []models.Trade
	if err := h.db.WithContext(c.Context()).Where("backtest_id = ?", id).Find(&trades).Error; err != nil {
		trades = []models.Trade{}
	}

	return c.JSON(fiber.Map{
		"backtest":     bt,
		"trades":       trades,
		"equity_curve": bt.EquityCurve,
	})
}

// deleteRun handles DELETE /api/v1/backtest/runs/:id
func (h *backtestHandler) deleteRun(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	result := h.db.WithContext(c.Context()).Delete(&models.Backtest{}, id)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete backtest"})
	}
	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "backtest not found"})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// ---- WebSocket progress stream ------------------------------------------

// progressWS handles GET /ws/backtest/:id/progress
// Polls Redis every 500 ms and pushes progress updates to the WS client.
func (h *backtestHandler) progressWS(c *websocket.Conn) {
	runIDStr := c.Params("id")
	runID, err := strconv.ParseInt(runIDStr, 10, 64)
	if err != nil {
		_ = c.WriteJSON(fiber.Map{"error": "invalid backtest id"})
		return
	}

	progressKey := fmt.Sprintf("backtest:%d:progress", runID)
	resultKey := fmt.Sprintf("backtest:%d:result", runID)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			val, err := h.rdb.Get(ctx, progressKey).Result()
			if err != nil {
				// Key not found yet — backtest may be queued, keep polling
				if err == redis.Nil {
					_ = c.WriteJSON(fiber.Map{"progress": 0})
					continue
				}
				_ = c.WriteJSON(fiber.Map{"error": "redis error"})
				return
			}

			progress, _ := strconv.Atoi(val)

			if progress >= 100 {
				// Fetch the result metrics if available
				resultVal, redisErr := h.rdb.Get(ctx, resultKey).Result()
				if redisErr == nil {
					var metrics map[string]interface{}
					if jsonErr := json.Unmarshal([]byte(resultVal), &metrics); jsonErr == nil {
						_ = c.WriteJSON(fiber.Map{"progress": 100, "done": true, "metrics": metrics})
						return
					}
				}
				_ = c.WriteJSON(fiber.Map{"progress": 100, "done": true})
				return
			}

			_ = c.WriteJSON(fiber.Map{"progress": progress})
		}
	}
}
