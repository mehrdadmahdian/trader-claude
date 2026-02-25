package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/adapter"
	"github.com/trader-claude/backend/internal/backtest"
	"github.com/trader-claude/backend/internal/models"
	"github.com/trader-claude/backend/internal/registry"
	"github.com/trader-claude/backend/internal/replay"
)

type replayHandler struct {
	db      *gorm.DB
	ds      *adapter.DataService
	manager *replay.Manager
}

func newReplayHandler(db *gorm.DB, ds *adapter.DataService, mgr *replay.Manager) *replayHandler {
	return &replayHandler{db: db, ds: ds, manager: mgr}
}

// createReplay handles POST /api/v1/backtest/runs/:id/replay
func (h *replayHandler) createReplay(c *fiber.Ctx) error {
	runID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid backtest run id"})
	}

	if h.db == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "database unavailable"})
	}

	var bt models.Backtest
	if err := h.db.WithContext(c.Context()).First(&bt, runID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "backtest not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load backtest"})
	}

	if bt.Status != models.BacktestStatusCompleted {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "backtest must be completed before replay"})
	}

	var trades []models.Trade
	if err := h.db.WithContext(c.Context()).Where("backtest_id = ?", runID).Order("entry_time ASC").Find(&trades).Error; err != nil {
		trades = []models.Trade{}
	}

	var mktAdapter registry.MarketAdapter
	if bt.AdapterID != "" {
		mktAdapter, err = registry.Adapters().Get(bt.AdapterID)
		if err != nil {
			log.Printf("[replay] adapter %q not found, falling back to market lookup", bt.AdapterID)
		}
	}
	if mktAdapter == nil {
		for _, name := range registry.Adapters().Names() {
			a, _ := registry.Adapters().Get(name)
			for _, m := range a.Markets() {
				if m == bt.Market {
					mktAdapter = a
					break
				}
			}
			if mktAdapter != nil {
				break
			}
		}
	}
	if mktAdapter == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "no adapter found for market " + bt.Market})
	}

	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()

	modelCandles, err := h.ds.GetCandles(ctx, mktAdapter, bt.Symbol, bt.Market, bt.Timeframe, bt.StartDate, bt.EndDate)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to fetch candles: " + err.Error()})
	}

	candles := make([]registry.Candle, len(modelCandles))
	for i, mc := range modelCandles {
		candles[i] = registry.Candle{
			Symbol:    mc.Symbol,
			Market:    mc.Market,
			Timeframe: mc.Timeframe,
			Timestamp: mc.Timestamp,
			Open:      mc.Open,
			High:      mc.High,
			Low:       mc.Low,
			Close:     mc.Close,
			Volume:    mc.Volume,
		}
	}

	equity := make([]backtest.EquityPoint, 0, len(bt.EquityCurve))
	for _, item := range bt.EquityCurve {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		tsStr, _ := m["timestamp"].(string)
		val, _ := m["value"].(float64)
		ts, _ := time.Parse(time.RFC3339, tsStr)
		equity = append(equity, backtest.EquityPoint{Timestamp: ts, Value: val})
	}

	replayID, err := generateID()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to generate replay id"})
	}

	session := replay.NewSession(replayID, runID, candles, equity, trades)
	h.manager.Store(session)

	return c.JSON(fiber.Map{
		"replay_id":     replayID,
		"total_candles": len(candles),
	})
}

// replayWS handles WS /ws/replay/:replay_id
func (h *replayHandler) replayWS(c *websocket.Conn) {
	replayID := c.Params("replay_id")
	session, ok := h.manager.Get(replayID)
	if !ok {
		_ = c.WriteJSON(fiber.Map{"type": "error", "message": "replay session not found"})
		return
	}
	defer h.manager.Delete(replayID)

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			var ctrl replay.ControlMsg
			if err := c.ReadJSON(&ctrl); err != nil {
				return
			}
			select {
			case session.ControlChan <- ctrl:
			default:
			}
		}
	}()

	writeFn := func(msg replay.Message) error {
		return c.WriteJSON(msg)
	}

	if err := session.Run(writeFn, done); err != nil {
		log.Printf("[replay %s] session error: %v", replayID, err)
	}
}

type createBookmarkRequest struct {
	BacktestRunID int64  `json:"backtest_run_id"`
	CandleIndex   int    `json:"candle_index"`
	Label         string `json:"label"`
	Note          string `json:"note"`
	ChartSnapshot string `json:"chart_snapshot"`
}

// createBookmark handles POST /api/v1/replay/bookmarks
func (h *replayHandler) createBookmark(c *fiber.Ctx) error {
	var req createBookmarkRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.BacktestRunID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "backtest_run_id is required"})
	}

	if h.db == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "database unavailable"})
	}

	bm := models.ReplayBookmark{
		UserID:        1,
		BacktestRunID: req.BacktestRunID,
		CandleIndex:   req.CandleIndex,
		Label:         req.Label,
		Note:          req.Note,
		ChartSnapshot: req.ChartSnapshot,
	}

	if err := h.db.WithContext(c.Context()).Create(&bm).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to save bookmark"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"data": bm})
}

// listBookmarks handles GET /api/v1/replay/bookmarks?run_id=N
func (h *replayHandler) listBookmarks(c *fiber.Ctx) error {
	runIDStr := c.Query("run_id")
	if runIDStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "run_id query param is required"})
	}
	runID, err := strconv.ParseInt(runIDStr, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid run_id"})
	}

	if h.db == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "database unavailable"})
	}

	var bookmarks []models.ReplayBookmark
	if err := h.db.WithContext(c.Context()).
		Where("backtest_run_id = ?", runID).
		Order("created_at DESC").
		Find(&bookmarks).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list bookmarks"})
	}

	if bookmarks == nil {
		bookmarks = []models.ReplayBookmark{}
	}

	return c.JSON(fiber.Map{"data": bookmarks})
}

// getBookmark handles GET /api/v1/replay/bookmarks/:id
func (h *replayHandler) getBookmark(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	if h.db == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "database unavailable"})
	}

	var bm models.ReplayBookmark
	if err := h.db.WithContext(c.Context()).First(&bm, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "bookmark not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to fetch bookmark"})
	}

	return c.JSON(fiber.Map{"data": bm})
}

// deleteBookmark handles DELETE /api/v1/replay/bookmarks/:id
func (h *replayHandler) deleteBookmark(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	if h.db == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "database unavailable"})
	}

	result := h.db.WithContext(c.Context()).Delete(&models.ReplayBookmark{}, id)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete bookmark"})
	}
	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "bookmark not found"})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func generateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("rand.Read: %w", err)
	}
	return hex.EncodeToString(b), nil
}
