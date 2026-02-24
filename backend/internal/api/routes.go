package api

import (
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/adapter"
	"github.com/trader-claude/backend/internal/ws"
)

// RegisterRoutes wires all HTTP and WebSocket routes onto the Fiber app
func RegisterRoutes(app *fiber.App, db *gorm.DB, rdb *redis.Client, hub *ws.Hub, version string) {
	// Health
	health := newHealthHandler(db, rdb, version)
	app.Get("/health", health.check)

	// API v1 group
	v1 := app.Group("/api/v1")

	// --- Markets ---
	ds := adapter.NewDataService(db, rdb)
	mh := newMarketsHandler(ds)
	v1.Get("/markets", mh.listAdapters)
	v1.Get("/markets/:adapterID/symbols", mh.listSymbols)

	// --- Candles ---
	v1.Get("/candles/timeframes", mh.listTimeframes)
	v1.Get("/candles", mh.getCandles)

	// --- Strategies ---
	v1.Get("/strategies", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"data": []interface{}{}, "message": "strategies endpoint — coming soon"})
	})

	// --- Backtests ---
	v1.Get("/backtests", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"data": []interface{}{}, "message": "backtests endpoint — coming soon"})
	})
	v1.Post("/backtests", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"message": "backtest run endpoint — coming soon"})
	})
	v1.Get("/backtests/:id", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"data": nil, "message": "backtest detail endpoint — coming soon"})
	})

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

	// --- WebSocket ---
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	app.Get("/ws", websocket.New(hub.ServeWS))
}
