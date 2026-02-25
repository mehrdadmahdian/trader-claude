package api

import (
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/adapter"
	"github.com/trader-claude/backend/internal/indicator"
	"github.com/trader-claude/backend/internal/replay"
	"github.com/trader-claude/backend/internal/worker"
	"github.com/trader-claude/backend/internal/ws"
)

// RegisterRoutes wires all HTTP and WebSocket routes onto the Fiber app
func RegisterRoutes(app *fiber.App, db *gorm.DB, rdb *redis.Client, hub *ws.Hub, version string, pool *worker.WorkerPool, ds *adapter.DataService, mgr *replay.Manager) {
	// Health
	health := newHealthHandler(db, rdb, version)
	app.Get("/health", health.check)

	// API v1 group
	v1 := app.Group("/api/v1")

	// --- Markets ---
	mh := newMarketsHandler(ds)
	v1.Get("/markets", mh.listAdapters)
	v1.Get("/markets/:adapterID/symbols", mh.listSymbols)

	// --- Candles ---
	v1.Get("/candles/timeframes", mh.listTimeframes)
	v1.Get("/candles", mh.getCandles)

	// --- Strategies ---
	bh := newBacktestHandler(db, rdb, pool, ds)
	v1.Get("/strategies", bh.listStrategies)
	v1.Get("/strategies/:id", bh.getStrategy)

	// --- Backtests ---
	v1.Post("/backtest/run", bh.runBacktest)
	v1.Get("/backtest/runs", bh.listRuns)
	v1.Get("/backtest/runs/:id", bh.getRun)
	v1.Delete("/backtest/runs/:id", bh.deleteRun)

	// --- Replay ---
	rh := newReplayHandler(db, ds, mgr)
	v1.Post("/backtest/runs/:id/replay", rh.createReplay)
	v1.Post("/replay/bookmarks", rh.createBookmark)
	v1.Get("/replay/bookmarks", rh.listBookmarks)
	v1.Get("/replay/bookmarks/:id", rh.getBookmark)
	v1.Delete("/replay/bookmarks/:id", rh.deleteBookmark)

	// --- Indicators ---
	ih := indicator.NewHandler()
	v1.Get("/indicators", ih.ListIndicators)
	v1.Post("/indicators/calculate", ih.Calculate)

	// --- Portfolios ---
	v1.Get("/portfolios", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"data": []interface{}{}, "message": "portfolios endpoint — coming soon"})
	})
	v1.Post("/portfolios", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"message": "portfolio create endpoint — coming soon"})
	})

	// --- Alerts ---
	v1.Get("/alerts", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"data": []interface{}{}, "message": "alerts endpoint — coming soon"})
	})
	v1.Post("/alerts", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"message": "alert create endpoint — coming soon"})
	})
	v1.Delete("/alerts/:id", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})

	// --- Notifications ---
	v1.Get("/notifications", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"data": []interface{}{}, "message": "notifications endpoint — coming soon"})
	})
	v1.Patch("/notifications/:id/read", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})

	// --- WebSocket upgrade middleware ---
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	// Market data WebSocket (hub)
	app.Get("/ws", websocket.New(hub.ServeWS))

	// Backtest progress WebSocket
	app.Get("/ws/backtest/:id/progress", websocket.New(bh.progressWS))

	// Replay WebSocket
	app.Get("/ws/replay/:replay_id", websocket.New(rh.replayWS))
}
